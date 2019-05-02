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
	"sort"
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
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	docker "github.com/fsouza/go-dockerclient"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerfile/parser"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	PullIfMissing = buildah.PullIfMissing
	PullAlways    = buildah.PullAlways
	PullNever     = buildah.PullNever

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
	// Accepted values are buildah.OCIv1ImageManifest and buildah.Dockerv2ImageManifest.
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
	// BlobDirectory is a directory which we'll use for caching layer blobs.
	BlobDirectory string
	// Target the targeted FROM in the Dockerfile to build
	Target string
}

// Executor is a buildah-based implementation of the imagebuilder.Executor
// interface.  It coordinates the entire build by using one StageExecutors to
// handle each stage of the build.
type Executor struct {
	stages                         map[string]*StageExecutor
	store                          storage.Store
	contextDir                     string
	pullPolicy                     buildah.PullPolicy
	registry                       string
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
	useCache                       bool
	removeIntermediateCtrs         bool
	forceRmIntermediateCtrs        bool
	imageMap                       map[string]string           // Used to map images that we create to handle the AS construct.
	containerMap                   map[string]*buildah.Builder // Used to map from image names to only-created-for-the-rootfs containers.
	baseMap                        map[string]bool             // Holds the names of every base image, as given.
	rootfsMap                      map[string]bool             // Holds the names of every stage whose rootfs is referenced in a COPY or ADD instruction.
	blobDirectory                  string
	excludes                       []string
	unusedArgs                     map[string]struct{}
	buildArgs                      map[string]string
}

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

// builtinAllowedBuildArgs is list of built-in allowed build args.  Normally we
// complain if we're given values for arguments which have no corresponding ARG
// instruction in the Dockerfile, since that's usually an indication of a user
// error, but for these values we make exceptions and ignore them.
var builtinAllowedBuildArgs = map[string]bool{
	"HTTP_PROXY":  true,
	"http_proxy":  true,
	"HTTPS_PROXY": true,
	"https_proxy": true,
	"FTP_PROXY":   true,
	"ftp_proxy":   true,
	"NO_PROXY":    true,
	"no_proxy":    true,
}

// startStage creates a new stage executor that will be referenced whenever a
// COPY or ADD statement uses a --from=NAME flag.
func (b *Executor) startStage(name string, index, stages int, from, output string) *StageExecutor {
	if b.stages == nil {
		b.stages = make(map[string]*StageExecutor)
	}
	stage := &StageExecutor{
		executor:        b,
		index:           index,
		stages:          stages,
		name:            name,
		volumeCache:     make(map[string]string),
		volumeCacheInfo: make(map[string]os.FileInfo),
		output:          output,
	}
	b.stages[name] = stage
	b.stages[from] = stage
	if idx := strconv.Itoa(index); idx != name {
		b.stages[idx] = stage
	}
	return stage
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
	archivedPath := filepath.Join(s.mountPoint, path)

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
// imagebuilder tells us the instruction was "ADD" and not "COPY".
func (s *StageExecutor) Copy(excludes []string, copies ...imagebuilder.Copy) error {
	for _, copy := range copies {
		// If the file exists, check to see if it's a symlink.
		// If it is a symlink, convert to it's target otherwise
		// the symlink will be overwritten.
		fileDest, _ := os.Lstat(filepath.Join(s.mountPoint, copy.Dest))
		if fileDest != nil {
			if fileDest.Mode()&os.ModeSymlink != 0 {
				if symLink, err := resolveSymlink(s.mountPoint, copy.Dest); err == nil {
					copy.Dest = symLink
				} else {
					return errors.Wrapf(err, "error reading symbolic link to %q", copy.Dest)
				}
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
			if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
				sources = append(sources, src)
			} else if len(copy.From) > 0 {
				if other, ok := s.executor.stages[copy.From]; ok && other.index < s.index {
					sources = append(sources, filepath.Join(other.mountPoint, src))
				} else if builder, ok := s.executor.containerMap[copy.From]; ok {
					sources = append(sources, filepath.Join(builder.MountPoint, src))
				} else {
					return errors.Errorf("the stage %q has not been built", copy.From)
				}
			} else {
				sources = append(sources, filepath.Join(s.executor.contextDir, src))
			}
		}

		options := buildah.AddAndCopyOptions{
			Chown:      copy.Chown,
			ContextDir: s.executor.contextDir,
			Excludes:   s.executor.excludes,
		}

		if err := s.builder.Add(copy.Dest, copy.Download, options, sources...); err != nil {
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
		args = append([]string{"/bin/sh", "-c"}, args...)
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

// NewExecutor creates a new instance of the imagebuilder.Executor interface.
func NewExecutor(store storage.Store, options BuildOptions, mainNode *parser.Node) (*Executor, error) {
	excludes, err := imagebuilder.ParseDockerignore(options.ContextDirectory)
	if err != nil {
		return nil, err
	}

	exec := Executor{
		store:                          store,
		contextDir:                     options.ContextDirectory,
		excludes:                       excludes,
		pullPolicy:                     options.PullPolicy,
		registry:                       options.Registry,
		ignoreUnrecognizedInstructions: options.IgnoreUnrecognizedInstructions,
		quiet:                          options.Quiet,
		runtime:                        options.Runtime,
		runtimeArgs:                    options.RuntimeArgs,
		transientMounts:                options.TransientMounts,
		compression:                    options.Compression,
		output:                         options.Output,
		outputFormat:                   options.OutputFormat,
		additionalTags:                 options.AdditionalTags,
		signaturePolicyPath:            options.SignaturePolicyPath,
		systemContext:                  options.SystemContext,
		log:                            options.Log,
		in:                             options.In,
		out:                            options.Out,
		err:                            options.Err,
		reportWriter:                   options.ReportWriter,
		isolation:                      options.Isolation,
		namespaceOptions:               options.NamespaceOptions,
		configureNetwork:               options.ConfigureNetwork,
		cniPluginPath:                  options.CNIPluginPath,
		cniConfigDir:                   options.CNIConfigDir,
		idmappingOptions:               options.IDMappingOptions,
		commonBuildOptions:             options.CommonBuildOpts,
		defaultMountsFilePath:          options.DefaultMountsFilePath,
		iidfile:                        options.IIDFile,
		squash:                         options.Squash,
		labels:                         append([]string{}, options.Labels...),
		annotations:                    append([]string{}, options.Annotations...),
		layers:                         options.Layers,
		useCache:                       !options.NoCache,
		removeIntermediateCtrs:         options.RemoveIntermediateCtrs,
		forceRmIntermediateCtrs:        options.ForceRmIntermediateCtrs,
		imageMap:                       make(map[string]string),
		containerMap:                   make(map[string]*buildah.Builder),
		baseMap:                        make(map[string]bool),
		rootfsMap:                      make(map[string]bool),
		blobDirectory:                  options.BlobDirectory,
		unusedArgs:                     make(map[string]struct{}),
		buildArgs:                      options.Args,
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
	for arg := range options.Args {
		if _, isBuiltIn := builtinAllowedBuildArgs[arg]; !isBuiltIn {
			exec.unusedArgs[arg] = struct{}{}
		}
	}
	for _, line := range mainNode.Children {
		node := line
		for node != nil { // tokens on this line, though we only care about the first
			switch strings.ToUpper(node.Value) { // first token - instruction
			case "ARG":
				arg := node.Next
				if arg != nil {
					// We have to be careful here - it's either an argument
					// and value, or just an argument, since they can be
					// separated by either "=" or whitespace.
					list := strings.SplitN(arg.Value, "=", 2)
					if _, stillUnused := exec.unusedArgs[list[0]]; stillUnused {
						delete(exec.unusedArgs, list[0])
					}
				}
			}
			break
		}
	}
	return &exec, nil
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
		} else {
			asImageName = ""
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
		// Add the top layer of this image to b.topLayers so we can
		// keep track of them when building with cached images.
		s.executor.topLayers = append(s.executor.topLayers, builder.TopLayer)
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

// resolveNameToImageRef creates a types.ImageReference for the output name in local storage
func (b *Executor) resolveNameToImageRef(output string) (types.ImageReference, error) {
	imageRef, err := alltransports.ParseImageName(output)
	if err != nil {
		candidates, _, _, err := util.ResolveName(output, "", b.systemContext, b.store)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing target image name %q", output)
		}
		if len(candidates) == 0 {
			return nil, errors.Errorf("error parsing target image name %q", output)
		}
		imageRef2, err2 := is.Transport.ParseStoreReference(b.store, candidates[0])
		if err2 != nil {
			return nil, errors.Wrapf(err, "error parsing target image name %q", output)
		}
		return imageRef2, nil
	}
	return imageRef, nil
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
			if imgID, ref, err = s.commit(ctx, ib, s.executor.getCreatedBy(nil), false, s.output); err != nil {
				return "", nil, errors.Wrapf(err, "error committing base container")
			}
		} else {
			// We don't need to squash the base image, so just
			// reuse the base image.
			logCommit(s.output, -1)
			if imgID, ref, err = s.copyExistingImage(ctx, s.builder.FromImageID, s.output); err != nil {
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
			cacheID, err = s.layerExists(ctx, node, children[:i])
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
				if imgID, ref, err = s.copyExistingImage(ctx, cacheID, commitName); err != nil {
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

// copyExistingImage creates a copy of an image already in the store
func (s *StageExecutor) copyExistingImage(ctx context.Context, cacheID, output string) (string, reference.Canonical, error) {
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
	defer policyContext.Destroy()

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
func (s *StageExecutor) layerExists(ctx context.Context, currNode *parser.Node, children []*parser.Node) (string, error) {
	// Get the list of images available in the image store
	images, err := s.executor.store.Images()
	if err != nil {
		return "", errors.Wrap(err, "error getting image list from store")
	}
	for _, image := range images {
		layer, err := s.executor.store.Layer(image.TopLayer)
		if err != nil {
			return "", errors.Wrapf(err, "error getting top layer info")
		}
		// If the parent of the top layer of an image is equal to the last entry in b.topLayers
		// it means that this image is potentially a cached intermediate image from a previous
		// build. Next we double check that the history of this image is equivalent to the previous
		// lines in the Dockerfile up till the point we are at in the build.
		if layer.Parent == s.executor.topLayers[len(s.executor.topLayers)-1] || layer.ID == s.executor.topLayers[len(s.executor.topLayers)-1] {
			history, err := s.executor.getImageHistory(ctx, image.ID)
			if err != nil {
				return "", errors.Wrapf(err, "error getting history of %q", image.ID)
			}
			// children + currNode is the point of the Dockerfile we are currently at.
			if s.executor.historyMatches(append(children, currNode), history) {
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

// getImageHistory returns the history of imageID.
func (b *Executor) getImageHistory(ctx context.Context, imageID string) ([]v1.History, error) {
	imageRef, err := is.Transport.ParseStoreReference(b.store, "@"+imageID)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting image reference %q", imageID)
	}
	ref, err := imageRef.NewImage(ctx, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating new image from reference to image %q", imageID)
	}
	defer ref.Close()
	oci, err := ref.OCIConfig(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting possibly-converted OCI config of image %q", imageID)
	}
	return oci.History, nil
}

// getCreatedBy returns the command the image at node will be created by.
func (b *Executor) getCreatedBy(node *parser.Node) string {
	if node == nil {
		return "/bin/sh"
	}
	if node.Value == "run" {
		buildArgs := b.getBuildArgs()
		if buildArgs != "" {
			return "|" + strconv.Itoa(len(strings.Split(buildArgs, " "))) + " " + buildArgs + " /bin/sh -c " + node.Original[4:]
		}
		return "/bin/sh -c " + node.Original[4:]
	}
	return "/bin/sh -c #(nop) " + node.Original
}

// historyMatches returns true if the history of the image matches the lines
// in the Dockerfile till the point of build we are at.
// Used to verify whether a cache of the intermediate image exists and whether
// to run the build again.
func (b *Executor) historyMatches(children []*parser.Node, history []v1.History) bool {
	i := len(history) - 1
	for j := len(children) - 1; j >= 0; j-- {
		instruction := children[j].Original
		if children[j].Value == "run" {
			instruction = instruction[4:]
			buildArgs := b.getBuildArgs()
			// If a previous image was built with some build-args but the new build process doesn't have any build-args
			// specified, so compare the lengths of the old instruction with the current one
			// 11 is the length of "/bin/sh -c " that is used to run the run commands
			if buildArgs == "" && len(history[i].CreatedBy) > len(instruction)+11 {
				return false
			}
			// There are build-args, so check if anything with the build-args has changed
			if buildArgs != "" && !strings.Contains(history[i].CreatedBy, buildArgs) {
				return false
			}
		}
		if !strings.Contains(history[i].CreatedBy, instruction) {
			return false
		}
		i--
	}
	return true
}

// getBuildArgs returns a string of the build-args specified during the build process
// it excludes any build-args that were not used in the build process
func (b *Executor) getBuildArgs() string {
	var buildArgs []string
	for k, v := range b.buildArgs {
		if _, ok := b.unusedArgs[k]; !ok {
			buildArgs = append(buildArgs, k+"="+v)
		}
	}
	return strings.Join(buildArgs, " ")
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
		AdditionalTags:        s.executor.additionalTags,
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

// Build takes care of the details of running Prepare/Execute/Commit/Delete
// over each of the one or more parsed Dockerfiles and stages.
func (b *Executor) Build(ctx context.Context, stages imagebuilder.Stages) (imageID string, ref reference.Canonical, err error) {
	if len(stages) == 0 {
		return "", nil, errors.New("error building: no stages to build")
	}
	var cleanupImages []string
	cleanupStages := make(map[int]*StageExecutor)

	cleanup := func() error {
		var lastErr error
		// Clean up any containers associated with the final container
		// built by a stage, for stages that succeeded, since we no
		// longer need their filesystem contents.
		for _, stage := range cleanupStages {
			if err := stage.Delete(); err != nil {
				logrus.Debugf("Failed to cleanup stage containers: %v", err)
				lastErr = err
			}
		}
		cleanupStages = nil
		// Clean up any builders that we used to get data from images.
		for _, builder := range b.containerMap {
			if err := builder.Delete(); err != nil {
				logrus.Debugf("Failed to cleanup image containers: %v", err)
				lastErr = err
			}
		}
		b.containerMap = nil
		// Clean up any intermediate containers associated with stages,
		// since we're not keeping them for debugging.
		if b.removeIntermediateCtrs {
			if err := b.deleteSuccessfulIntermediateCtrs(); err != nil {
				logrus.Debugf("Failed to cleanup intermediate containers: %v", err)
				lastErr = err
			}
		}
		// Remove images from stages except the last one, since we're
		// not going to use them as a starting point for any new
		// stages.
		for i := range cleanupImages {
			removeID := cleanupImages[len(cleanupImages)-i-1]
			if removeID == imageID {
				continue
			}
			if _, err := b.store.DeleteImage(removeID, true); err != nil {
				logrus.Debugf("failed to remove intermediate image %q: %v", removeID, err)
				if b.forceRmIntermediateCtrs || errors.Cause(err) != storage.ErrImageUsedByContainer {
					lastErr = err
				}
			}
		}
		cleanupImages = nil
		return lastErr
	}
	defer cleanup()

	// Build maps of every named base image and every referenced stage root
	// filesystem.  Individual stages can use them to determine whether or
	// not they can skip certain steps near the end of their stages.
	for _, stage := range stages {
		node := stage.Node // first line
		for node != nil {  // each line
			for _, child := range node.Children { // tokens on this line, though we only care about the first
				switch strings.ToUpper(child.Value) { // first token - instruction
				case "FROM":
					if child.Next != nil { // second token on this line
						base := child.Next.Value
						if base != "scratch" {
							// TODO: this didn't undergo variable and arg
							// expansion, so if the AS clause in another
							// FROM instruction uses argument values,
							// we might not record the right value here.
							b.baseMap[base] = true
							logrus.Debugf("base: %q", base)
						}
					}
				case "ADD", "COPY":
					for _, flag := range child.Flags { // flags for this instruction
						if strings.HasPrefix(flag, "--from=") {
							// TODO: this didn't undergo variable and
							// arg expansion, so if the previous stage
							// was named using argument values, we might
							// not record the right value here.
							rootfs := flag[7:]
							b.rootfsMap[rootfs] = true
							logrus.Debugf("rootfs: %q", rootfs)
						}
					}
				}
				break
			}
			node = node.Next // next line
		}
	}

	// Run through the build stages, one at a time.
	for stageIndex, stage := range stages {
		var lastErr error

		ib := stage.Builder
		node := stage.Node
		base, err := ib.From(node)
		if err != nil {
			logrus.Debugf("Build(node.Children=%#v)", node.Children)
			return "", nil, err
		}

		// If this is the last stage, then the image that we produce at
		// its end should be given the desired output name.
		output := ""
		if stageIndex == len(stages)-1 {
			output = b.output
		}

		stageExecutor := b.startStage(stage.Name, stage.Position, len(stages), base, output)

		// If this a single-layer build, or if it's a multi-layered
		// build and b.forceRmIntermediateCtrs is set, make sure we
		// remove the intermediate/build containers, regardless of
		// whether or not the stage's build fails.
		if b.forceRmIntermediateCtrs || !b.layers {
			cleanupStages[stage.Position] = stageExecutor
		}

		// Build this stage.
		if imageID, ref, err = stageExecutor.Execute(ctx, stage, base); err != nil {
			lastErr = err
		}
		if lastErr != nil {
			return "", nil, lastErr
		}

		// The stage succeeded, so remove its build container if we're
		// told to delete successful intermediate/build containers for
		// multi-layered builds.
		if b.removeIntermediateCtrs {
			cleanupStages[stage.Position] = stageExecutor
		}

		// If this is an intermediate stage, make a note of the ID, so
		// that we can look it up later.
		if stageIndex < len(stages)-1 && imageID != "" {
			b.imageMap[stage.Name] = imageID
			// We're not populating the cache with intermediate
			// images, so add this one to the list of images that
			// we'll remove later.
			if !b.layers {
				cleanupImages = append(cleanupImages, imageID)
			}
			imageID = ""
		}
	}

	if len(b.unusedArgs) > 0 {
		unusedList := make([]string, 0, len(b.unusedArgs))
		for k := range b.unusedArgs {
			unusedList = append(unusedList, k)
		}
		sort.Strings(unusedList)
		fmt.Fprintf(b.out, "[Warning] one or more build args were not consumed: %v\n", unusedList)
	}

	if err := cleanup(); err != nil {
		return "", nil, err
	}

	if b.iidfile != "" {
		if err = ioutil.WriteFile(b.iidfile, []byte(imageID), 0644); err != nil {
			return imageID, ref, errors.Wrapf(err, "failed to write image ID to file %q", b.iidfile)
		}
	}

	return imageID, ref, nil
}

// BuildDockerfiles parses a set of one or more Dockerfiles (which may be
// URLs), creates a new Executor, and then runs Prepare/Execute/Commit/Delete
// over the entire set of instructions.
func BuildDockerfiles(ctx context.Context, store storage.Store, options BuildOptions, paths ...string) (string, reference.Canonical, error) {
	if len(paths) == 0 {
		return "", nil, errors.Errorf("error building: no dockerfiles specified")
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
				return "", nil, errors.Wrapf(err, "error getting %q", dfile)
			}
			if resp.ContentLength == 0 {
				resp.Body.Close()
				return "", nil, errors.Errorf("no contents in %q", dfile)
			}
			data = resp.Body
		} else {
			// If the Dockerfile isn't found try prepending the
			// context directory to it.
			dinfo, err := os.Stat(dfile)
			if os.IsNotExist(err) {
				dfile = filepath.Join(options.ContextDirectory, dfile)
			}
			dinfo, err = os.Stat(dfile)
			if err != nil {
				return "", nil, errors.Wrapf(err, "error reading info about %q", dfile)
			}
			// If given a directory, add '/Dockerfile' to it.
			if dinfo.Mode().IsDir() {
				dfile = filepath.Join(dfile, "Dockerfile")
			}
			logrus.Debugf("reading local Dockerfile %q", dfile)
			contents, err := os.Open(dfile)
			if err != nil {
				return "", nil, errors.Wrapf(err, "error reading %q", dfile)
			}
			dinfo, err = contents.Stat()
			if err != nil {
				contents.Close()
				return "", nil, errors.Wrapf(err, "error reading info about %q", dfile)
			}
			if dinfo.Mode().IsRegular() && dinfo.Size() == 0 {
				contents.Close()
				return "", nil, errors.Wrapf(err, "no contents in %q", dfile)
			}
			data = contents
		}

		// pre-process Dockerfiles with ".in" suffix
		if strings.HasSuffix(dfile, ".in") {
			pData, err := preprocessDockerfileContents(data, options.ContextDirectory)
			if err != nil {
				return "", nil, err
			}
			data = *pData
		}

		dockerfiles = append(dockerfiles, data)
	}

	mainNode, err := imagebuilder.ParseDockerfile(dockerfiles[0])
	if err != nil {
		return "", nil, errors.Wrapf(err, "error parsing main Dockerfile")
	}
	for _, d := range dockerfiles[1:] {
		additionalNode, err := imagebuilder.ParseDockerfile(d)
		if err != nil {
			return "", nil, errors.Wrapf(err, "error parsing additional Dockerfile")
		}
		mainNode.Children = append(mainNode.Children, additionalNode.Children...)
	}
	exec, err := NewExecutor(store, options, mainNode)
	if err != nil {
		return "", nil, errors.Wrapf(err, "error creating build executor")
	}
	b := imagebuilder.NewBuilder(options.Args)
	stages, err := imagebuilder.NewStages(mainNode, b)
	if err != nil {
		return "", nil, errors.Wrap(err, "error reading multiple stages")
	}
	if options.Target != "" {
		stagesTargeted, ok := stages.ThroughTarget(options.Target)
		if !ok {
			return "", nil, errors.Errorf("The target %q was not found in the provided Dockerfile", options.Target)
		}
		stages = stagesTargeted
	}
	return exec.Build(ctx, stages)
}

// deleteSuccessfulIntermediateCtrs goes through the container IDs in each
// stage's containerIDs list and deletes the containers associated with those
// IDs.
func (b *Executor) deleteSuccessfulIntermediateCtrs() error {
	var lastErr error
	for _, s := range b.stages {
		for _, ctr := range s.containerIDs {
			if err := b.store.DeleteContainer(ctr); err != nil {
				logrus.Errorf("error deleting build container %q: %v\n", ctr, err)
				lastErr = err
			}
		}
		// The stages map includes some stages under multiple keys, so
		// clearing their lists after we process a given stage is
		// necessary to avoid triggering errors that would occur if we
		// tried to delete a given stage's containers multiple times.
		s.containerIDs = nil
	}
	return lastErr
}

func (s *StageExecutor) EnsureContainerPath(path string) error {
	targetPath := filepath.Join(s.mountPoint, path)
	_, err := os.Lstat(targetPath)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(targetPath, 0755)
	}
	if err != nil {
		return errors.Wrapf(err, "error ensuring container path %q", path)
	}
	return nil
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
