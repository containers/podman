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
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/imagebuilder"
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
	Zstd         = archive.Zstd
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
				dinfo, err = os.Stat(dfile)
				if err != nil {
					return "", nil, errors.Wrapf(err, "error reading info about %q", dfile)
				}
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
