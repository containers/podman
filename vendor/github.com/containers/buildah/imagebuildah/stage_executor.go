package imagebuildah

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/buildah/copier"
	"github.com/containers/buildah/define"
	buildahdocker "github.com/containers/buildah/docker"
	"github.com/containers/buildah/pkg/rusage"
	"github.com/containers/buildah/util"
	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/chrootarchive"
	docker "github.com/fsouza/go-dockerclient"
	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
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
	ctx             context.Context
	executor        *Executor
	log             func(format string, args ...interface{})
	index           int
	stages          imagebuilder.Stages
	name            string
	builder         *buildah.Builder
	preserved       int
	volumes         imagebuilder.VolumeSet
	volumeCache     map[string]string
	volumeCacheInfo map[string]os.FileInfo
	mountPoint      string
	output          string
	containerIDs    []string
	stage           *imagebuilder.Stage
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
		createdDirPerms := os.FileMode(0755)
		if err := copier.Mkdir(s.mountPoint, filepath.Join(s.mountPoint, path), copier.MkdirOptions{ChmodNew: &createdDirPerms}); err != nil {
			return errors.Wrapf(err, "error ensuring volume path exists")
		}
		if err := s.volumeCacheInvalidate(path); err != nil {
			return errors.Wrapf(err, "error ensuring volume path %q is preserved", filepath.Join(s.mountPoint, path))
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
	if evaluated, err := copier.Eval(s.mountPoint, filepath.Join(s.mountPoint, path), copier.EvalOptions{}); err == nil {
		symLink, err := filepath.Rel(s.mountPoint, evaluated)
		if err != nil {
			return errors.Wrapf(err, "making evaluated path %q relative to %q", evaluated, s.mountPoint)
		}
		if strings.HasPrefix(symLink, ".."+string(os.PathSeparator)) {
			return errors.Errorf("evaluated path %q was not below %q", evaluated, s.mountPoint)
		}
		archivedPath = evaluated
		path = string(os.PathSeparator) + symLink
	} else {
		return errors.Wrapf(err, "error evaluating path %q", path)
	}

	st, err := os.Stat(archivedPath)
	if os.IsNotExist(err) {
		createdDirPerms := os.FileMode(0755)
		if err = copier.Mkdir(s.mountPoint, archivedPath, copier.MkdirOptions{ChmodNew: &createdDirPerms}); err != nil {
			return errors.Wrapf(err, "error ensuring volume path exists")
		}
		st, err = os.Stat(archivedPath)
	}
	if err != nil {
		logrus.Debugf("error reading info about %q: %v", archivedPath, err)
		return err
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
			return err
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
			return err
		}
		archivedPath := filepath.Join(s.mountPoint, cachedPath)
		logrus.Debugf("invalidated volume cache %q for %q from %q", archivedPath, path, s.volumeCache[cachedPath])
	}
	return nil
}

// Save the contents of each of the executor's list of volumes for which we
// don't already have a cache file.
func (s *StageExecutor) volumeCacheSaveVFS() (mounts []specs.Mount, err error) {
	for cachedPath, cacheFile := range s.volumeCache {
		archivedPath, err := copier.Eval(s.mountPoint, filepath.Join(s.mountPoint, cachedPath), copier.EvalOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "error evaluating volume path")
		}
		relativePath, err := filepath.Rel(s.mountPoint, archivedPath)
		if err != nil {
			return nil, errors.Wrapf(err, "error converting %q into a path relative to %q", archivedPath, s.mountPoint)
		}
		if strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
			return nil, errors.Errorf("error converting %q into a path relative to %q", archivedPath, s.mountPoint)
		}
		_, err = os.Stat(cacheFile)
		if err == nil {
			logrus.Debugf("contents of volume %q are already cached in %q", archivedPath, cacheFile)
			continue
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		createdDirPerms := os.FileMode(0755)
		if err := copier.Mkdir(s.mountPoint, archivedPath, copier.MkdirOptions{ChmodNew: &createdDirPerms}); err != nil {
			return nil, errors.Wrapf(err, "error ensuring volume path exists")
		}
		logrus.Debugf("caching contents of volume %q in %q", archivedPath, cacheFile)
		cache, err := os.Create(cacheFile)
		if err != nil {
			return nil, err
		}
		defer cache.Close()
		rc, err := chrootarchive.Tar(archivedPath, nil, s.mountPoint)
		if err != nil {
			return nil, errors.Wrapf(err, "error archiving %q", archivedPath)
		}
		defer rc.Close()
		_, err = io.Copy(cache, rc)
		if err != nil {
			return nil, errors.Wrapf(err, "error archiving %q to %q", archivedPath, cacheFile)
		}
		mount := specs.Mount{
			Source:      archivedPath,
			Destination: string(os.PathSeparator) + relativePath,
			Type:        "bind",
			Options:     []string{"private"},
		}
		mounts = append(mounts, mount)
	}
	return nil, nil
}

// Restore the contents of each of the executor's list of volumes.
func (s *StageExecutor) volumeCacheRestoreVFS() (err error) {
	for cachedPath, cacheFile := range s.volumeCache {
		archivedPath, err := copier.Eval(s.mountPoint, filepath.Join(s.mountPoint, cachedPath), copier.EvalOptions{})
		if err != nil {
			return errors.Wrapf(err, "error evaluating volume path")
		}
		logrus.Debugf("restoring contents of volume %q from %q", archivedPath, cacheFile)
		cache, err := os.Open(cacheFile)
		if err != nil {
			return err
		}
		defer cache.Close()
		if err := copier.Remove(s.mountPoint, archivedPath, copier.RemoveOptions{All: true}); err != nil {
			return err
		}
		createdDirPerms := os.FileMode(0755)
		if err := copier.Mkdir(s.mountPoint, archivedPath, copier.MkdirOptions{ChmodNew: &createdDirPerms}); err != nil {
			return err
		}
		err = chrootarchive.Untar(cache, archivedPath, nil)
		if err != nil {
			return errors.Wrapf(err, "error extracting archive at %q", archivedPath)
		}
		if st, ok := s.volumeCacheInfo[cachedPath]; ok {
			if err := os.Chmod(archivedPath, st.Mode()); err != nil {
				return err
			}
			uid := 0
			gid := 0
			if st.Sys() != nil {
				uid = util.UID(st)
				gid = util.GID(st)
			}
			if err := os.Chown(archivedPath, uid, gid); err != nil {
				return err
			}
			if err := os.Chtimes(archivedPath, st.ModTime(), st.ModTime()); err != nil {
				return err
			}
		}
	}
	return nil
}

// Save the contents of each of the executor's list of volumes for which we
// don't already have a cache file.
func (s *StageExecutor) volumeCacheSaveOverlay() (mounts []specs.Mount, err error) {
	for cachedPath := range s.volumeCache {
		err = copier.Mkdir(s.mountPoint, filepath.Join(s.mountPoint, cachedPath), copier.MkdirOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "ensuring volume exists")
		}
		volumePath := filepath.Join(s.mountPoint, cachedPath)
		mount := specs.Mount{
			Source:      volumePath,
			Destination: cachedPath,
			Options:     []string{"O", "private"},
		}
		mounts = append(mounts, mount)
	}
	return mounts, nil
}

// Reset the contents of each of the executor's list of volumes.
func (s *StageExecutor) volumeCacheRestoreOverlay() error {
	return nil
}

// Save the contents of each of the executor's list of volumes for which we
// don't already have a cache file.
func (s *StageExecutor) volumeCacheSave() (mounts []specs.Mount, err error) {
	switch s.executor.store.GraphDriverName() {
	case "overlay":
		return s.volumeCacheSaveOverlay()
	}
	return s.volumeCacheSaveVFS()
}

// Reset the contents of each of the executor's list of volumes.
func (s *StageExecutor) volumeCacheRestore() error {
	switch s.executor.store.GraphDriverName() {
	case "overlay":
		return s.volumeCacheRestoreOverlay()
	}
	return s.volumeCacheRestoreVFS()
}

// Copy copies data into the working tree.  The "Download" field is how
// imagebuilder tells us the instruction was "ADD" and not "COPY".
func (s *StageExecutor) Copy(excludes []string, copies ...imagebuilder.Copy) error {
	s.builder.ContentDigester.Restart()
	for _, copy := range copies {
		if copy.Download {
			logrus.Debugf("ADD %#v, %#v", excludes, copy)
		} else {
			logrus.Debugf("COPY %#v, %#v", excludes, copy)
		}
		if err := s.volumeCacheInvalidate(copy.Dest); err != nil {
			return err
		}
		var sources []string
		// The From field says to read the content from another
		// container.  Update the ID mappings and
		// all-content-comes-from-below-this-directory value.
		var idMappingOptions *define.IDMappingOptions
		var copyExcludes []string
		stripSetuid := false
		stripSetgid := false
		preserveOwnership := false
		contextDir := s.executor.contextDir
		if len(copy.From) > 0 {
			// If from has an argument within it, resolve it to its
			// value.  Otherwise just return the value found.
			from, fromErr := imagebuilder.ProcessWord(copy.From, s.stage.Builder.Arguments())
			if fromErr != nil {
				return errors.Wrapf(fromErr, "unable to resolve argument %q", copy.From)
			}
			if isStage, err := s.executor.waitForStage(s.ctx, from, s.stages[:s.index]); isStage && err != nil {
				return err
			}
			if other, ok := s.executor.stages[from]; ok && other.index < s.index {
				contextDir = other.mountPoint
				idMappingOptions = &other.builder.IDMappingOptions
			} else if builder, ok := s.executor.containerMap[copy.From]; ok {
				contextDir = builder.MountPoint
				idMappingOptions = &builder.IDMappingOptions
			} else {
				return errors.Errorf("the stage %q has not been built", copy.From)
			}
			preserveOwnership = true
			copyExcludes = excludes
		} else {
			copyExcludes = append(s.executor.excludes, excludes...)
			stripSetuid = true // did this change between 18.06 and 19.03?
			stripSetgid = true // did this change between 18.06 and 19.03?
		}
		for _, src := range copy.Src {
			if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
				// Source is a URL, allowed for ADD but not COPY.
				if copy.Download {
					sources = append(sources, src)
				} else {
					// returns an error to be compatible with docker
					return errors.Errorf("source can't be a URL for COPY")
				}
			} else {
				sources = append(sources, filepath.Join(contextDir, src))
			}
		}
		options := buildah.AddAndCopyOptions{
			Chmod:             copy.Chmod,
			Chown:             copy.Chown,
			PreserveOwnership: preserveOwnership,
			ContextDir:        contextDir,
			Excludes:          copyExcludes,
			IDMappingOptions:  idMappingOptions,
			StripSetuidBit:    stripSetuid,
			StripSetgidBit:    stripSetgid,
		}
		if err := s.builder.Add(copy.Dest, copy.Download, options, sources...); err != nil {
			return err
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
		Logger:           s.executor.logger,
		Hostname:         config.Hostname,
		Runtime:          s.executor.runtime,
		Args:             s.executor.runtimeArgs,
		NoPivot:          os.Getenv("BUILDAH_NOPIVOT") != "",
		Mounts:           append([]Mount{}, s.executor.transientMounts...),
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
		Terminal:         buildah.WithoutTerminal,
		Secrets:          s.executor.secrets,
		SSHSources:       s.executor.sshsources,
		RunMounts:        run.Mounts,
	}
	if config.NetworkDisabled {
		options.ConfigureNetwork = buildah.NetworkDisabled
	} else {
		options.ConfigureNetwork = buildah.NetworkEnabled
	}

	args := run.Args
	if run.Shell {
		if len(config.Shell) > 0 && s.builder.Format == define.Dockerv2ImageManifest {
			args = append(config.Shell, args...)
		} else {
			args = append([]string{"/bin/sh", "-c"}, args...)
		}
	}
	mounts, err := s.volumeCacheSave()
	if err != nil {
		return err
	}
	options.Mounts = append(options.Mounts, mounts...)
	err = s.builder.Run(args, options)
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
		s.executor.logger.Errorf(errStr)
	case logrus.DebugLevel:
		logrus.Debugf(err)
	default:
		s.executor.logger.Errorf("+(UNHANDLED LOGLEVEL) %#v", step)
	}

	return errors.Errorf(err)
}

// prepare creates a working container based on the specified image, or if one
// isn't specified, the first argument passed to the first FROM instruction we
// can find in the stage's parsed tree.
func (s *StageExecutor) prepare(ctx context.Context, from string, initializeIBConfig, rebase bool, pullPolicy define.PullPolicy) (builder *buildah.Builder, err error) {
	stage := s.stage
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
			s.log("FROM %s", displayFrom)
		}
	}

	builderOptions := buildah.BuilderOptions{
		Args:                  ib.Args,
		FromImage:             from,
		PullPolicy:            pullPolicy,
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
		Capabilities:          s.executor.capabilities,
		Devices:               s.executor.devices,
		MaxPullRetries:        s.executor.maxPullPushRetries,
		PullRetryDelay:        s.executor.retryPullPushDelay,
		OciDecryptConfig:      s.executor.ociDecryptConfig,
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
func (s *StageExecutor) getImageRootfs(ctx context.Context, image string) (mountPoint string, err error) {
	if builder, ok := s.executor.containerMap[image]; ok {
		return builder.MountPoint, nil
	}
	builder, err := s.prepare(ctx, image, false, false, s.executor.pullPolicy)
	if err != nil {
		return "", err
	}
	s.executor.containerMap[image] = builder
	return builder.MountPoint, nil
}

// Execute runs each of the steps in the stage's parsed tree, in turn.
func (s *StageExecutor) Execute(ctx context.Context, base string) (imgID string, ref reference.Canonical, err error) {
	var resourceUsage rusage.Rusage
	stage := s.stage
	ib := stage.Builder
	checkForLayers := s.executor.layers && s.executor.useCache
	moreStages := s.index < len(s.stages)-1
	lastStage := !moreStages
	imageIsUsedLater := moreStages && (s.executor.baseMap[stage.Name] || s.executor.baseMap[fmt.Sprintf("%d", stage.Position)])
	rootfsIsUsedLater := moreStages && (s.executor.rootfsMap[stage.Name] || s.executor.rootfsMap[fmt.Sprintf("%d", stage.Position)])

	// If the base image's name corresponds to the result of an earlier
	// stage, make sure that stage has finished building an image, and
	// substitute that image's ID for the base image's name here and force
	// the pull policy to "never" to avoid triggering an error when it's
	// set to "always", which doesn't make sense for image IDs.
	// If not, then go on assuming that it's just a regular image that's
	// either in local storage, or one that we have to pull from a
	// registry, subject to the passed-in pull policy.
	if isStage, err := s.executor.waitForStage(ctx, base, s.stages[:s.index]); isStage && err != nil {
		return "", nil, err
	}
	pullPolicy := s.executor.pullPolicy
	s.executor.stagesLock.Lock()
	if stageImage, isPreviousStage := s.executor.imageMap[base]; isPreviousStage {
		base = stageImage
		pullPolicy = define.PullNever
	}
	s.executor.stagesLock.Unlock()

	// Set things up so that we can log resource usage as we go.
	logRusage := func() {
		if rusage.Supported() {
			usage, err := rusage.Get()
			if err != nil {
				fmt.Fprintf(s.executor.out, "error gathering resource usage information: %v\n", err)
				return
			}
			if s.executor.rusageLogFile != nil {
				fmt.Fprintf(s.executor.rusageLogFile, "%s\n", rusage.FormatDiff(usage.Subtract(resourceUsage)))
			}
			resourceUsage = usage
		}
	}

	// Start counting resource usage before we potentially pull a base image.
	if rusage.Supported() {
		if resourceUsage, err = rusage.Get(); err != nil {
			return "", nil, err
		}
		// Log the final incremental resource usage counter before we return.
		defer logRusage()
	}

	// Create the (first) working container for this stage.  Reinitializing
	// the imagebuilder configuration may alter the list of steps we have,
	// so take a snapshot of them *after* that.
	if _, err := s.prepare(ctx, base, true, true, pullPolicy); err != nil {
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
			s.log(commitMessage)
		}
	}
	logCacheHit := func(cacheID string) {
		if !s.executor.quiet {
			cacheHitMessage := "--> Using cache"
			fmt.Fprintf(s.executor.out, "%s %s\n", cacheHitMessage, cacheID)
		}
	}
	logImageID := func(imgID string) {
		if len(imgID) > 11 {
			imgID = imgID[0:11]
		}
		if s.executor.iidfile == "" {
			fmt.Fprintf(s.executor.out, "--> %s\n", imgID)
		}
	}

	if len(children) == 0 {
		// There are no steps.
		if s.builder.FromImageID == "" || s.executor.squash {
			// We either don't have a base image, or we need to
			// squash the contents of the base image.  Whichever is
			// the case, we need to commit() to create a new image.
			logCommit(s.output, -1)
			if imgID, ref, err = s.commit(ctx, s.getCreatedBy(nil, ""), false, s.output); err != nil {
				return "", nil, errors.Wrapf(err, "error committing base container")
			}
		} else if len(s.executor.labels) > 0 || len(s.executor.annotations) > 0 {
			// The image would be modified by the labels passed
			// via the command line, so we need to commit.
			logCommit(s.output, -1)
			if imgID, ref, err = s.commit(ctx, s.getCreatedBy(stage.Node, ""), true, s.output); err != nil {
				return "", nil, err
			}
		} else {
			// We don't need to squash the base image, and the
			// image wouldn't be modified by the command line
			// options, so just reuse the base image.
			logCommit(s.output, -1)
			if imgID, ref, err = s.tagExistingImage(ctx, s.builder.FromImageID, s.output); err != nil {
				return "", nil, err
			}
		}
		logImageID(imgID)
	}

	for i, node := range children {
		logRusage()
		moreInstructions := i < len(children)-1
		lastInstruction := !moreInstructions
		// Resolve any arguments in this instruction.
		step := ib.Step()
		if err := step.Resolve(node); err != nil {
			return "", nil, errors.Wrapf(err, "error resolving step %+v", *node)
		}
		logrus.Debugf("Parsed Step: %+v", *step)
		if !s.executor.quiet {
			s.log("%s", step.Original)
		}

		// Check if there's a --from if the step command is COPY.
		// Also check the chmod and the chown flags for validity.
		for _, flag := range step.Flags {
			command := strings.ToUpper(step.Command)
			// chmod, chown and from flags should have an '=' sign, '--chmod=', '--chown=' or '--from='
			if command == "COPY" && (flag == "--chmod" || flag == "--chown" || flag == "--from") {
				return "", nil, errors.Errorf("COPY only supports the --chmod=<permissions> --chown=<uid:gid> and the --from=<image|stage> flags")
			}
			if command == "ADD" && (flag == "--chmod" || flag == "--chown") {
				return "", nil, errors.Errorf("ADD only supports the --chmod=<permissions> and the --chown=<uid:gid> flags")
			}
			if strings.Contains(flag, "--from") && command == "COPY" {
				arr := strings.Split(flag, "=")
				if len(arr) != 2 {
					return "", nil, errors.Errorf("%s: invalid --from flag, should be --from=<name|stage>", command)
				}
				// If arr[1] has an argument within it, resolve it to its
				// value.  Otherwise just return the value found.
				from, fromErr := imagebuilder.ProcessWord(arr[1], s.stage.Builder.Arguments())
				if fromErr != nil {
					return "", nil, errors.Wrapf(fromErr, "unable to resolve argument %q", arr[1])
				}
				// If the source's name corresponds to the
				// result of an earlier stage, wait for that
				// stage to finish being built.
				if isStage, err := s.executor.waitForStage(ctx, from, s.stages[:s.index]); isStage && err != nil {
					return "", nil, err
				}
				if otherStage, ok := s.executor.stages[from]; ok && otherStage.index < s.index {
					break
				} else if _, err = s.getImageRootfs(ctx, from); err != nil {
					return "", nil, errors.Errorf("%s --from=%s: no stage or image found with that name", command, from)
				}
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
			// In case we added content, retrieve its digest.
			addedContentType, addedContentDigest := s.builder.ContentDigester.Digest()
			addedContentSummary := addedContentType
			if addedContentDigest != "" {
				if addedContentSummary != "" {
					addedContentSummary = addedContentSummary + ":"
				}
				addedContentSummary = addedContentSummary + addedContentDigest.Encoded()
				logrus.Debugf("added content %s", addedContentSummary)
			}
			if moreInstructions {
				// There are still more instructions to process
				// for this stage.  Make a note of the
				// instruction in the history that we'll write
				// for the image when we eventually commit it.
				timestamp := time.Now().UTC()
				if s.executor.timestamp != nil {
					timestamp = *s.executor.timestamp
				}
				s.builder.AddPrependedEmptyLayer(&timestamp, s.getCreatedBy(node, addedContentSummary), "", "")
				continue
			} else {
				// This is the last instruction for this stage,
				// so we should commit this container to create
				// an image, but only if it's the last stage,
				// or if it's used as the basis for a later
				// stage.
				if lastStage || imageIsUsedLater {
					logCommit(s.output, i)
					imgID, ref, err = s.commit(ctx, s.getCreatedBy(node, addedContentSummary), false, s.output)
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
			commitName          string
			cacheID             string
			err                 error
			rebase              bool
			addedContentSummary string
		)

		// If we have to commit for this instruction, only assign the
		// stage's configured output name to the last layer.
		if lastInstruction {
			commitName = s.output
		}

		// Check if there's already an image based on our parent that
		// has the same change that we're about to make, so far as we
		// can tell.
		// Only do this if the step we are on is not an ARG step,
		// we need to call ib.Run() to correctly put the args together before
		// determining if a cached layer with the same build args already exists
		// and that is done in the if block below.
		if checkForLayers && step.Command != "arg" {
			cacheID, err = s.intermediateImageExists(ctx, node, addedContentSummary, s.stepRequiresLayer(step))
			if err != nil {
				return "", nil, errors.Wrap(err, "error checking if cached image exists from a previous build")
			}
		}

		// If we didn't find a cache entry, or we need to add content
		// to find the digest of the content to check for a cached
		// image, run the step so that we can check if the result
		// matches a cache.
		if cacheID == "" {
			// Process the instruction directly.
			if err = ib.Run(step, s, noRunsRemaining); err != nil {
				logrus.Debugf("%v", errors.Wrapf(err, "error building at step %+v", *step))
				return "", nil, errors.Wrapf(err, "error building at STEP \"%s\"", step.Message)
			}

			// In case we added content, retrieve its digest.
			addedContentType, addedContentDigest := s.builder.ContentDigester.Digest()
			addedContentSummary = addedContentType
			if addedContentDigest != "" {
				if addedContentSummary != "" {
					addedContentSummary = addedContentSummary + ":"
				}
				addedContentSummary = addedContentSummary + addedContentDigest.Encoded()
				logrus.Debugf("added content %s", addedContentSummary)
			}

			// Check if there's already an image based on our parent that
			// has the same change that we just made.
			if checkForLayers {
				cacheID, err = s.intermediateImageExists(ctx, node, addedContentSummary, s.stepRequiresLayer(step))
				if err != nil {
					return "", nil, errors.Wrap(err, "error checking if cached image exists from a previous build")
				}
			}
		} else {
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
		}

		if cacheID != "" && !(s.executor.squash && lastInstruction) {
			logCacheHit(cacheID)
			// A suitable cached image was found, so we can just
			// reuse it.  If we need to add a name to the resulting
			// image because it's the last step in this stage, add
			// the name to the image.
			imgID = cacheID
			if commitName != "" {
				logCommit(commitName, i)
				if imgID, ref, err = s.tagExistingImage(ctx, cacheID, commitName); err != nil {
					return "", nil, err
				}
			}
		} else {
			// We're not going to find any more cache hits, so we
			// can stop looking for them.
			checkForLayers = false
			// Create a new image, maybe with a new layer, with the
			// name for this stage if it's the last instruction.
			logCommit(s.output, i)
			imgID, ref, err = s.commit(ctx, s.getCreatedBy(node, addedContentSummary), !s.stepRequiresLayer(step), commitName)
			if err != nil {
				return "", nil, errors.Wrapf(err, "error committing container for step %+v", *step)
			}
		}
		logImageID(imgID)

		// Update our working container to be based off of the cached
		// image, if we might need to use it as a basis for the next
		// instruction, or if we need the root filesystem to match the
		// image contents for the sake of a later stage that wants to
		// copy content from it.
		rebase = moreInstructions || rootfsIsUsedLater

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
			// Enforce pull "never" since we already have an image
			// ID that we really should not be pulling anymore (see
			// containers/podman/issues/10307).
			if _, err := s.prepare(ctx, imgID, false, true, define.PullNever); err != nil {
				return "", nil, errors.Wrap(err, "error preparing container for next step")
			}
		}
	}
	return imgID, ref, nil
}

func historyEntriesEqual(base, derived v1.History) bool {
	if base.CreatedBy != derived.CreatedBy {
		return false
	}
	if base.Comment != derived.Comment {
		return false
	}
	if base.Author != derived.Author {
		return false
	}
	if base.EmptyLayer != derived.EmptyLayer {
		return false
	}
	if base.Created != nil && derived.Created == nil {
		return false
	}
	if base.Created == nil && derived.Created != nil {
		return false
	}
	if base.Created != nil && derived.Created != nil && !base.Created.Equal(*derived.Created) {
		return false
	}
	return true
}

// historyAndDiffIDsMatch returns true if a candidate history matches the
// history of our base image (if we have one), plus the current instruction,
// and if the list of diff IDs for the images do for the part of the history
// that we're comparing.
// Used to verify whether a cache of the intermediate image exists and whether
// to run the build again.
func (s *StageExecutor) historyAndDiffIDsMatch(baseHistory []v1.History, baseDiffIDs []digest.Digest, child *parser.Node, history []v1.History, diffIDs []digest.Digest, addedContentSummary string, buildAddsLayer bool) bool {
	// our history should be as long as the base's, plus one entry for what
	// we're doing
	if len(history) != len(baseHistory)+1 {
		return false
	}
	// check that each entry in the base history corresponds to an entry in
	// our history, and count how many of them add a layer diff
	expectedDiffIDs := 0
	for i := range baseHistory {
		if !historyEntriesEqual(baseHistory[i], history[i]) {
			return false
		}
		if !baseHistory[i].EmptyLayer {
			expectedDiffIDs++
		}
	}
	if len(baseDiffIDs) != expectedDiffIDs {
		return false
	}
	if buildAddsLayer {
		// we're adding a layer, so we should have exactly one more
		// layer than the base image
		if len(diffIDs) != expectedDiffIDs+1 {
			return false
		}
	} else {
		// we're not adding a layer, so we should have exactly the same
		// layers as the base image
		if len(diffIDs) != expectedDiffIDs {
			return false
		}
	}
	// compare the diffs for the layers that we should have in common
	for i := range baseDiffIDs {
		if diffIDs[i] != baseDiffIDs[i] {
			return false
		}
	}
	return history[len(baseHistory)].CreatedBy == s.getCreatedBy(child, addedContentSummary)
}

// getCreatedBy returns the command the image at node will be created by.  If
// the passed-in CompositeDigester is not nil, it is assumed to have the digest
// information for the content if the node is ADD or COPY.
func (s *StageExecutor) getCreatedBy(node *parser.Node, addedContentSummary string) string {
	if node == nil {
		return "/bin/sh"
	}
	switch strings.ToUpper(node.Value) {
	case "ARG":
		buildArgs := s.getBuildArgs()
		return "/bin/sh -c #(nop) ARG " + buildArgs
	case "RUN":
		buildArgs := s.getBuildArgs()
		if buildArgs != "" {
			return "|" + strconv.Itoa(len(strings.Split(buildArgs, " "))) + " " + buildArgs + " /bin/sh -c " + node.Original[4:]
		}
		return "/bin/sh -c " + node.Original[4:]
	case "ADD", "COPY":
		destination := node
		for destination.Next != nil {
			destination = destination.Next
		}
		return "/bin/sh -c #(nop) " + strings.ToUpper(node.Value) + " " + addedContentSummary + " in " + destination.Value + " "
	default:
		return "/bin/sh -c #(nop) " + node.Original
	}
}

// getBuildArgs returns a string of the build-args specified during the build process
// it excludes any build-args that were not used in the build process
func (s *StageExecutor) getBuildArgs() string {
	buildArgs := s.stage.Builder.Arguments()
	sort.Strings(buildArgs)
	return strings.Join(buildArgs, " ")
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
	options := cp.Options{
		RemoveSignatures: true, // more like "ignore signatures", since they don't get removed when src and dest are the same image
	}
	manifestBytes, err := cp.Image(ctx, policyContext, dest, src, &options)
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

// intermediateImageExists returns true if an intermediate image of currNode exists in the image store from a previous build.
// It verifies this by checking the parent of the top layer of the image and the history.
func (s *StageExecutor) intermediateImageExists(ctx context.Context, currNode *parser.Node, addedContentDigest string, buildAddsLayer bool) (string, error) {
	// Get the list of images available in the image store
	images, err := s.executor.store.Images()
	if err != nil {
		return "", errors.Wrap(err, "error getting image list from store")
	}
	var baseHistory []v1.History
	var baseDiffIDs []digest.Digest
	if s.builder.FromImageID != "" {
		_, baseHistory, baseDiffIDs, err = s.executor.getImageTypeAndHistoryAndDiffIDs(ctx, s.builder.FromImageID)
		if err != nil {
			return "", errors.Wrapf(err, "error getting history of base image %q", s.builder.FromImageID)
		}
	}
	for _, image := range images {
		var imageTopLayer *storage.Layer
		var imageParentLayerID string
		if image.TopLayer != "" {
			imageTopLayer, err = s.executor.store.Layer(image.TopLayer)
			if err != nil {
				return "", errors.Wrapf(err, "error getting top layer info")
			}
			// Figure out which layer from this image we should
			// compare our container's base layer to.
			imageParentLayerID = imageTopLayer.ID
			// If we haven't added a layer here, then our base
			// layer should be the same as the image's layer.  If
			// did add a layer, then our base layer should be the
			// same as the parent of the image's layer.
			if buildAddsLayer {
				imageParentLayerID = imageTopLayer.Parent
			}
		}
		// If the parent of the top layer of an image is equal to the current build image's top layer,
		// it means that this image is potentially a cached intermediate image from a previous
		// build.
		if s.builder.TopLayer != imageParentLayerID {
			continue
		}
		// Next we double check that the history of this image is equivalent to the previous
		// lines in the Dockerfile up till the point we are at in the build.
		manifestType, history, diffIDs, err := s.executor.getImageTypeAndHistoryAndDiffIDs(ctx, image.ID)
		if err != nil {
			// It's possible that this image is for another architecture, which results
			// in a custom-crafted error message that we'd have to use substring matching
			// to recognize.  Instead, ignore the image.
			logrus.Debugf("error getting history of %q (%v), ignoring it", image.ID, err)
			continue
		}
		// If this candidate isn't of the type that we're building, then it may have lost
		// some format-specific information that a building-without-cache run wouldn't lose.
		if manifestType != s.executor.outputFormat {
			continue
		}
		// children + currNode is the point of the Dockerfile we are currently at.
		if s.historyAndDiffIDsMatch(baseHistory, baseDiffIDs, currNode, history, diffIDs, addedContentDigest, buildAddsLayer) {
			return image.ID, nil
		}
	}
	return "", nil
}

// commit writes the container's contents to an image, using a passed-in tag as
// the name if there is one, generating a unique ID-based one otherwise.
func (s *StageExecutor) commit(ctx context.Context, createdBy string, emptyLayer bool, output string) (string, reference.Canonical, error) {
	ib := s.stage.Builder
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
	if s.executor.architecture != "" {
		s.builder.SetArchitecture(s.executor.architecture)
	}
	if s.executor.os != "" {
		s.builder.SetOS(s.executor.os)
	}
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
	s.builder.SetLabel(buildah.BuilderIdentityAnnotation, define.Version)
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
		SignBy:                s.executor.signBy,
		MaxRetries:            s.executor.maxPullPushRetries,
		RetryDelay:            s.executor.retryPullPushDelay,
		HistoryTimestamp:      s.executor.timestamp,
		Manifest:              s.executor.manifest,
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
	return copier.Mkdir(s.mountPoint, filepath.Join(s.mountPoint, path), copier.MkdirOptions{})
}
