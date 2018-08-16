package imagebuildah

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cp "github.com/containers/image/copy"
	is "github.com/containers/image/storage"
	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/stringid"
	"github.com/docker/docker/builder/dockerfile/parser"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/imagebuilder"
	"github.com/pkg/errors"
	"github.com/projectatomic/buildah"
	"github.com/projectatomic/buildah/util"
	"github.com/sirupsen/logrus"
)

const (
	PullIfMissing       = buildah.PullIfMissing
	PullAlways          = buildah.PullAlways
	PullNever           = buildah.PullNever
	OCIv1ImageFormat    = buildah.OCIv1ImageManifest
	Dockerv2ImageFormat = buildah.Dockerv2ImageManifest

	Gzip         = archive.Gzip
	Bzip2        = archive.Bzip2
	Xz           = archive.Xz
	Uncompressed = archive.Uncompressed
)

// Mount is a mountpoint for the build container.
type Mount specs.Mount

// BuildOptions can be used to alter how an image is built.
type BuildOptions struct {
	// ContextDirectory is the default source location for COPY and ADD
	// commands.
	ContextDirectory string
	// PullPolicy controls whether or not we pull images.  It should be one
	// of PullIfMissing, PullAlways, or PullNever.
	PullPolicy buildah.PullPolicy
	// Registry is a value which is prepended to the image's name, if it
	// needs to be pulled and the image name alone can not be resolved to a
	// reference to a source image.  No separator is implicitly added.
	Registry string
	// Transport is a value which is prepended to the image's name, if it
	// needs to be pulled and the image name alone, or the image name and
	// the registry together, can not be resolved to a reference to a
	// source image.  No separator is implicitly added.
	Transport string
	// IgnoreUnrecognizedInstructions tells us to just log instructions we
	// don't recognize, and try to keep going.
	IgnoreUnrecognizedInstructions bool
	// Quiet tells us whether or not to announce steps as we go through them.
	Quiet bool
	// Isolation controls how Run() runs things.
	Isolation buildah.Isolation
	// Runtime is the name of the command to run for RUN instructions when
	// Isolation is either IsolationDefault or IsolationOCI.  It should
	// accept the same arguments and flags that runc does.
	Runtime string
	// RuntimeArgs adds global arguments for the runtime.
	RuntimeArgs []string
	// TransientMounts is a list of mounts that won't be kept in the image.
	TransientMounts []Mount
	// Compression specifies the type of compression which is applied to
	// layer blobs.  The default is to not use compression, but
	// archive.Gzip is recommended.
	Compression archive.Compression
	// Arguments which can be interpolated into Dockerfiles
	Args map[string]string
	// Name of the image to write to.
	Output string
	// Additional tags to add to the image that we write, if we know of a
	// way to add them.
	AdditionalTags []string
	// Log is a callback that will print a progress message.  If no value
	// is supplied, the message will be sent to Err (or os.Stderr, if Err
	// is nil) by default.
	Log func(format string, args ...interface{})
	// In is connected to stdin for RUN instructions.
	In io.Reader
	// Out is a place where non-error log messages are sent.
	Out io.Writer
	// Err is a place where error log messages should be sent.
	Err io.Writer
	// SignaturePolicyPath specifies an override location for the signature
	// policy which should be used for verifying the new image as it is
	// being written.  Except in specific circumstances, no value should be
	// specified, indicating that the shared, system-wide default policy
	// should be used.
	SignaturePolicyPath string
	// ReportWriter is an io.Writer which will be used to report the
	// progress of the (possible) pulling of the source image and the
	// writing of the new image.
	ReportWriter io.Writer
	// OutputFormat is the format of the output image's manifest and
	// configuration data.
	// Accepted values are OCIv1ImageFormat and Dockerv2ImageFormat.
	OutputFormat string
	// SystemContext holds parameters used for authentication.
	SystemContext *types.SystemContext
	// NamespaceOptions controls how we set up namespaces processes that we
	// might need when handling RUN instructions.
	NamespaceOptions []buildah.NamespaceOption
	// ConfigureNetwork controls whether or not network interfaces and
	// routing are configured for a new network namespace (i.e., when not
	// joining another's namespace and not just using the host's
	// namespace), effectively deciding whether or not the process has a
	// usable network.
	ConfigureNetwork buildah.NetworkConfigurationPolicy
	// CNIPluginPath is the location of CNI plugin helpers, if they should be
	// run from a location other than the default location.
	CNIPluginPath string
	// CNIConfigDir is the location of CNI configuration files, if the files in
	// the default configuration directory shouldn't be used.
	CNIConfigDir string
	// ID mapping options to use if we're setting up our own user namespace
	// when handling RUN instructions.
	IDMappingOptions *buildah.IDMappingOptions
	// AddCapabilities is a list of capabilities to add to the default set when
	// handling RUN instructions.
	AddCapabilities []string
	// DropCapabilities is a list of capabilities to remove from the default set
	// when handling RUN instructions. If a capability appears in both lists, it
	// will be dropped.
	DropCapabilities []string
	CommonBuildOpts  *buildah.CommonBuildOptions
	// DefaultMountsFilePath is the file path holding the mounts to be mounted in "host-path:container-path" format
	DefaultMountsFilePath string
	// IIDFile tells the builder to write the image ID to the specified file
	IIDFile string
	// Squash tells the builder to produce an image with a single layer
	// instead of with possibly more than one layer.
	Squash bool
	// Labels metadata for an image
	Labels []string
	// Annotation metadata for an image
	Annotations []string
	// OnBuild commands to be run by images based on this image
	OnBuild []string
	// Layers tells the builder to create a cache of images for each step in the Dockerfile
	Layers bool
	// NoCache tells the builder to build the image from scratch without checking for a cache.
	// It creates a new set of cached images for the build.
	NoCache bool
	// RemoveIntermediateCtrs tells the builder whether to remove intermediate containers used
	// during the build process. Default is true.
	RemoveIntermediateCtrs bool
	// ForceRmIntermediateCtrs tells the builder to remove all intermediate containers even if
	// the build was unsuccessful.
	ForceRmIntermediateCtrs bool
}

// Executor is a buildah-based implementation of the imagebuilder.Executor
// interface.
type Executor struct {
	index                          int
	name                           string
	named                          map[string]*Executor
	store                          storage.Store
	contextDir                     string
	builder                        *buildah.Builder
	pullPolicy                     buildah.PullPolicy
	registry                       string
	transport                      string
	ignoreUnrecognizedInstructions bool
	quiet                          bool
	runtime                        string
	runtimeArgs                    []string
	transientMounts                []Mount
	compression                    archive.Compression
	output                         string
	outputFormat                   string
	additionalTags                 []string
	log                            func(format string, args ...interface{})
	in                             io.Reader
	out                            io.Writer
	err                            io.Writer
	signaturePolicyPath            string
	systemContext                  *types.SystemContext
	mountPoint                     string
	preserved                      int
	volumes                        imagebuilder.VolumeSet
	volumeCache                    map[string]string
	volumeCacheInfo                map[string]os.FileInfo
	reportWriter                   io.Writer
	isolation                      buildah.Isolation
	namespaceOptions               []buildah.NamespaceOption
	configureNetwork               buildah.NetworkConfigurationPolicy
	cniPluginPath                  string
	cniConfigDir                   string
	idmappingOptions               *buildah.IDMappingOptions
	commonBuildOptions             *buildah.CommonBuildOptions
	defaultMountsFilePath          string
	iidfile                        string
	squash                         bool
	labels                         []string
	annotations                    []string
	onbuild                        []string
	layers                         bool
	topLayers                      []string
	noCache                        bool
	removeIntermediateCtrs         bool
	forceRmIntermediateCtrs        bool
	containerIDs                   []string // Stores the IDs of the successful intermediate containers used during layer build
}

// withName creates a new child executor that will be used whenever a COPY statement uses --from=NAME.
func (b *Executor) withName(name string, index int) *Executor {
	if b.named == nil {
		b.named = make(map[string]*Executor)
	}
	copied := *b
	copied.index = index
	copied.name = name
	child := &copied
	b.named[name] = child
	if idx := strconv.Itoa(index); idx != name {
		b.named[idx] = child
	}
	return child
}

// Preserve informs the executor that from this point on, it needs to ensure
// that only COPY and ADD instructions can modify the contents of this
// directory or anything below it.
// The Executor handles this by caching the contents of directories which have
// been marked this way before executing a RUN instruction, invalidating that
// cache when an ADD or COPY instruction sets any location under the directory
// as the destination, and using the cache to reset the contents of the
// directory tree after processing each RUN instruction.
// It would be simpler if we could just mark the directory as a read-only bind
// mount of itself during Run(), but the directory is expected to be remain
// writeable, even if any changes within it are ultimately discarded.
func (b *Executor) Preserve(path string) error {
	logrus.Debugf("PRESERVE %q", path)
	if b.volumes.Covers(path) {
		// This path is already a subdirectory of a volume path that
		// we're already preserving, so there's nothing new to be done
		// except ensure that it exists.
		archivedPath := filepath.Join(b.mountPoint, path)
		if err := os.MkdirAll(archivedPath, 0755); err != nil {
			return errors.Wrapf(err, "error ensuring volume path %q exists", archivedPath)
		}
		if err := b.volumeCacheInvalidate(path); err != nil {
			return errors.Wrapf(err, "error ensuring volume path %q is preserved", archivedPath)
		}
		return nil
	}
	// Figure out where the cache for this volume would be stored.
	b.preserved++
	cacheDir, err := b.store.ContainerDirectory(b.builder.ContainerID)
	if err != nil {
		return errors.Errorf("unable to locate temporary directory for container")
	}
	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("volume%d.tar", b.preserved))
	// Save info about the top level of the location that we'll be archiving.
	archivedPath := filepath.Join(b.mountPoint, path)

	// Try and resolve the symlink (if one exists)
	// Set archivedPath and path based on whether a symlink is found or not
	if symLink, err := resolveSymLink(b.mountPoint, path); err == nil {
		archivedPath = filepath.Join(b.mountPoint, symLink)
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
	b.volumeCacheInfo[path] = st
	if !b.volumes.Add(path) {
		// This path is not a subdirectory of a volume path that we're
		// already preserving, so adding it to the list should work.
		return errors.Errorf("error adding %q to the volume cache", path)
	}
	b.volumeCache[path] = cacheFile
	// Now prune cache files for volumes that are now supplanted by this one.
	removed := []string{}
	for cachedPath := range b.volumeCache {
		// Walk our list of cached volumes, and check that they're
		// still in the list of locations that we need to cache.
		found := false
		for _, volume := range b.volumes {
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
		archivedPath := filepath.Join(b.mountPoint, cachedPath)
		logrus.Debugf("no longer need cache of %q in %q", archivedPath, b.volumeCache[cachedPath])
		if err := os.Remove(b.volumeCache[cachedPath]); err != nil {
			return errors.Wrapf(err, "error removing %q", b.volumeCache[cachedPath])
		}
		delete(b.volumeCache, cachedPath)
	}
	return nil
}

// Remove any volume cache item which will need to be re-saved because we're
// writing to part of it.
func (b *Executor) volumeCacheInvalidate(path string) error {
	invalidated := []string{}
	for cachedPath := range b.volumeCache {
		if strings.HasPrefix(path, cachedPath+string(os.PathSeparator)) {
			invalidated = append(invalidated, cachedPath)
		}
	}
	for _, cachedPath := range invalidated {
		if err := os.Remove(b.volumeCache[cachedPath]); err != nil {
			return errors.Wrapf(err, "error removing volume cache %q", b.volumeCache[cachedPath])
		}
		archivedPath := filepath.Join(b.mountPoint, cachedPath)
		logrus.Debugf("invalidated volume cache for %q from %q", archivedPath, b.volumeCache[cachedPath])
		delete(b.volumeCache, cachedPath)
	}
	return nil
}

// Save the contents of each of the executor's list of volumes for which we
// don't already have a cache file.
func (b *Executor) volumeCacheSave() error {
	for cachedPath, cacheFile := range b.volumeCache {
		archivedPath := filepath.Join(b.mountPoint, cachedPath)
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
func (b *Executor) volumeCacheRestore() error {
	for cachedPath, cacheFile := range b.volumeCache {
		archivedPath := filepath.Join(b.mountPoint, cachedPath)
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
		if st, ok := b.volumeCacheInfo[cachedPath]; ok {
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
// imagebuilder tells us the instruction was "ADD" and not "COPY".
func (b *Executor) Copy(excludes []string, copies ...imagebuilder.Copy) error {
	for _, copy := range copies {
		logrus.Debugf("COPY %#v, %#v", excludes, copy)
		if err := b.volumeCacheInvalidate(copy.Dest); err != nil {
			return err
		}
		sources := []string{}
		for _, src := range copy.Src {
			if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
				sources = append(sources, src)
			} else if len(copy.From) > 0 {
				if other, ok := b.named[copy.From]; ok && other.index < b.index {
					sources = append(sources, filepath.Join(other.mountPoint, src))
				} else {
					return errors.Errorf("the stage %q has not been built", copy.From)
				}
			} else {
				sources = append(sources, filepath.Join(b.contextDir, src))
			}
		}

		options := buildah.AddAndCopyOptions{
			Chown: copy.Chown,
		}

		if err := b.builder.Add(copy.Dest, copy.Download, options, sources...); err != nil {
			return err
		}
	}
	return nil
}

func convertMounts(mounts []Mount) []specs.Mount {
	specmounts := []specs.Mount{}
	for _, m := range mounts {
		s := specs.Mount{
			Destination: m.Destination,
			Type:        m.Type,
			Source:      m.Source,
			Options:     m.Options,
		}
		specmounts = append(specmounts, s)
	}
	return specmounts
}

// Run executes a RUN instruction using the working container as a root
// directory.
func (b *Executor) Run(run imagebuilder.Run, config docker.Config) error {
	logrus.Debugf("RUN %#v, %#v", run, config)
	if b.builder == nil {
		return errors.Errorf("no build container available")
	}
	stdin := b.in
	if stdin == nil {
		devNull, err := os.Open(os.DevNull)
		if err != nil {
			return errors.Errorf("error opening %q for reading: %v", os.DevNull, err)
		}
		defer devNull.Close()
		stdin = devNull
	}
	options := buildah.RunOptions{
		Hostname:   config.Hostname,
		Runtime:    b.runtime,
		Args:       b.runtimeArgs,
		Mounts:     convertMounts(b.transientMounts),
		Env:        config.Env,
		User:       config.User,
		WorkingDir: config.WorkingDir,
		Entrypoint: config.Entrypoint,
		Cmd:        config.Cmd,
		Stdin:      stdin,
		Stdout:     b.out,
		Stderr:     b.err,
		Quiet:      b.quiet,
	}
	if config.NetworkDisabled {
		options.ConfigureNetwork = buildah.NetworkDisabled
	} else {
		options.ConfigureNetwork = buildah.NetworkEnabled
	}

	args := run.Args
	if run.Shell {
		args = append([]string{"/bin/sh", "-c"}, args...)
	}
	if err := b.volumeCacheSave(); err != nil {
		return err
	}
	err := b.builder.Run(args, options)
	if err2 := b.volumeCacheRestore(); err2 != nil {
		if err == nil {
			return err2
		}
	}
	return err
}

// UnrecognizedInstruction is called when we encounter an instruction that the
// imagebuilder parser didn't understand.
func (b *Executor) UnrecognizedInstruction(step *imagebuilder.Step) error {
	errStr := fmt.Sprintf("Build error: Unknown instruction: %q ", step.Command)
	err := fmt.Sprintf(errStr+"%#v", step)
	if b.ignoreUnrecognizedInstructions {
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

// NewExecutor creates a new instance of the imagebuilder.Executor interface.
func NewExecutor(store storage.Store, options BuildOptions) (*Executor, error) {
	exec := Executor{
		store:                          store,
		contextDir:                     options.ContextDirectory,
		pullPolicy:                     options.PullPolicy,
		registry:                       options.Registry,
		transport:                      options.Transport,
		ignoreUnrecognizedInstructions: options.IgnoreUnrecognizedInstructions,
		quiet:                   options.Quiet,
		runtime:                 options.Runtime,
		runtimeArgs:             options.RuntimeArgs,
		transientMounts:         options.TransientMounts,
		compression:             options.Compression,
		output:                  options.Output,
		outputFormat:            options.OutputFormat,
		additionalTags:          options.AdditionalTags,
		signaturePolicyPath:     options.SignaturePolicyPath,
		systemContext:           options.SystemContext,
		volumeCache:             make(map[string]string),
		volumeCacheInfo:         make(map[string]os.FileInfo),
		log:                     options.Log,
		in:                      options.In,
		out:                     options.Out,
		err:                     options.Err,
		reportWriter:            options.ReportWriter,
		isolation:               options.Isolation,
		namespaceOptions:        options.NamespaceOptions,
		configureNetwork:        options.ConfigureNetwork,
		cniPluginPath:           options.CNIPluginPath,
		cniConfigDir:            options.CNIConfigDir,
		idmappingOptions:        options.IDMappingOptions,
		commonBuildOptions:      options.CommonBuildOpts,
		defaultMountsFilePath:   options.DefaultMountsFilePath,
		iidfile:                 options.IIDFile,
		squash:                  options.Squash,
		labels:                  append([]string{}, options.Labels...),
		annotations:             append([]string{}, options.Annotations...),
		layers:                  options.Layers,
		noCache:                 options.NoCache,
		removeIntermediateCtrs:  options.RemoveIntermediateCtrs,
		forceRmIntermediateCtrs: options.ForceRmIntermediateCtrs,
	}
	if exec.err == nil {
		exec.err = os.Stderr
	}
	if exec.out == nil {
		exec.out = os.Stdout
	}
	if exec.log == nil {
		stepCounter := 0
		exec.log = func(format string, args ...interface{}) {
			stepCounter++
			prefix := fmt.Sprintf("STEP %d: ", stepCounter)
			suffix := "\n"
			fmt.Fprintf(exec.err, prefix+format+suffix, args...)
		}
	}
	return &exec, nil
}

// Prepare creates a working container based on specified image, or if one
// isn't specified, the first FROM instruction we can find in the parsed tree.
func (b *Executor) Prepare(ctx context.Context, ib *imagebuilder.Builder, node *parser.Node, from string) error {
	if from == "" {
		base, err := ib.From(node)
		if err != nil {
			logrus.Debugf("Prepare(node.Children=%#v)", node.Children)
			return errors.Wrapf(err, "error determining starting point for build")
		}
		from = base
	}
	logrus.Debugf("FROM %#v", from)
	if !b.quiet {
		b.log("FROM %s", from)
	}
	builderOptions := buildah.BuilderOptions{
		Args:                  ib.Args,
		FromImage:             from,
		PullPolicy:            b.pullPolicy,
		Registry:              b.registry,
		Transport:             b.transport,
		SignaturePolicyPath:   b.signaturePolicyPath,
		ReportWriter:          b.reportWriter,
		SystemContext:         b.systemContext,
		Isolation:             b.isolation,
		NamespaceOptions:      b.namespaceOptions,
		ConfigureNetwork:      b.configureNetwork,
		CNIPluginPath:         b.cniPluginPath,
		CNIConfigDir:          b.cniConfigDir,
		IDMappingOptions:      b.idmappingOptions,
		CommonBuildOpts:       b.commonBuildOptions,
		DefaultMountsFilePath: b.defaultMountsFilePath,
	}
	builder, err := buildah.NewBuilder(ctx, b.store, builderOptions)
	if err != nil {
		return errors.Wrapf(err, "error creating build container")
	}
	volumes := map[string]struct{}{}
	for _, v := range builder.Volumes() {
		volumes[v] = struct{}{}
	}
	dConfig := docker.Config{
		Hostname:   builder.Hostname(),
		Domainname: builder.Domainname(),
		User:       builder.User(),
		Env:        builder.Env(),
		Cmd:        builder.Cmd(),
		Image:      from,
		Volumes:    volumes,
		WorkingDir: builder.WorkDir(),
		Entrypoint: builder.Entrypoint(),
		Labels:     builder.Labels(),
		Shell:      builder.Shell(),
		StopSignal: builder.StopSignal(),
		OnBuild:    builder.OnBuild(),
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
		return errors.Wrapf(err, "error updating build context")
	}
	mountPoint, err := builder.Mount(builder.MountLabel)
	if err != nil {
		if err2 := builder.Delete(); err2 != nil {
			logrus.Debugf("error deleting container which we failed to mount: %v", err2)
		}
		return errors.Wrapf(err, "error mounting new container")
	}
	b.mountPoint = mountPoint
	b.builder = builder
	// Add the top layer of this image to b.topLayers so we can keep track of them
	// when building with cached images.
	b.topLayers = append(b.topLayers, builder.TopLayer)
	logrus.Debugln("Container ID:", builder.ContainerID)
	return nil
}

// Delete deletes the working container, if we have one.  The Executor object
// should not be used to build another image, as the name of the output image
// isn't resettable.
func (b *Executor) Delete() (err error) {
	if b.builder != nil {
		err = b.builder.Delete()
		b.builder = nil
	}
	return err
}

// resolveNameToImageRef creates a types.ImageReference from b.output
func (b *Executor) resolveNameToImageRef() (types.ImageReference, error) {
	var (
		imageRef types.ImageReference
		err      error
	)
	if b.output != "" {
		imageRef, err = alltransports.ParseImageName(b.output)
		if err != nil {
			candidates := util.ResolveName(b.output, "", b.systemContext, b.store)
			if len(candidates) == 0 {
				return nil, errors.Errorf("error parsing target image name %q", b.output)
			}
			imageRef2, err2 := is.Transport.ParseStoreReference(b.store, candidates[0])
			if err2 != nil {
				return nil, errors.Wrapf(err, "error parsing target image name %q", b.output)
			}
			return imageRef2, nil
		}
		return imageRef, nil
	}
	imageRef, err = is.Transport.ParseStoreReference(b.store, "@"+stringid.GenerateRandomID())
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing reference for image to be written")
	}
	return imageRef, nil
}

// Execute runs each of the steps in the parsed tree, in turn.
func (b *Executor) Execute(ctx context.Context, ib *imagebuilder.Builder, node *parser.Node) error {
	checkForLayers := true
	children := node.Children
	commitName := b.output
	for i, node := range node.Children {
		step := ib.Step()
		if err := step.Resolve(node); err != nil {
			return errors.Wrapf(err, "error resolving step %+v", *node)
		}
		logrus.Debugf("Parsed Step: %+v", *step)
		if !b.quiet {
			b.log("%s", step.Original)
		}
		requiresStart := false
		if i < len(node.Children)-1 {
			requiresStart = ib.RequiresStart(&parser.Node{Children: node.Children[i+1:]})
		}

		if !b.layers && !b.noCache {
			err := ib.Run(step, b, requiresStart)
			if err != nil {
				return errors.Wrapf(err, "error building at step %+v", *step)
			}
			continue
		}

		if i < len(children)-1 {
			b.output = ""
		} else {
			b.output = commitName
		}

		var (
			cacheID string
			err     error
			imgID   string
		)
		// checkForLayers will be true if b.layers is true and a cached intermediate image is found.
		// checkForLayers is set to false when either there is no cached image or a break occurs where
		// the instructions in the Dockerfile change from a previous build.
		// Don't check for cache if b.noCache is set to true.
		if checkForLayers && !b.noCache {
			cacheID, err = b.layerExists(ctx, node, children[:i])
			if err != nil {
				return errors.Wrap(err, "error checking if cached image exists from a previous build")
			}
		}

		if cacheID != "" {
			fmt.Fprintf(b.out, "--> Using cache %s\n", cacheID)
		}

		// If a cache is found for the last step, that means nothing in the
		// Dockerfile changed. Just create a copy of the existing image and
		// save it with the new name passed in by the user.
		if cacheID != "" && i == len(children)-1 {
			if err := b.copyExistingImage(ctx, cacheID); err != nil {
				return err
			}
			break
		}

		if cacheID == "" || !checkForLayers {
			checkForLayers = false
			err := ib.Run(step, b, requiresStart)
			if err != nil {
				return errors.Wrapf(err, "error building at step %+v", *step)
			}
		}

		// Commit if no cache is found
		if cacheID == "" {
			imgID, err = b.Commit(ctx, ib, getCreatedBy(node))
			if err != nil {
				return errors.Wrapf(err, "error committing container for step %+v", *step)
			}
			if i == len(children)-1 {
				b.log("COMMIT %s", b.output)
			}
		} else {
			// Cache is found, assign imgID the id of the cached image so
			// it is used to create the container for the next step.
			imgID = cacheID
		}
		// Add container ID of successful intermediate container to b.containerIDs
		b.containerIDs = append(b.containerIDs, b.builder.ContainerID)
		// Prepare for the next step with imgID as the new base image.
		if i != len(children)-1 {
			if err := b.Prepare(ctx, ib, node, imgID); err != nil {
				return errors.Wrap(err, "error preparing container for next step")
			}
		}
	}
	return nil
}

// copyExistingImage creates a copy of an image already in store
func (b *Executor) copyExistingImage(ctx context.Context, cacheID string) error {
	// Get the destination Image Reference
	dest, err := b.resolveNameToImageRef()
	if err != nil {
		return err
	}

	policyContext, err := util.GetPolicyContext(b.systemContext)
	if err != nil {
		return err
	}
	defer policyContext.Destroy()

	// Look up the source image, expecting it to be in local storage
	src, err := is.Transport.ParseStoreReference(b.store, cacheID)
	if err != nil {
		return errors.Wrapf(err, "error getting source imageReference for %q", cacheID)
	}
	if err := cp.Image(ctx, policyContext, dest, src, nil); err != nil {
		return errors.Wrapf(err, "error copying image %q", cacheID)
	}
	b.log("COMMIT %s", b.output)
	return nil
}

// layerExists returns true if an intermediate image of currNode exists in the image store from a previous build.
// It verifies tihis by checking the parent of the top layer of the image and the history.
func (b *Executor) layerExists(ctx context.Context, currNode *parser.Node, children []*parser.Node) (string, error) {
	// Get the list of images available in the image store
	images, err := b.store.Images()
	if err != nil {
		return "", errors.Wrap(err, "error getting image list from store")
	}
	for _, image := range images {
		layer, err := b.store.Layer(image.TopLayer)
		if err != nil {
			return "", errors.Wrapf(err, "error getting top layer info")
		}
		// If the parent of the top layer of an image is equal to the last entry in b.topLayers
		// it means that this image is potentially a cached intermediate image from a previous
		// build. Next we double check that the history of this image is equivalent to the previous
		// lines in the Dockerfile up till the point we are at in the build.
		if layer.Parent == b.topLayers[len(b.topLayers)-1] {
			history, err := b.getImageHistory(ctx, image.ID)
			if err != nil {
				return "", errors.Wrapf(err, "error getting history of %q", image.ID)
			}
			// children + currNode is the point of the Dockerfile we are currently at.
			if historyMatches(append(children, currNode), history) {
				// This checks if the files copied during build have been changed if the node is
				// a COPY or ADD command.
				filesMatch, err := b.copiedFilesMatch(currNode, history[len(history)-1].Created)
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

// getImageHistory returns the history of imageID.
func (b *Executor) getImageHistory(ctx context.Context, imageID string) ([]v1.History, error) {
	imageRef, err := is.Transport.ParseStoreReference(b.store, "@"+imageID)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting image reference %q", imageID)
	}
	ref, err := imageRef.NewImage(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error creating new image from reference")
	}
	oci, err := ref.OCIConfig(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting oci config of image %q", imageID)
	}
	return oci.History, nil
}

// getCreatedBy returns the command the image at node will be created by.
func getCreatedBy(node *parser.Node) string {
	if node.Value == "run" {
		return "/bin/sh -c " + node.Original[4:]
	}
	return "/bin/sh -c #(nop) " + node.Original
}

// historyMatches returns true if the history of the image matches the lines
// in the Dockerfile till the point of build we are at.
// Used to verify whether a cache of the intermediate image exists and whether
// to run the build again.
func historyMatches(children []*parser.Node, history []v1.History) bool {
	i := len(history) - 1
	for j := len(children) - 1; j >= 0; j-- {
		instruction := children[j].Original
		if children[j].Value == "run" {
			instruction = instruction[4:]
		}
		if !strings.Contains(history[i].CreatedBy, instruction) {
			return false
		}
		i--
	}
	return true
}

// getFilesToCopy goes through node to get all the src files that are copied, added or downloaded.
// It is possible for the Dockerfile to have src as hom*, which means all files that have hom as a prefix.
// Another format is hom?.txt, which means all files that have that name format with the ? replaced by another character.
func (b *Executor) getFilesToCopy(node *parser.Node) ([]string, error) {
	currNode := node.Next
	var src []string
	for currNode.Next != nil {
		if currNode.Next == nil {
			break
		}
		if strings.HasPrefix(currNode.Value, "http://") || strings.HasPrefix(currNode.Value, "https://") {
			src = append(src, currNode.Value)
			continue
		}
		matches, err := filepath.Glob(filepath.Join(b.contextDir, currNode.Value))
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
func (b *Executor) copiedFilesMatch(node *parser.Node, historyTime *time.Time) (bool, error) {
	if node.Value != "add" && node.Value != "copy" {
		return true, nil
	}

	src, err := b.getFilesToCopy(node)
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
		// For local files, walk the file tree and check the time stamps.
		timeIsGreater := false
		err := filepath.Walk(item, func(path string, info os.FileInfo, err error) error {
			if info.ModTime().After(*historyTime) {
				timeIsGreater = true
				return nil
			}
			return nil
		})
		if err != nil {
			return false, errors.Wrapf(err, "error walking file tree %q", item)
		}
		if timeIsGreater {
			return false, nil
		}
	}
	return true, nil
}

// urlContentModified sends a get request to the url and checks if the header has a value in
// Last-Modified, and if it does compares the time stamp to that of the history of the cached image.
// returns true if there is no Last-Modified value in the header.
func urlContentModified(url string, historyTime *time.Time) (bool, error) {
	resp, err := http.Get(url)
	if err != nil {
		return false, errors.Wrapf(err, "error getting %q", url)
	}
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

// Commit writes the container's contents to an image, using a passed-in tag as
// the name if there is one, generating a unique ID-based one otherwise.
func (b *Executor) Commit(ctx context.Context, ib *imagebuilder.Builder, createdBy string) (string, error) {
	imageRef, err := b.resolveNameToImageRef()
	if err != nil {
		return "", err
	}

	if ib.Author != "" {
		b.builder.SetMaintainer(ib.Author)
	}
	config := ib.Config()
	b.builder.SetCreatedBy(createdBy)
	b.builder.SetHostname(config.Hostname)
	b.builder.SetDomainname(config.Domainname)
	b.builder.SetUser(config.User)
	b.builder.ClearPorts()
	for p := range config.ExposedPorts {
		b.builder.SetPort(string(p))
	}
	for _, envSpec := range config.Env {
		spec := strings.SplitN(envSpec, "=", 2)
		b.builder.SetEnv(spec[0], spec[1])
	}
	b.builder.SetCmd(config.Cmd)
	b.builder.ClearVolumes()
	for v := range config.Volumes {
		b.builder.AddVolume(v)
	}
	b.builder.ClearOnBuild()
	for _, onBuildSpec := range config.OnBuild {
		b.builder.SetOnBuild(onBuildSpec)
	}
	b.builder.SetWorkDir(config.WorkingDir)
	b.builder.SetEntrypoint(config.Entrypoint)
	b.builder.SetShell(config.Shell)
	b.builder.SetStopSignal(config.StopSignal)
	b.builder.ClearLabels()
	for k, v := range config.Labels {
		b.builder.SetLabel(k, v)
	}
	for _, labelSpec := range b.labels {
		label := strings.SplitN(labelSpec, "=", 2)
		if len(label) > 1 {
			b.builder.SetLabel(label[0], label[1])
		} else {
			b.builder.SetLabel(label[0], "")
		}
	}
	for _, annotationSpec := range b.annotations {
		annotation := strings.SplitN(annotationSpec, "=", 2)
		if len(annotation) > 1 {
			b.builder.SetAnnotation(annotation[0], annotation[1])
		} else {
			b.builder.SetAnnotation(annotation[0], "")
		}
	}
	if imageRef != nil {
		logName := transports.ImageName(imageRef)
		logrus.Debugf("COMMIT %q", logName)
		if !b.quiet && !b.layers && !b.noCache {
			b.log("COMMIT %s", logName)
		}
	} else {
		logrus.Debugf("COMMIT")
		if !b.quiet && !b.layers && !b.noCache {
			b.log("COMMIT")
		}
	}
	writer := b.reportWriter
	if b.layers || b.noCache {
		writer = nil
	}
	options := buildah.CommitOptions{
		Compression:           b.compression,
		SignaturePolicyPath:   b.signaturePolicyPath,
		AdditionalTags:        b.additionalTags,
		ReportWriter:          writer,
		PreferredManifestType: b.outputFormat,
		IIDFile:               b.iidfile,
		Squash:                b.squash,
		Parent:                b.builder.FromImageID,
	}
	imgID, err := b.builder.Commit(ctx, imageRef, options)
	if err != nil {
		return "", err
	}
	if options.IIDFile == "" && imgID != "" {
		fmt.Fprintf(b.out, "--> %s\n", imgID)
	}
	return imgID, nil
}

// Build takes care of the details of running Prepare/Execute/Commit/Delete
// over each of the one or more parsed Dockerfiles and stages.
func (b *Executor) Build(ctx context.Context, stages imagebuilder.Stages) error {
	if len(stages) == 0 {
		errors.New("error building: no stages to build")
	}
	var (
		stageExecutor *Executor
		lastErr       error
	)
	for _, stage := range stages {
		stageExecutor = b.withName(stage.Name, stage.Position)
		if err := stageExecutor.Prepare(ctx, stage.Builder, stage.Node, ""); err != nil {
			lastErr = err
		}
		// Always remove the intermediate/build containers, even if the build was unsuccessful.
		// If building with layers, remove all intermediate/build containers if b.forceRmIntermediateCtrs
		// is true.
		if b.forceRmIntermediateCtrs || (!b.layers && !b.noCache) {
			defer stageExecutor.Delete()
		}
		if err := stageExecutor.Execute(ctx, stage.Builder, stage.Node); err != nil {
			lastErr = err
		}

		// Delete the successful intermediate containers if an error in the build
		// process occurs and b.removeIntermediateCtrs is true.
		if lastErr != nil {
			if b.removeIntermediateCtrs {
				stageExecutor.deleteSuccessfulIntermediateCtrs()
			}
			return lastErr
		}
		b.containerIDs = append(b.containerIDs, stageExecutor.containerIDs...)
	}

	if !b.layers && !b.noCache {
		_, err := stageExecutor.Commit(ctx, stages[len(stages)-1].Builder, "")
		if err != nil {
			return err
		}
	}
	// If building with layers and b.removeIntermediateCtrs is true
	// only remove intermediate container for each step if an error
	// during the build process doesn't occur.
	// If the build is unsuccessful, the container created at the step
	// the failure happened will persist in the container store.
	// This if condition will be false if not building with layers and
	// the removal of intermediate/build containers will be handled by the
	// defer statement above.
	if b.removeIntermediateCtrs && (b.layers || b.noCache) {
		if err := b.deleteSuccessfulIntermediateCtrs(); err != nil {
			return errors.Errorf("Failed to cleanup intermediate containers")
		}
	}
	return nil
}

// BuildDockerfiles parses a set of one or more Dockerfiles (which may be
// URLs), creates a new Executor, and then runs Prepare/Execute/Commit/Delete
// over the entire set of instructions.
func BuildDockerfiles(ctx context.Context, store storage.Store, options BuildOptions, paths ...string) error {
	if len(paths) == 0 {
		return errors.Errorf("error building: no dockerfiles specified")
	}
	var dockerfiles []io.ReadCloser
	defer func(dockerfiles ...io.ReadCloser) {
		for _, d := range dockerfiles {
			d.Close()
		}
	}(dockerfiles...)
	for _, dfile := range paths {
		var data io.ReadCloser

		if strings.HasPrefix(dfile, "http://") || strings.HasPrefix(dfile, "https://") {
			logrus.Debugf("reading remote Dockerfile %q", dfile)
			resp, err := http.Get(dfile)
			if err != nil {
				return errors.Wrapf(err, "error getting %q", dfile)
			}
			if resp.ContentLength == 0 {
				resp.Body.Close()
				return errors.Errorf("no contents in %q", dfile)
			}
			data = resp.Body
		} else {
			// If the Dockerfile isn't found try prepending the
			// context directory to it.
			if _, err := os.Stat(dfile); os.IsNotExist(err) {
				dfile = filepath.Join(options.ContextDirectory, dfile)
			}
			logrus.Debugf("reading local Dockerfile %q", dfile)
			contents, err := os.Open(dfile)
			if err != nil {
				return errors.Wrapf(err, "error reading %q", dfile)
			}
			dinfo, err := contents.Stat()
			if err != nil {
				contents.Close()
				return errors.Wrapf(err, "error reading info about %q", dfile)
			}
			if dinfo.Mode().IsRegular() && dinfo.Size() == 0 {
				contents.Close()
				return errors.Wrapf(err, "no contents in %q", dfile)
			}
			data = contents
		}

		// pre-process Dockerfiles with ".in" suffix
		if strings.HasSuffix(dfile, ".in") {
			pData, err := preprocessDockerfileContents(data, options.ContextDirectory)
			if err != nil {
				return err
			}
			data = *pData
		}

		dockerfiles = append(dockerfiles, data)
	}
	mainNode, err := imagebuilder.ParseDockerfile(dockerfiles[0])
	if err != nil {
		return errors.Wrapf(err, "error parsing main Dockerfile")
	}
	for _, d := range dockerfiles[1:] {
		additionalNode, err := imagebuilder.ParseDockerfile(d)
		if err != nil {
			return errors.Wrapf(err, "error parsing additional Dockerfile")
		}
		mainNode.Children = append(mainNode.Children, additionalNode.Children...)
	}
	exec, err := NewExecutor(store, options)
	if err != nil {
		return errors.Wrapf(err, "error creating build executor")
	}
	b := imagebuilder.NewBuilder(options.Args)
	stages := imagebuilder.NewStages(mainNode, b)
	return exec.Build(ctx, stages)
}

// deleteSuccessfulIntermediateCtrs goes through the container IDs in b.containerIDs
// and deletes the containers associated with that ID.
func (b *Executor) deleteSuccessfulIntermediateCtrs() error {
	var lastErr error
	for _, ctr := range b.containerIDs {
		if err := b.store.DeleteContainer(ctr); err != nil {
			logrus.Errorf("error deleting build container %q: %v\n", ctr, err)
			lastErr = err
		}
	}
	return lastErr
}

// preprocessDockerfileContents runs CPP(1) in preprocess-only mode on the input
// dockerfile content and will use ctxDir as the base include path.
//
// Note: we cannot use cmd.StdoutPipe() as cmd.Wait() closes it.
func preprocessDockerfileContents(r io.ReadCloser, ctxDir string) (rdrCloser *io.ReadCloser, err error) {
	cppPath := "/usr/bin/cpp"
	if _, err = os.Stat(cppPath); err != nil {
		if os.IsNotExist(err) {
			err = errors.Errorf("error: Dockerfile.in support requires %s to be installed", cppPath)
		}
		return nil, err
	}

	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}

	cmd := exec.Command(cppPath, "-E", "-iquote", ctxDir, "-")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			pipe.Close()
		}
	}()

	if err = cmd.Start(); err != nil {
		return nil, err
	}

	if _, err = io.Copy(pipe, r); err != nil {
		return nil, err
	}

	pipe.Close()
	if err = cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			err = fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
		}
		return nil, errors.Wrapf(err, "error pre-processing Dockerfile")
	}

	rc := ioutil.NopCloser(bytes.NewReader(stdout.Bytes()))
	return &rc, nil
}
