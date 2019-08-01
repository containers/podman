package imagebuildah

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/buildah"
	buildahdocker "github.com/containers/buildah/docker"
	"github.com/containers/buildah/util"
	cp "github.com/containers/image/copy"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	is "github.com/containers/image/storage"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	securejoin "github.com/cyphar/filepath-securejoin"
	docker "github.com/fsouza/go-dockerclient"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerfile/parser"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// StageExecutor bundles up what we need to know when executing one stage of a
// (possibly multi-stage) build.
// Each stage may need to produce an image to be used as the base in a later
// stage (with the last stage's image being the end product of the build), and
// it may need to leave its working container in place so that the container's
// root filesystem's contents can be used as the source for a COPY instruction
// in a later stage.
// Each stage has its own base image, so it starts with its own configuration
// and set of volumes.
// If we're naming the result of the build, only the last stage will apply that
// name to the image that it produces.
type StageExecutor struct {
	executor        *Executor
	index           int
	stages          int
	name            string
	builder         *buildah.Builder
	preserved       int
	volumes         imagebuilder.VolumeSet
	volumeCache     map[string]string
	volumeCacheInfo map[string]os.FileInfo
	mountPoint      string
	copyFrom        string // Used to keep track of the --from flag from COPY and ADD
	output          string
	containerIDs    []string
}

// Preserve informs the stage executor that from this point on, it needs to
// ensure that only COPY and ADD instructions can modify the contents of this
// directory or anything below it.
// The StageExecutor handles this by caching the contents of directories which
// have been marked this way before executing a RUN instruction, invalidating
// that cache when an ADD or COPY instruction sets any location under the
// directory as the destination, and using the cache to reset the contents of
// the directory tree after processing each RUN instruction.
// It would be simpler if we could just mark the directory as a read-only bind
// mount of itself during Run(), but the directory is expected to be remain
// writeable while the RUN instruction is being handled, even if any changes
// made within the directory are ultimately discarded.
func (s *StageExecutor) Preserve(path string) error {
	logrus.Debugf("PRESERVE %q", path)
	if s.volumes.Covers(path) {
		// This path is already a subdirectory of a volume path that
		// we're already preserving, so there's nothing new to be done
		// except ensure that it exists.
		archivedPath := filepath.Join(s.mountPoint, path)
		if err := os.MkdirAll(archivedPath, 0755); err != nil {
			return errors.Wrapf(err, "error ensuring volume path %q exists", archivedPath)
		}
		if err := s.volumeCacheInvalidate(path); err != nil {
			return errors.Wrapf(err, "error ensuring volume path %q is preserved", archivedPath)
		}
		return nil
	}
	// Figure out where the cache for this volume would be stored.
	s.preserved++
	cacheDir, err := s.executor.store.ContainerDirectory(s.builder.ContainerID)
	if err != nil {
		return errors.Errorf("unable to locate temporary directory for container")
	}
	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("volume%d.tar", s.preserved))
	// Save info about the top level of the location that we'll be archiving.
	var archivedPath string

	// Try and resolve the symlink (if one exists)
	// Set archivedPath and path based on whether a symlink is found or not
	if symLink, err := resolveSymlink(s.mountPoint, path); err == nil {
		archivedPath = filepath.Join(s.mountPoint, symLink)
		path = symLink
	} else {
		return errors.Wrapf(err, "error reading symbolic link to %q", path)
	}

	st, err := os.Stat(archivedPath)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(archivedPath, 0755); err != nil {
			return errors.Wrapf(err, "error ensuring volume path %q exists", archivedPath)
		}
		st, err = os.Stat(archivedPath)
	}
	if err != nil {
		logrus.Debugf("error reading info about %q: %v", archivedPath, err)
		return errors.Wrapf(err, "error reading info about volume path %q", archivedPath)
	}
	s.volumeCacheInfo[path] = st
	if !s.volumes.Add(path) {
		// This path is not a subdirectory of a volume path that we're
		// already preserving, so adding it to the list should work.
		return errors.Errorf("error adding %q to the volume cache", path)
	}
	s.volumeCache[path] = cacheFile
	// Now prune cache files for volumes that are now supplanted by this one.
	removed := []string{}
	for cachedPath := range s.volumeCache {
		// Walk our list of cached volumes, and check that they're
		// still in the list of locations that we need to cache.
		found := false
		for _, volume := range s.volumes {
			if volume == cachedPath {
				// We need to keep this volume's cache.
				found = true
				break
			}
		}
		if !found {
			// We don't need to keep this volume's cache.  Make a
			// note to remove it.
			removed = append(removed, cachedPath)
		}
	}
	// Actually remove the caches that we decided to remove.
	for _, cachedPath := range removed {
		archivedPath := filepath.Join(s.mountPoint, cachedPath)
		logrus.Debugf("no longer need cache of %q in %q", archivedPath, s.volumeCache[cachedPath])
		if err := os.Remove(s.volumeCache[cachedPath]); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return errors.Wrapf(err, "error removing %q", s.volumeCache[cachedPath])
		}
		delete(s.volumeCache, cachedPath)
	}
	return nil
}

// Remove any volume cache item which will need to be re-saved because we're
// writing to part of it.
func (s *StageExecutor) volumeCacheInvalidate(path string) error {
	invalidated := []string{}
	for cachedPath := range s.volumeCache {
		if strings.HasPrefix(path, cachedPath+string(os.PathSeparator)) {
			invalidated = append(invalidated, cachedPath)
		}
	}
	for _, cachedPath := range invalidated {
		if err := os.Remove(s.volumeCache[cachedPath]); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return errors.Wrapf(err, "error removing volume cache %q", s.volumeCache[cachedPath])
		}
		archivedPath := filepath.Join(s.mountPoint, cachedPath)
		logrus.Debugf("invalidated volume cache for %q from %q", archivedPath, s.volumeCache[cachedPath])
		delete(s.volumeCache, cachedPath)
	}
	return nil
}

// Save the contents of each of the executor's list of volumes for which we
// don't already have a cache file.
func (s *StageExecutor) volumeCacheSave() error {
	for cachedPath, cacheFile := range s.volumeCache {
		archivedPath := filepath.Join(s.mountPoint, cachedPath)
		_, err := os.Stat(cacheFile)
		if err == nil {
			logrus.Debugf("contents of volume %q are already cached in %q", archivedPath, cacheFile)
			continue
		}
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "error checking for cache of %q in %q", archivedPath, cacheFile)
		}
		if err := os.MkdirAll(archivedPath, 0755); err != nil {
			return errors.Wrapf(err, "error ensuring volume path %q exists", archivedPath)
		}
		logrus.Debugf("caching contents of volume %q in %q", archivedPath, cacheFile)
		cache, err := os.Create(cacheFile)
		if err != nil {
			return errors.Wrapf(err, "error creating archive at %q", cacheFile)
		}
		defer cache.Close()
		rc, err := archive.Tar(archivedPath, archive.Uncompressed)
		if err != nil {
			return errors.Wrapf(err, "error archiving %q", archivedPath)
		}
		defer rc.Close()
		_, err = io.Copy(cache, rc)
		if err != nil {
			return errors.Wrapf(err, "error archiving %q to %q", archivedPath, cacheFile)
		}
	}
	return nil
}

// Restore the contents of each of the executor's list of volumes.
func (s *StageExecutor) volumeCacheRestore() error {
	for cachedPath, cacheFile := range s.volumeCache {
		archivedPath := filepath.Join(s.mountPoint, cachedPath)
		logrus.Debugf("restoring contents of volume %q from %q", archivedPath, cacheFile)
		cache, err := os.Open(cacheFile)
		if err != nil {
			return errors.Wrapf(err, "error opening archive at %q", cacheFile)
		}
		defer cache.Close()
		if err := os.RemoveAll(archivedPath); err != nil {
			return errors.Wrapf(err, "error clearing volume path %q", archivedPath)
		}
		if err := os.MkdirAll(archivedPath, 0755); err != nil {
			return errors.Wrapf(err, "error recreating volume path %q", archivedPath)
		}
		err = archive.Untar(cache, archivedPath, nil)
		if err != nil {
			return errors.Wrapf(err, "error extracting archive at %q", archivedPath)
		}
		if st, ok := s.volumeCacheInfo[cachedPath]; ok {
			if err := os.Chmod(archivedPath, st.Mode()); err != nil {
				return errors.Wrapf(err, "error restoring permissions on %q", archivedPath)
			}
			if err := os.Chown(archivedPath, 0, 0); err != nil {
				return errors.Wrapf(err, "error setting ownership on %q", archivedPath)
			}
			if err := os.Chtimes(archivedPath, st.ModTime(), st.ModTime()); err != nil {
				return errors.Wrapf(err, "error restoring datestamps on %q", archivedPath)
			}
		}
	}
	return nil
}

// Copy copies data into the working tree.  The "Download" field is how
// imagebuilder tells us the instruction was "ADD" and not "COPY"
func (s *StageExecutor) Copy(excludes []string, copies ...imagebuilder.Copy) error {
	for _, copy := range copies {
		// Check the file and see if part of it is a symlink.
		// Convert it to the target if so.  To be ultrasafe
		// do the same for the mountpoint.
		hadFinalPathSeparator := len(copy.Dest) > 0 && copy.Dest[len(copy.Dest)-1] == os.PathSeparator
		secureMountPoint, err := securejoin.SecureJoin("", s.mountPoint)
		if err != nil {
			return errors.Wrapf(err, "error resolving symlinks for copy destination %s", copy.Dest)
		}
		finalPath, err := securejoin.SecureJoin(secureMountPoint, copy.Dest)
		if err != nil {
			return errors.Wrapf(err, "error resolving symlinks for copy destination %s", copy.Dest)
		}
		if !strings.HasPrefix(finalPath, secureMountPoint) {
			return errors.Wrapf(err, "error resolving copy destination %s", copy.Dest)
		}
		copy.Dest = strings.TrimPrefix(finalPath, secureMountPoint)
		if len(copy.Dest) == 0 || copy.Dest[len(copy.Dest)-1] != os.PathSeparator {
			if hadFinalPathSeparator {
				copy.Dest += string(os.PathSeparator)
			}
		}

		if copy.Download {
			logrus.Debugf("ADD %#v, %#v", excludes, copy)
		} else {
			logrus.Debugf("COPY %#v, %#v", excludes, copy)
		}
		if err := s.volumeCacheInvalidate(copy.Dest); err != nil {
			return err
		}
		sources := []string{}
		for _, src := range copy.Src {
			contextDir := s.executor.contextDir
			copyExcludes := excludes
			var idMappingOptions *buildah.IDMappingOptions
			if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
				sources = append(sources, src)
			} else if len(copy.From) > 0 {
				var srcRoot string
				if other, ok := s.executor.stages[copy.From]; ok && other.index < s.index {
					srcRoot = other.mountPoint
					contextDir = other.mountPoint
					idMappingOptions = &other.builder.IDMappingOptions
				} else if builder, ok := s.executor.containerMap[copy.From]; ok {
					srcRoot = builder.MountPoint
					contextDir = builder.MountPoint
					idMappingOptions = &builder.IDMappingOptions
				} else {
					return errors.Errorf("the stage %q has not been built", copy.From)
				}
				srcSecure, err := securejoin.SecureJoin(srcRoot, src)
				if err != nil {
					return err
				}
				// If destination is a folder, we need to take extra care to
				// ensure that files are copied with correct names (since
				// resolving a symlink may result in a different name).
				if hadFinalPathSeparator {
					_, srcName := filepath.Split(src)
					_, srcNameSecure := filepath.Split(srcSecure)
					if srcName != srcNameSecure {
						options := buildah.AddAndCopyOptions{
							Chown:      copy.Chown,
							ContextDir: contextDir,
							Excludes:   copyExcludes,
						}
						if err := s.builder.Add(filepath.Join(copy.Dest, srcName), copy.Download, options, srcSecure); err != nil {
							return err
						}
						continue
					}
				}
				sources = append(sources, srcSecure)

			} else {
				sources = append(sources, filepath.Join(s.executor.contextDir, src))
				copyExcludes = append(s.executor.excludes, excludes...)
			}
			options := buildah.AddAndCopyOptions{
				Chown:            copy.Chown,
				ContextDir:       contextDir,
				Excludes:         copyExcludes,
				IDMappingOptions: idMappingOptions,
			}
			if err := s.builder.Add(copy.Dest, copy.Download, options, sources...); err != nil {
				return err
			}
		}
	}
	return nil
}

// Run executes a RUN instruction using the stage's current working container
// as a root directory.
func (s *StageExecutor) Run(run imagebuilder.Run, config docker.Config) error {
	logrus.Debugf("RUN %#v, %#v", run, config)
	if s.builder == nil {
		return errors.Errorf("no build container available")
	}
	stdin := s.executor.in
	if stdin == nil {
		devNull, err := os.Open(os.DevNull)
		if err != nil {
			return errors.Errorf("error opening %q for reading: %v", os.DevNull, err)
		}
		defer devNull.Close()
		stdin = devNull
	}
	options := buildah.RunOptions{
		Hostname:         config.Hostname,
		Runtime:          s.executor.runtime,
		Args:             s.executor.runtimeArgs,
		NoPivot:          os.Getenv("BUILDAH_NOPIVOT") != "",
		Mounts:           convertMounts(s.executor.transientMounts),
		Env:              config.Env,
		User:             config.User,
		WorkingDir:       config.WorkingDir,
		Entrypoint:       config.Entrypoint,
		Cmd:              config.Cmd,
		Stdin:            stdin,
		Stdout:           s.executor.out,
		Stderr:           s.executor.err,
		Quiet:            s.executor.quiet,
		NamespaceOptions: s.executor.namespaceOptions,
	}
	if config.NetworkDisabled {
		options.ConfigureNetwork = buildah.NetworkDisabled
	} else {
		options.ConfigureNetwork = buildah.NetworkEnabled
	}

	args := run.Args
	if run.Shell {
		if len(config.Shell) > 0 && s.builder.Format == buildah.Dockerv2ImageManifest {
			args = append(config.Shell, args...)
		} else {
			args = append([]string{"/bin/sh", "-c"}, args...)
		}
	}
	if err := s.volumeCacheSave(); err != nil {
		return err
	}
	err := s.builder.Run(args, options)
	if err2 := s.volumeCacheRestore(); err2 != nil {
		if err == nil {
			return err2
		}
	}
	return err
}

// UnrecognizedInstruction is called when we encounter an instruction that the
// imagebuilder parser didn't understand.
func (s *StageExecutor) UnrecognizedInstruction(step *imagebuilder.Step) error {
	errStr := fmt.Sprintf("Build error: Unknown instruction: %q ", strings.ToUpper(step.Command))
	err := fmt.Sprintf(errStr+"%#v", step)
	if s.executor.ignoreUnrecognizedInstructions {
		logrus.Debugf(err)
		return nil
	}

	switch logrus.GetLevel() {
	case logrus.ErrorLevel:
		logrus.Errorf(errStr)
	case logrus.DebugLevel:
		logrus.Debugf(err)
	default:
		logrus.Errorf("+(UNHANDLED LOGLEVEL) %#v", step)
	}

	return errors.Errorf(err)
}

// prepare creates a working container based on the specified image, or if one
// isn't specified, the first argument passed to the first FROM instruction we
// can find in the stage's parsed tree.
func (s *StageExecutor) prepare(ctx context.Context, stage imagebuilder.Stage, from string, initializeIBConfig, rebase bool) (builder *buildah.Builder, err error) {
	ib := stage.Builder
	node := stage.Node

	if from == "" {
		base, err := ib.From(node)
		if err != nil {
			logrus.Debugf("prepare(node.Children=%#v)", node.Children)
			return nil, errors.Wrapf(err, "error determining starting point for build")
		}
		from = base
	}
	displayFrom := from

	// stage.Name will be a numeric string for all stages without an "AS" clause
	asImageName := stage.Name
	if asImageName != "" {
		if _, err := strconv.Atoi(asImageName); err != nil {
			displayFrom = from + " AS " + asImageName
		}
	}

	if initializeIBConfig && rebase {
		logrus.Debugf("FROM %#v", displayFrom)
		if !s.executor.quiet {
			s.executor.log("FROM %s", displayFrom)
		}
	}

	builderOptions := buildah.BuilderOptions{
		Args:                  ib.Args,
		FromImage:             from,
		PullPolicy:            s.executor.pullPolicy,
		Registry:              s.executor.registry,
		BlobDirectory:         s.executor.blobDirectory,
		SignaturePolicyPath:   s.executor.signaturePolicyPath,
		ReportWriter:          s.executor.reportWriter,
		SystemContext:         s.executor.systemContext,
		Isolation:             s.executor.isolation,
		NamespaceOptions:      s.executor.namespaceOptions,
		ConfigureNetwork:      s.executor.configureNetwork,
		CNIPluginPath:         s.executor.cniPluginPath,
		CNIConfigDir:          s.executor.cniConfigDir,
		IDMappingOptions:      s.executor.idmappingOptions,
		CommonBuildOpts:       s.executor.commonBuildOptions,
		DefaultMountsFilePath: s.executor.defaultMountsFilePath,
		Format:                s.executor.outputFormat,
	}

	// Check and see if the image is a pseudonym for the end result of a
	// previous stage, named by an AS clause in the Dockerfile.
	if asImageFound, ok := s.executor.imageMap[from]; ok {
		builderOptions.FromImage = asImageFound
	}
	builder, err = buildah.NewBuilder(ctx, s.executor.store, builderOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating build container")
	}

	if initializeIBConfig {
		volumes := map[string]struct{}{}
		for _, v := range builder.Volumes() {
			volumes[v] = struct{}{}
		}
		ports := map[docker.Port]struct{}{}
		for _, p := range builder.Ports() {
			ports[docker.Port(p)] = struct{}{}
		}
		dConfig := docker.Config{
			Hostname:     builder.Hostname(),
			Domainname:   builder.Domainname(),
			User:         builder.User(),
			Env:          builder.Env(),
			Cmd:          builder.Cmd(),
			Image:        from,
			Volumes:      volumes,
			WorkingDir:   builder.WorkDir(),
			Entrypoint:   builder.Entrypoint(),
			Labels:       builder.Labels(),
			Shell:        builder.Shell(),
			StopSignal:   builder.StopSignal(),
			OnBuild:      builder.OnBuild(),
			ExposedPorts: ports,
		}
		var rootfs *docker.RootFS
		if builder.Docker.RootFS != nil {
			rootfs = &docker.RootFS{
				Type: builder.Docker.RootFS.Type,
			}
			for _, id := range builder.Docker.RootFS.DiffIDs {
				rootfs.Layers = append(rootfs.Layers, id.String())
			}
		}
		dImage := docker.Image{
			Parent:          builder.FromImage,
			ContainerConfig: dConfig,
			Container:       builder.Container,
			Author:          builder.Maintainer(),
			Architecture:    builder.Architecture(),
			RootFS:          rootfs,
		}
		dImage.Config = &dImage.ContainerConfig
		err = ib.FromImage(&dImage, node)
		if err != nil {
			if err2 := builder.Delete(); err2 != nil {
				logrus.Debugf("error deleting container which we failed to update: %v", err2)
			}
			return nil, errors.Wrapf(err, "error updating build context")
		}
	}
	mountPoint, err := builder.Mount(builder.MountLabel)
	if err != nil {
		if err2 := builder.Delete(); err2 != nil {
			logrus.Debugf("error deleting container which we failed to mount: %v", err2)
		}
		return nil, errors.Wrapf(err, "error mounting new container")
	}
	if rebase {
		// Make this our "current" working container.
		s.mountPoint = mountPoint
		s.builder = builder
	}
	logrus.Debugln("Container ID:", builder.ContainerID)
	return builder, nil
}

// Delete deletes the stage's working container, if we have one.
func (s *StageExecutor) Delete() (err error) {
	if s.builder != nil {
		err = s.builder.Delete()
		s.builder = nil
	}
	return err
}

// stepRequiresLayer indicates whether or not the step should be followed by
// committing a layer container when creating an intermediate image.
func (*StageExecutor) stepRequiresLayer(step *imagebuilder.Step) bool {
	switch strings.ToUpper(step.Command) {
	case "ADD", "COPY", "RUN":
		return true
	}
	return false
}

// getImageRootfs checks for an image matching the passed-in name in local
// storage.  If it isn't found, it pulls down a copy.  Then, if we don't have a
// working container root filesystem based on the image, it creates one.  Then
// it returns that root filesystem's location.
func (s *StageExecutor) getImageRootfs(ctx context.Context, stage imagebuilder.Stage, image string) (mountPoint string, err error) {
	if builder, ok := s.executor.containerMap[image]; ok {
		return builder.MountPoint, nil
	}
	builder, err := s.prepare(ctx, stage, image, false, false)
	if err != nil {
		return "", err
	}
	s.executor.containerMap[image] = builder
	return builder.MountPoint, nil
}

// Execute runs each of the steps in the stage's parsed tree, in turn.
func (s *StageExecutor) Execute(ctx context.Context, stage imagebuilder.Stage, base string) (imgID string, ref reference.Canonical, err error) {
	ib := stage.Builder
	checkForLayers := s.executor.layers && s.executor.useCache
	moreStages := s.index < s.stages-1
	lastStage := !moreStages
	imageIsUsedLater := moreStages && (s.executor.baseMap[stage.Name] || s.executor.baseMap[fmt.Sprintf("%d", stage.Position)])
	rootfsIsUsedLater := moreStages && (s.executor.rootfsMap[stage.Name] || s.executor.rootfsMap[fmt.Sprintf("%d", stage.Position)])

	// If the base image's name corresponds to the result of an earlier
	// stage, substitute that image's ID for the base image's name here.
	// If not, then go on assuming that it's just a regular image that's
	// either in local storage, or one that we have to pull from a
	// registry.
	if stageImage, isPreviousStage := s.executor.imageMap[base]; isPreviousStage {
		base = stageImage
	}

	// Create the (first) working container for this stage.  Reinitializing
	// the imagebuilder configuration may alter the list of steps we have,
	// so take a snapshot of them *after* that.
	if _, err := s.prepare(ctx, stage, base, true, true); err != nil {
		return "", nil, err
	}
	children := stage.Node.Children

	// A helper function to only log "COMMIT" as an explicit step if it's
	// the very last step of a (possibly multi-stage) build.
	logCommit := func(output string, instruction int) {
		moreInstructions := instruction < len(children)-1
		if moreInstructions || moreStages {
			return
		}
		commitMessage := "COMMIT"
		if output != "" {
			commitMessage = fmt.Sprintf("%s %s", commitMessage, output)
		}
		logrus.Debugf(commitMessage)
		if !s.executor.quiet {
			s.executor.log(commitMessage)
		}
	}
	logImageID := func(imgID string) {
		if s.executor.iidfile == "" {
			fmt.Fprintf(s.executor.out, "%s\n", imgID)
		}
	}

	if len(children) == 0 {
		// There are no steps.
		if s.builder.FromImageID == "" || s.executor.squash {
			// We either don't have a base image, or we need to
			// squash the contents of the base image.  Whichever is
			// the case, we need to commit() to create a new image.
			logCommit(s.output, -1)
			if imgID, ref, err = s.commit(ctx, ib, s.executor.getCreatedBy(nil), false, s.output); err != nil {
				return "", nil, errors.Wrapf(err, "error committing base container")
			}
		} else {
			// We don't need to squash the base image, so just
			// reuse the base image.
			logCommit(s.output, -1)
			if imgID, ref, err = s.tagExistingImage(ctx, s.builder.FromImageID, s.output); err != nil {
				return "", nil, err
			}
		}
		logImageID(imgID)
	}

	for i, node := range children {
		moreInstructions := i < len(children)-1
		lastInstruction := !moreInstructions
		// Resolve any arguments in this instruction.
		step := ib.Step()
		if err := step.Resolve(node); err != nil {
			return "", nil, errors.Wrapf(err, "error resolving step %+v", *node)
		}
		logrus.Debugf("Parsed Step: %+v", *step)
		if !s.executor.quiet {
			s.executor.log("%s", step.Original)
		}

		// Check if there's a --from if the step command is COPY or
		// ADD.  Set copyFrom to point to either the context directory
		// or the root of the container from the specified stage.
		s.copyFrom = s.executor.contextDir
		for _, n := range step.Flags {
			command := strings.ToUpper(step.Command)
			if strings.Contains(n, "--from") && (command == "COPY" || command == "ADD") {
				var mountPoint string
				arr := strings.Split(n, "=")
				otherStage, ok := s.executor.stages[arr[1]]
				if !ok {
					if mountPoint, err = s.getImageRootfs(ctx, stage, arr[1]); err != nil {
						return "", nil, errors.Errorf("%s --from=%s: no stage or image found with that name", command, arr[1])
					}
				} else {
					mountPoint = otherStage.mountPoint
				}
				s.copyFrom = mountPoint
				break
			}
		}

		// Determine if there are any RUN instructions to be run after
		// this step.  If not, we won't have to bother preserving the
		// contents of any volumes declared between now and when we
		// finish.
		noRunsRemaining := false
		if moreInstructions {
			noRunsRemaining = !ib.RequiresStart(&parser.Node{Children: children[i+1:]})
		}

		// If we're doing a single-layer build, just process the
		// instruction.
		if !s.executor.layers {
			err := ib.Run(step, s, noRunsRemaining)
			if err != nil {
				logrus.Debugf("%v", errors.Wrapf(err, "error building at step %+v", *step))
				return "", nil, errors.Wrapf(err, "error building at STEP \"%s\"", step.Message)
			}
			if moreInstructions {
				// There are still more instructions to process
				// for this stage.  Make a note of the
				// instruction in the history that we'll write
				// for the image when we eventually commit it.
				now := time.Now()
				s.builder.AddPrependedEmptyLayer(&now, s.executor.getCreatedBy(node), "", "")
				continue
			} else {
				// This is the last instruction for this stage,
				// so we should commit this container to create
				// an image, but only if it's the last one, or
				// if it's used as the basis for a later stage.
				if lastStage || imageIsUsedLater {
					logCommit(s.output, i)
					imgID, ref, err = s.commit(ctx, ib, s.executor.getCreatedBy(node), false, s.output)
					if err != nil {
						return "", nil, errors.Wrapf(err, "error committing container for step %+v", *step)
					}
					logImageID(imgID)
				} else {
					imgID = ""
				}
				break
			}
		}

		// We're in a multi-layered build.
		var (
			commitName string
			cacheID    string
			err        error
			rebase     bool
		)

		// If we have to commit for this instruction, only assign the
		// stage's configured output name to the last layer.
		if lastInstruction {
			commitName = s.output
		}

		// If we're using the cache, and we've managed to stick with
		// cached images so far, look for one that matches what we
		// expect to produce for this instruction.
		if checkForLayers && !(s.executor.squash && lastInstruction && lastStage) {
			cacheID, err = s.layerExists(ctx, node)
			if err != nil {
				return "", nil, errors.Wrap(err, "error checking if cached image exists from a previous build")
			}
			if cacheID != "" {
				// Note the cache hit.
				fmt.Fprintf(s.executor.out, "--> Using cache %s\n", cacheID)
			} else {
				// We're not going to find any more cache hits.
				checkForLayers = false
			}
		}

		if cacheID != "" {
			// A suitable cached image was found, so just reuse it.
			// If we need to name the resulting image because it's
			// the last step in this stage, add the name to the
			// image.
			imgID = cacheID
			if commitName != "" {
				logCommit(commitName, i)
				if imgID, ref, err = s.tagExistingImage(ctx, cacheID, commitName); err != nil {
					return "", nil, err
				}
				logImageID(imgID)
			}
			// Update our working container to be based off of the
			// cached image, if we might need to use it as a basis
			// for the next instruction, or if we need the root
			// filesystem to match the image contents for the sake
			// of a later stage that wants to copy content from it.
			rebase = moreInstructions || rootfsIsUsedLater
			// If the instruction would affect our configuration,
			// process the configuration change so that, if we fall
			// off the cache path, the filesystem changes from the
			// last cache image will be all that we need, since we
			// still don't want to restart using the image's
			// configuration blob.
			if !s.stepRequiresLayer(step) {
				err := ib.Run(step, s, noRunsRemaining)
				if err != nil {
					logrus.Debugf("%v", errors.Wrapf(err, "error building at step %+v", *step))
					return "", nil, errors.Wrapf(err, "error building at STEP \"%s\"", step.Message)
				}
			}
		} else {
			// If we didn't find a cached image that we could just reuse,
			// process the instruction directly.
			err := ib.Run(step, s, noRunsRemaining)
			if err != nil {
				logrus.Debugf("%v", errors.Wrapf(err, "error building at step %+v", *step))
				return "", nil, errors.Wrapf(err, "error building at STEP \"%s\"", step.Message)
			}
			// Create a new image, maybe with a new layer.
			logCommit(s.output, i)
			imgID, ref, err = s.commit(ctx, ib, s.executor.getCreatedBy(node), !s.stepRequiresLayer(step), commitName)
			if err != nil {
				return "", nil, errors.Wrapf(err, "error committing container for step %+v", *step)
			}
			logImageID(imgID)
			// We only need to build a new container rootfs
			// using this image if we plan on making
			// further changes to it.  Subsequent stages
			// that just want to use the rootfs as a source
			// for COPY or ADD will be content with what we
			// already have.
			rebase = moreInstructions
		}

		if rebase {
			// Since we either committed the working container or
			// are about to replace it with one based on a cached
			// image, add the current working container's ID to the
			// list of successful intermediate containers that
			// we'll clean up later.
			s.containerIDs = append(s.containerIDs, s.builder.ContainerID)

			// Prepare for the next step or subsequent phases by
			// creating a new working container with the
			// just-committed or updated cached image as its new
			// base image.
			if _, err := s.prepare(ctx, stage, imgID, false, true); err != nil {
				return "", nil, errors.Wrap(err, "error preparing container for next step")
			}
		}
	}

	return imgID, ref, nil
}

// tagExistingImage adds names to an image already in the store
func (s *StageExecutor) tagExistingImage(ctx context.Context, cacheID, output string) (string, reference.Canonical, error) {
	// If we don't need to attach a name to the image, just return the cache ID.
	if output == "" {
		return cacheID, nil, nil
	}

	// Get the destination image reference.
	dest, err := s.executor.resolveNameToImageRef(output)
	if err != nil {
		return "", nil, err
	}

	policyContext, err := util.GetPolicyContext(s.executor.systemContext)
	if err != nil {
		return "", nil, err
	}
	defer func() {
		if destroyErr := policyContext.Destroy(); destroyErr != nil {
			if err == nil {
				err = destroyErr
			} else {
				err = errors.Wrap(err, destroyErr.Error())
			}
		}
	}()

	// Look up the source image, expecting it to be in local storage
	src, err := is.Transport.ParseStoreReference(s.executor.store, cacheID)
	if err != nil {
		return "", nil, errors.Wrapf(err, "error getting source imageReference for %q", cacheID)
	}
	manifestBytes, err := cp.Image(ctx, policyContext, dest, src, nil)
	if err != nil {
		return "", nil, errors.Wrapf(err, "error copying image %q", cacheID)
	}
	manifestDigest, err := manifest.Digest(manifestBytes)
	if err != nil {
		return "", nil, errors.Wrapf(err, "error computing digest of manifest for image %q", cacheID)
	}
	img, err := is.Transport.GetStoreImage(s.executor.store, dest)
	if err != nil {
		return "", nil, errors.Wrapf(err, "error locating new copy of image %q (i.e., %q)", cacheID, transports.ImageName(dest))
	}
	var ref reference.Canonical
	if dref := dest.DockerReference(); dref != nil {
		if ref, err = reference.WithDigest(dref, manifestDigest); err != nil {
			return "", nil, errors.Wrapf(err, "error computing canonical reference for new image %q (i.e., %q)", cacheID, transports.ImageName(dest))
		}
	}
	return img.ID, ref, nil
}

// layerExists returns true if an intermediate image of currNode exists in the image store from a previous build.
// It verifies this by checking the parent of the top layer of the image and the history.
func (s *StageExecutor) layerExists(ctx context.Context, currNode *parser.Node) (string, error) {
	// Get the list of images available in the image store
	images, err := s.executor.store.Images()
	if err != nil {
		return "", errors.Wrap(err, "error getting image list from store")
	}
	var baseHistory []v1.History
	if s.builder.FromImageID != "" {
		baseHistory, err = s.executor.getImageHistory(ctx, s.builder.FromImageID)
		if err != nil {
			return "", errors.Wrapf(err, "error getting history of base image %q", s.builder.FromImageID)
		}
	}
	for _, image := range images {
		var imageTopLayer *storage.Layer
		if image.TopLayer != "" {
			imageTopLayer, err = s.executor.store.Layer(image.TopLayer)
			if err != nil {
				return "", errors.Wrapf(err, "error getting top layer info")
			}
		}
		// If the parent of the top layer of an image is equal to the current build image's top layer,
		// it means that this image is potentially a cached intermediate image from a previous
		// build. Next we double check that the history of this image is equivalent to the previous
		// lines in the Dockerfile up till the point we are at in the build.
		if imageTopLayer == nil || (s.builder.TopLayer != "" && (imageTopLayer.Parent == s.builder.TopLayer || imageTopLayer.ID == s.builder.TopLayer)) {
			history, err := s.executor.getImageHistory(ctx, image.ID)
			if err != nil {
				return "", errors.Wrapf(err, "error getting history of %q", image.ID)
			}
			// children + currNode is the point of the Dockerfile we are currently at.
			if s.executor.historyMatches(baseHistory, currNode, history) {
				// This checks if the files copied during build have been changed if the node is
				// a COPY or ADD command.
				filesMatch, err := s.copiedFilesMatch(currNode, history[len(history)-1].Created)
				if err != nil {
					return "", errors.Wrapf(err, "error checking if copied files match")
				}
				if filesMatch {
					return image.ID, nil
				}
			}
		}
	}
	return "", nil
}

// getFilesToCopy goes through node to get all the src files that are copied, added or downloaded.
// It is possible for the Dockerfile to have src as hom*, which means all files that have hom as a prefix.
// Another format is hom?.txt, which means all files that have that name format with the ? replaced by another character.
func (s *StageExecutor) getFilesToCopy(node *parser.Node) ([]string, error) {
	currNode := node.Next
	var src []string
	for currNode.Next != nil {
		if strings.HasPrefix(currNode.Value, "http://") || strings.HasPrefix(currNode.Value, "https://") {
			src = append(src, currNode.Value)
			currNode = currNode.Next
			continue
		}
		matches, err := filepath.Glob(filepath.Join(s.copyFrom, currNode.Value))
		if err != nil {
			return nil, errors.Wrapf(err, "error finding match for pattern %q", currNode.Value)
		}
		src = append(src, matches...)
		currNode = currNode.Next
	}
	return src, nil
}

// copiedFilesMatch checks to see if the node instruction is a COPY or ADD.
// If it is either of those two it checks the timestamps on all the files copied/added
// by the dockerfile. If the host version has a time stamp greater than the time stamp
// of the build, the build will not use the cached version and will rebuild.
func (s *StageExecutor) copiedFilesMatch(node *parser.Node, historyTime *time.Time) (bool, error) {
	if node.Value != "add" && node.Value != "copy" {
		return true, nil
	}

	src, err := s.getFilesToCopy(node)
	if err != nil {
		return false, err
	}
	for _, item := range src {
		// for urls, check the Last-Modified field in the header.
		if strings.HasPrefix(item, "http://") || strings.HasPrefix(item, "https://") {
			urlContentNew, err := urlContentModified(item, historyTime)
			if err != nil {
				return false, err
			}
			if urlContentNew {
				return false, nil
			}
			continue
		}
		// Walks the file tree for local files and uses chroot to ensure we don't escape out of the allowed path
		// when resolving any symlinks.
		// Change the time format to ensure we don't run into a parsing error when converting again from string
		// to time.Time. It is a known Go issue that the conversions cause errors sometimes, so specifying a particular
		// time format here when converting to a string.
		timeIsGreater, err := resolveModifiedTime(s.copyFrom, item, historyTime.Format(time.RFC3339Nano))
		if err != nil {
			return false, errors.Wrapf(err, "error resolving symlinks and comparing modified times: %q", item)
		}
		if timeIsGreater {
			return false, nil
		}
	}
	return true, nil
}

// commit writes the container's contents to an image, using a passed-in tag as
// the name if there is one, generating a unique ID-based one otherwise.
func (s *StageExecutor) commit(ctx context.Context, ib *imagebuilder.Builder, createdBy string, emptyLayer bool, output string) (string, reference.Canonical, error) {
	var imageRef types.ImageReference
	if output != "" {
		imageRef2, err := s.executor.resolveNameToImageRef(output)
		if err != nil {
			return "", nil, err
		}
		imageRef = imageRef2
	}

	if ib.Author != "" {
		s.builder.SetMaintainer(ib.Author)
	}
	config := ib.Config()
	if createdBy != "" {
		s.builder.SetCreatedBy(createdBy)
	}
	s.builder.SetHostname(config.Hostname)
	s.builder.SetDomainname(config.Domainname)
	s.builder.SetUser(config.User)
	s.builder.ClearPorts()
	for p := range config.ExposedPorts {
		s.builder.SetPort(string(p))
	}
	for _, envSpec := range config.Env {
		spec := strings.SplitN(envSpec, "=", 2)
		s.builder.SetEnv(spec[0], spec[1])
	}
	s.builder.SetCmd(config.Cmd)
	s.builder.ClearVolumes()
	for v := range config.Volumes {
		s.builder.AddVolume(v)
	}
	s.builder.ClearOnBuild()
	for _, onBuildSpec := range config.OnBuild {
		s.builder.SetOnBuild(onBuildSpec)
	}
	s.builder.SetWorkDir(config.WorkingDir)
	s.builder.SetEntrypoint(config.Entrypoint)
	s.builder.SetShell(config.Shell)
	s.builder.SetStopSignal(config.StopSignal)
	if config.Healthcheck != nil {
		s.builder.SetHealthcheck(&buildahdocker.HealthConfig{
			Test:        append([]string{}, config.Healthcheck.Test...),
			Interval:    config.Healthcheck.Interval,
			Timeout:     config.Healthcheck.Timeout,
			StartPeriod: config.Healthcheck.StartPeriod,
			Retries:     config.Healthcheck.Retries,
		})
	} else {
		s.builder.SetHealthcheck(nil)
	}
	s.builder.ClearLabels()
	for k, v := range config.Labels {
		s.builder.SetLabel(k, v)
	}
	for _, labelSpec := range s.executor.labels {
		label := strings.SplitN(labelSpec, "=", 2)
		if len(label) > 1 {
			s.builder.SetLabel(label[0], label[1])
		} else {
			s.builder.SetLabel(label[0], "")
		}
	}
	for _, annotationSpec := range s.executor.annotations {
		annotation := strings.SplitN(annotationSpec, "=", 2)
		if len(annotation) > 1 {
			s.builder.SetAnnotation(annotation[0], annotation[1])
		} else {
			s.builder.SetAnnotation(annotation[0], "")
		}
	}
	if imageRef != nil {
		logName := transports.ImageName(imageRef)
		logrus.Debugf("COMMIT %q", logName)
	} else {
		logrus.Debugf("COMMIT")
	}
	writer := s.executor.reportWriter
	if s.executor.layers || !s.executor.useCache {
		writer = nil
	}
	options := buildah.CommitOptions{
		Compression:           s.executor.compression,
		SignaturePolicyPath:   s.executor.signaturePolicyPath,
		ReportWriter:          writer,
		PreferredManifestType: s.executor.outputFormat,
		SystemContext:         s.executor.systemContext,
		Squash:                s.executor.squash,
		EmptyLayer:            emptyLayer,
		BlobDirectory:         s.executor.blobDirectory,
	}
	imgID, _, manifestDigest, err := s.builder.Commit(ctx, imageRef, options)
	if err != nil {
		return "", nil, err
	}
	var ref reference.Canonical
	if imageRef != nil {
		if dref := imageRef.DockerReference(); dref != nil {
			if ref, err = reference.WithDigest(dref, manifestDigest); err != nil {
				return "", nil, errors.Wrapf(err, "error computing canonical reference for new image %q", imgID)
			}
		}
	}
	return imgID, ref, nil
}

func (s *StageExecutor) EnsureContainerPath(path string) error {
	targetPath, err := securejoin.SecureJoin(s.mountPoint, path)
	if err != nil {
		return errors.Wrapf(err, "error ensuring container path %q", path)
	}
	_, err = os.Lstat(targetPath)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(targetPath, 0755)
	}
	if err != nil {
		return errors.Wrapf(err, "error ensuring container path %q", path)
	}
	return nil
}

// urlContentModified sends a get request to the url and checks if the header has a value in
// Last-Modified, and if it does compares the time stamp to that of the history of the cached image.
// returns true if there is no Last-Modified value in the header.
func urlContentModified(url string, historyTime *time.Time) (bool, error) {
	resp, err := http.Get(url)
	if err != nil {
		return false, errors.Wrapf(err, "error getting %q", url)
	}
	defer resp.Body.Close()
	if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" {
		lastModifiedTime, err := time.Parse(time.RFC1123, lastModified)
		if err != nil {
			return false, errors.Wrapf(err, "error parsing time for %q", url)
		}
		return lastModifiedTime.After(*historyTime), nil
	}
	logrus.Debugf("Response header did not have Last-Modified %q, will rebuild.", url)
	return true, nil
}
