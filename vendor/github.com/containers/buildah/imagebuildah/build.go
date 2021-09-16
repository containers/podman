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
	"sync"

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/util"
	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	istorage "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/hashicorp/go-multierror"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerfile/parser"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

const (
	PullIfMissing = define.PullIfMissing
	PullAlways    = define.PullAlways
	PullIfNewer   = define.PullIfNewer
	PullNever     = define.PullNever

	Gzip         = archive.Gzip
	Bzip2        = archive.Bzip2
	Xz           = archive.Xz
	Zstd         = archive.Zstd
	Uncompressed = archive.Uncompressed
)

// Mount is a mountpoint for the build container.
type Mount = specs.Mount

type BuildOptions = define.BuildOptions

// BuildDockerfiles parses a set of one or more Dockerfiles (which may be
// URLs), creates one or more new Executors, and then runs
// Prepare/Execute/Commit/Delete over the entire set of instructions.
// If the Manifest option is set, returns the ID of the manifest list, else it
// returns the ID of the built image, and if a name was assigned to it, a
// canonical reference for that image.
func BuildDockerfiles(ctx context.Context, store storage.Store, options define.BuildOptions, paths ...string) (id string, ref reference.Canonical, err error) {
	if len(paths) == 0 {
		return "", nil, errors.Errorf("error building: no dockerfiles specified")
	}
	if len(options.Platforms) > 1 && options.IIDFile != "" {
		return "", nil, errors.Errorf("building multiple images, but iidfile %q can only be used to store one image ID", options.IIDFile)
	}

	logger := logrus.New()
	if options.Err != nil {
		logger.SetOutput(options.Err)
	} else {
		logger.SetOutput(os.Stderr)
	}
	logger.SetLevel(logrus.GetLevel())

	var dockerfiles []io.ReadCloser
	defer func(dockerfiles ...io.ReadCloser) {
		for _, d := range dockerfiles {
			d.Close()
		}
	}(dockerfiles...)

	for _, tag := range append([]string{options.Output}, options.AdditionalTags...) {
		if tag == "" {
			continue
		}
		if _, err := util.VerifyTagName(tag); err != nil {
			return "", nil, errors.Wrapf(err, "tag %s", tag)
		}
	}

	for _, dfile := range paths {
		var data io.ReadCloser

		if strings.HasPrefix(dfile, "http://") || strings.HasPrefix(dfile, "https://") {
			logger.Debugf("reading remote Dockerfile %q", dfile)
			resp, err := http.Get(dfile)
			if err != nil {
				return "", nil, err
			}
			if resp.ContentLength == 0 {
				resp.Body.Close()
				return "", nil, errors.Errorf("no contents in %q", dfile)
			}
			data = resp.Body
		} else {
			dinfo, err := os.Stat(dfile)
			if err != nil {
				// If the Dockerfile isn't available, try again with
				// context directory prepended (if not prepended yet).
				if !strings.HasPrefix(dfile, options.ContextDirectory) {
					dfile = filepath.Join(options.ContextDirectory, dfile)
					dinfo, err = os.Stat(dfile)
				}
			}
			if err != nil {
				return "", nil, err
			}

			var contents *os.File
			// If given a directory, add '/Dockerfile' to it.
			if dinfo.Mode().IsDir() {
				for _, file := range []string{"Containerfile", "Dockerfile"} {
					f := filepath.Join(dfile, file)
					logger.Debugf("reading local %q", f)
					contents, err = os.Open(f)
					if err == nil {
						break
					}
				}
			} else {
				contents, err = os.Open(dfile)
			}

			if err != nil {
				return "", nil, err
			}
			dinfo, err = contents.Stat()
			if err != nil {
				contents.Close()
				return "", nil, errors.Wrapf(err, "error reading info about %q", dfile)
			}
			if dinfo.Mode().IsRegular() && dinfo.Size() == 0 {
				contents.Close()
				return "", nil, errors.Errorf("no contents in %q", dfile)
			}
			data = contents
		}

		// pre-process Dockerfiles with ".in" suffix
		if strings.HasSuffix(dfile, ".in") {
			pData, err := preprocessContainerfileContents(logger, dfile, data, options.ContextDirectory)
			if err != nil {
				return "", nil, err
			}
			data = ioutil.NopCloser(pData)
		}

		dockerfiles = append(dockerfiles, data)
	}

	var files [][]byte
	for _, dockerfile := range dockerfiles {
		var b bytes.Buffer
		if _, err := b.ReadFrom(dockerfile); err != nil {
			return "", nil, err
		}
		files = append(files, b.Bytes())
	}

	if options.Jobs != nil && *options.Jobs != 0 {
		options.JobSemaphore = semaphore.NewWeighted(int64(*options.Jobs))
	}

	manifestList := options.Manifest
	options.Manifest = ""
	type instance struct {
		v1.Platform
		ID string
	}
	var instances []instance
	var instancesLock sync.Mutex

	var builds multierror.Group
	if options.SystemContext == nil {
		options.SystemContext = &types.SystemContext{}
	}

	if len(options.Platforms) == 0 {
		options.Platforms = append(options.Platforms, struct{ OS, Arch, Variant string }{
			OS:   options.SystemContext.OSChoice,
			Arch: options.SystemContext.ArchitectureChoice,
		})
	}

	systemContext := options.SystemContext
	for _, platform := range options.Platforms {
		platformContext := *systemContext
		platformContext.OSChoice = platform.OS
		platformContext.ArchitectureChoice = platform.Arch
		platformContext.VariantChoice = platform.Variant
		platformOptions := options
		platformOptions.SystemContext = &platformContext
		logPrefix := ""
		if len(options.Platforms) > 1 {
			logPrefix = "[" + platform.OS + "/" + platform.Arch
			if platform.Variant != "" {
				logPrefix += "/" + platform.Variant
			}
			logPrefix += "] "
		}
		builds.Go(func() error {
			thisID, thisRef, err := buildDockerfilesOnce(ctx, store, logger, logPrefix, platformOptions, paths, files)
			if err != nil {
				return err
			}
			id, ref = thisID, thisRef
			instancesLock.Lock()
			instances = append(instances, instance{
				ID: thisID,
				Platform: v1.Platform{
					OS:           platformContext.OSChoice,
					Architecture: platformContext.ArchitectureChoice,
					Variant:      platformContext.VariantChoice,
				},
			})
			instancesLock.Unlock()
			return nil
		})
	}

	if merr := builds.Wait(); merr != nil {
		if merr.Len() == 1 {
			return "", nil, merr.Errors[0]
		}
		return "", nil, merr.ErrorOrNil()
	}

	if manifestList != "" {
		rt, err := libimage.RuntimeFromStore(store, nil)
		if err != nil {
			return "", nil, err
		}
		// Create the manifest list ourselves, so that it's not in a
		// partially-populated state at any point if we're creating it
		// fresh.
		list, err := rt.LookupManifestList(manifestList)
		if err != nil && errors.Cause(err) == storage.ErrImageUnknown {
			list, err = rt.CreateManifestList(manifestList)
		}
		if err != nil {
			return "", nil, err
		}
		// Add each instance to the list in turn.
		storeTransportName := istorage.Transport.Name()
		for _, instance := range instances {
			instanceDigest, err := list.Add(ctx, storeTransportName+":"+instance.ID, nil)
			if err != nil {
				return "", nil, err
			}
			err = list.AnnotateInstance(instanceDigest, &libimage.ManifestListAnnotateOptions{
				Architecture: instance.Architecture,
				OS:           instance.OS,
				Variant:      instance.Variant,
			})
			if err != nil {
				return "", nil, err
			}
		}
		id, ref = list.ID(), nil
		// Put together a canonical reference
		storeRef, err := istorage.Transport.NewStoreReference(store, nil, list.ID())
		if err != nil {
			return "", nil, err
		}
		imgSource, err := storeRef.NewImageSource(ctx, nil)
		if err != nil {
			return "", nil, err
		}
		defer imgSource.Close()
		manifestBytes, _, err := imgSource.GetManifest(ctx, nil)
		if err != nil {
			return "", nil, err
		}
		manifestDigest, err := manifest.Digest(manifestBytes)
		if err != nil {
			return "", nil, err
		}
		img, err := store.Image(id)
		if err != nil {
			return "", nil, err
		}
		for _, name := range img.Names {
			if named, err := reference.ParseNamed(name); err == nil {
				if r, err := reference.WithDigest(reference.TrimNamed(named), manifestDigest); err == nil {
					ref = r
					break
				}
			}
		}
	}

	return id, ref, nil
}

func buildDockerfilesOnce(ctx context.Context, store storage.Store, logger *logrus.Logger, logPrefix string, options define.BuildOptions, dockerfiles []string, dockerfilecontents [][]byte) (string, reference.Canonical, error) {
	mainNode, err := imagebuilder.ParseDockerfile(bytes.NewReader(dockerfilecontents[0]))
	if err != nil {
		return "", nil, errors.Wrapf(err, "error parsing main Dockerfile: %s", dockerfiles[0])
	}

	warnOnUnsetBuildArgs(logger, mainNode, options.Args)

	for i, d := range dockerfilecontents[1:] {
		additionalNode, err := imagebuilder.ParseDockerfile(bytes.NewReader(d))
		if err != nil {
			return "", nil, errors.Wrapf(err, "error parsing additional Dockerfile %s", dockerfiles[i])
		}
		mainNode.Children = append(mainNode.Children, additionalNode.Children...)
	}
	exec, err := newExecutor(logger, logPrefix, store, options, mainNode)
	if err != nil {
		return "", nil, errors.Wrapf(err, "error creating build executor")
	}
	b := imagebuilder.NewBuilder(options.Args)
	defaultContainerConfig, err := config.Default()
	if err != nil {
		return "", nil, errors.Wrapf(err, "failed to get container config")
	}
	b.Env = append(defaultContainerConfig.GetDefaultEnv(), b.Env...)
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

func warnOnUnsetBuildArgs(logger *logrus.Logger, node *parser.Node, args map[string]string) {
	argFound := make(map[string]bool)
	for _, child := range node.Children {
		switch strings.ToUpper(child.Value) {
		case "ARG":
			argName := child.Next.Value
			if strings.Contains(argName, "=") {
				res := strings.Split(argName, "=")
				if res[1] != "" {
					argFound[res[0]] = true
				}
			}
			argHasValue := true
			if !strings.Contains(argName, "=") {
				argHasValue = argFound[argName]
			}
			if _, ok := args[argName]; !argHasValue && !ok {
				logger.Warnf("missing %q build argument. Try adding %q to the command line", argName, fmt.Sprintf("--build-arg %s=<VALUE>", argName))
			}
		default:
			continue
		}
	}
}

// preprocessContainerfileContents runs CPP(1) in preprocess-only mode on the input
// dockerfile content and will use ctxDir as the base include path.
func preprocessContainerfileContents(logger *logrus.Logger, containerfile string, r io.Reader, ctxDir string) (stdout io.Reader, err error) {
	cppCommand := "cpp"
	cppPath, err := exec.LookPath(cppCommand)
	if err != nil {
		if os.IsNotExist(err) {
			err = errors.Errorf("error: %s support requires %s to be installed", containerfile, cppPath)
		}
		return nil, err
	}

	stdoutBuffer := bytes.Buffer{}
	stderrBuffer := bytes.Buffer{}

	cmd := exec.Command(cppPath, "-E", "-iquote", ctxDir, "-traditional", "-undef", "-")
	cmd.Stdin = r
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer

	if err = cmd.Start(); err != nil {
		return nil, errors.Wrapf(err, "preprocessing %s", containerfile)
	}
	if err = cmd.Wait(); err != nil {
		if stderrBuffer.Len() != 0 {
			logger.Warnf("Ignoring %s\n", stderrBuffer.String())
		}
		if stdoutBuffer.Len() == 0 {
			return nil, errors.Wrapf(err, "error preprocessing %s: preprocessor produced no output", containerfile)
		}
	}
	return &stdoutBuffer, nil
}
