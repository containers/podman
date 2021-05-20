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

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/util"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerfile/parser"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
type Mount specs.Mount

type BuildOptions = define.BuildOptions

// BuildDockerfiles parses a set of one or more Dockerfiles (which may be
// URLs), creates a new Executor, and then runs Prepare/Execute/Commit/Delete
// over the entire set of instructions.
func BuildDockerfiles(ctx context.Context, store storage.Store, options define.BuildOptions, paths ...string) (string, reference.Canonical, error) {
	if len(paths) == 0 {
		return "", nil, errors.Errorf("error building: no dockerfiles specified")
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
			logrus.Debugf("reading remote Dockerfile %q", dfile)
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
					logrus.Debugf("reading local %q", f)
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
			pData, err := preprocessContainerfileContents(dfile, data, options.ContextDirectory)
			if err != nil {
				return "", nil, err
			}
			data = *pData
		}

		dockerfiles = append(dockerfiles, data)
	}

	mainNode, err := imagebuilder.ParseDockerfile(dockerfiles[0])
	if err != nil {
		return "", nil, errors.Wrapf(err, "error parsing main Dockerfile: %s", dockerfiles[0])
	}

	warnOnUnsetBuildArgs(logger, mainNode, options.Args)

	for _, d := range dockerfiles[1:] {
		additionalNode, err := imagebuilder.ParseDockerfile(d)
		if err != nil {
			return "", nil, errors.Wrapf(err, "error parsing additional Dockerfile %s", d)
		}
		mainNode.Children = append(mainNode.Children, additionalNode.Children...)
	}
	exec, err := NewExecutor(logger, store, options, mainNode)
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
//
// Note: we cannot use cmd.StdoutPipe() as cmd.Wait() closes it.
func preprocessContainerfileContents(containerfile string, r io.Reader, ctxDir string) (rdrCloser *io.ReadCloser, err error) {
	cppPath := "/usr/bin/cpp"
	if _, err = os.Stat(cppPath); err != nil {
		if os.IsNotExist(err) {
			err = errors.Errorf("error: %s support requires %s to be installed", containerfile, cppPath)
		}
		return nil, err
	}

	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}

	cmd := exec.Command(cppPath, "-E", "-iquote", ctxDir, "-traditional", "-undef", "-")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	defer pipe.Close()

	if err = cmd.Start(); err != nil {
		return nil, err
	}

	if _, err = io.Copy(pipe, r); err != nil {
		return nil, err
	}

	pipe.Close()
	if err = cmd.Wait(); err != nil {
		if stdout.Len() == 0 {
			return nil, errors.Wrapf(err, "error pre-processing Dockerfile")
		}
		logrus.Warnf("Ignoring %s\n", stderr.String())
	}

	rc := ioutil.NopCloser(bytes.NewReader(stdout.Bytes()))
	return &rc, nil
}
