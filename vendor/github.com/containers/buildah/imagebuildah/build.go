package imagebuildah

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/containerd/containerd/platforms"
	"github.com/containers/buildah/define"
	internalUtil "github.com/containers/buildah/internal/util"
	"github.com/containers/buildah/util"
	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/shortnames"
	istorage "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/hashicorp/go-multierror"
	"github.com/mattn/go-shellwords"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerfile/parser"
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
	if options.CommonBuildOpts == nil {
		options.CommonBuildOpts = &define.CommonBuildOptions{}
	}

	if len(paths) == 0 {
		return "", nil, errors.New("building: no dockerfiles specified")
	}
	if len(options.Platforms) > 1 && options.IIDFile != "" {
		return "", nil, fmt.Errorf("building multiple images, but iidfile %q can only be used to store one image ID", options.IIDFile)
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
			return "", nil, fmt.Errorf("tag %s: %w", tag, err)
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
				return "", nil, fmt.Errorf("no contents in %q", dfile)
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
			// If given a directory error out since `-f` does not supports path to directory
			if dinfo.Mode().IsDir() {
				return "", nil, fmt.Errorf("containerfile: %q cannot be path to a directory", dfile)
			}
			contents, err = os.Open(dfile)
			if err != nil {
				return "", nil, err
			}
			dinfo, err = contents.Stat()
			if err != nil {
				contents.Close()
				return "", nil, fmt.Errorf("reading info about %q: %w", dfile, err)
			}
			if dinfo.Mode().IsRegular() && dinfo.Size() == 0 {
				contents.Close()
				return "", nil, fmt.Errorf("no contents in %q", dfile)
			}
			data = contents
		}

		// pre-process Dockerfiles with ".in" suffix
		if strings.HasSuffix(dfile, ".in") {
			pData, err := preprocessContainerfileContents(logger, dfile, data, options.ContextDirectory, options.CPPFlags)
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

	if options.JobSemaphore == nil {
		if options.Jobs != nil {
			if *options.Jobs < 0 {
				return "", nil, errors.New("building: invalid value for jobs.  It must be a positive integer")
			}
			if *options.Jobs > 0 {
				options.JobSemaphore = semaphore.NewWeighted(int64(*options.Jobs))
			}
		} else {
			options.JobSemaphore = semaphore.NewWeighted(1)
		}
	}

	manifestList := options.Manifest
	options.Manifest = ""
	type instance struct {
		v1.Platform
		ID  string
		Ref reference.Canonical
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

	if options.AllPlatforms {
		if options.AdditionalBuildContexts == nil {
			options.AdditionalBuildContexts = make(map[string]*define.AdditionalBuildContext)
		}
		options.Platforms, err = platformsForBaseImages(ctx, logger, paths, files, options.From, options.Args, options.AdditionalBuildContexts, options.SystemContext)
		if err != nil {
			return "", nil, err
		}
	}

	systemContext := options.SystemContext
	for _, platform := range options.Platforms {
		platformContext := *systemContext
		platformSpec := internalUtil.NormalizePlatform(v1.Platform{
			OS:           platform.OS,
			Architecture: platform.Arch,
			Variant:      platform.Variant,
		})
		// internalUtil.NormalizePlatform converts an empty os value to GOOS
		// so we have to check the original value here to not overwrite the default for no reason
		if platform.OS != "" {
			platformContext.OSChoice = platformSpec.OS
		}
		if platform.Arch != "" {
			platformContext.ArchitectureChoice = platformSpec.Architecture
			platformContext.VariantChoice = platformSpec.Variant
		}
		platformOptions := options
		platformOptions.SystemContext = &platformContext
		platformOptions.OS = platformContext.OSChoice
		platformOptions.Architecture = platformContext.ArchitectureChoice
		logPrefix := ""
		if len(options.Platforms) > 1 {
			logPrefix = "[" + platforms.Format(platformSpec) + "] "
		}
		// Deep copy args to prevent concurrent read/writes over Args.
		argsCopy := make(map[string]string)
		for key, value := range options.Args {
			argsCopy[key] = value
		}
		platformOptions.Args = argsCopy
		builds.Go(func() error {
			loggerPerPlatform := logger
			if platformOptions.LogFile != "" && platformOptions.LogSplitByPlatform {
				logFile := platformOptions.LogFile + "_" + platformOptions.OS + "_" + platformOptions.Architecture
				f, err := os.OpenFile(logFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
				if err != nil {
					return fmt.Errorf("opening logfile: %q: %w", logFile, err)
				}
				defer f.Close()
				loggerPerPlatform = logrus.New()
				loggerPerPlatform.SetOutput(f)
				loggerPerPlatform.SetLevel(logrus.GetLevel())
				stdout := f
				stderr := f
				reporter := f
				platformOptions.Out = stdout
				platformOptions.ReportWriter = reporter
				platformOptions.Err = stderr
			}
			thisID, thisRef, err := buildDockerfilesOnce(ctx, store, loggerPerPlatform, logPrefix, platformOptions, paths, files)
			if err != nil {
				return err
			}
			instancesLock.Lock()
			instances = append(instances, instance{
				ID:       thisID,
				Ref:      thisRef,
				Platform: platformSpec,
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

	// Reasons for this id, ref assignment w.r.t to use-case:
	//
	// * Single-platform build: On single platform build we only
	//   have one built instance i.e on indice 0 of built instances,
	//   so assign that.
	//
	// * Multi-platform build with manifestList: If this is a build for
	//   multiple platforms ( more than one platform ) and --manifest
	//   option then this assignment is insignificant since it will be
	//   overridden anyways with the id and ref of manifest list later in
	//   in this code.
	//
	// * Multi-platform build without manifest list: If this is a build for
	//   multiple platforms without --manifest then we are free to return
	//   id and ref of any one of the image in the instance list so always
	//   return indice 0 for predictable output instead returning the id and
	//   ref of the go routine which completed at last.
	id, ref = instances[0].ID, instances[0].Ref

	if manifestList != "" {
		rt, err := libimage.RuntimeFromStore(store, nil)
		if err != nil {
			return "", nil, err
		}
		// Create the manifest list ourselves, so that it's not in a
		// partially-populated state at any point if we're creating it
		// fresh.
		list, err := rt.LookupManifestList(manifestList)
		if err != nil && errors.Is(err, storage.ErrImageUnknown) {
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

func buildDockerfilesOnce(ctx context.Context, store storage.Store, logger *logrus.Logger, logPrefix string, options define.BuildOptions, containerFiles []string, dockerfilecontents [][]byte) (string, reference.Canonical, error) {
	mainNode, err := imagebuilder.ParseDockerfile(bytes.NewReader(dockerfilecontents[0]))
	if err != nil {
		return "", nil, fmt.Errorf("parsing main Dockerfile: %s: %w", containerFiles[0], err)
	}

	// --platform was explicitly selected for this build
	// so set correct TARGETPLATFORM in args if it is not
	// already selected by the user.
	if options.SystemContext.OSChoice != "" && options.SystemContext.ArchitectureChoice != "" {
		// os component from --platform string populates TARGETOS
		// buildkit parity: give priority to user's `--build-arg`
		if _, ok := options.Args["TARGETOS"]; !ok {
			options.Args["TARGETOS"] = options.SystemContext.OSChoice
		}
		// arch component from --platform string populates TARGETARCH
		// buildkit parity: give priority to user's `--build-arg`
		if _, ok := options.Args["TARGETARCH"]; !ok {
			options.Args["TARGETARCH"] = options.SystemContext.ArchitectureChoice
		}
		// variant component from --platform string populates TARGETVARIANT
		// buildkit parity: give priority to user's `--build-arg`
		if _, ok := options.Args["TARGETVARIANT"]; !ok {
			if options.SystemContext.VariantChoice != "" {
				options.Args["TARGETVARIANT"] = options.SystemContext.VariantChoice
			}
		}
		// buildkit parity: give priority to user's `--build-arg`
		if _, ok := options.Args["TARGETPLATFORM"]; !ok {
			// buildkit parity: TARGETPLATFORM should be always created
			// from SystemContext and not `TARGETOS` and `TARGETARCH` because
			// users can always override values of `TARGETOS` and `TARGETARCH`
			// but `TARGETPLATFORM` should be set independent of those values.
			options.Args["TARGETPLATFORM"] = options.SystemContext.OSChoice + "/" + options.SystemContext.ArchitectureChoice
			if options.SystemContext.VariantChoice != "" {
				options.Args["TARGETPLATFORM"] = options.Args["TARGETPLATFORM"] + "/" + options.SystemContext.VariantChoice
			}
		}
	}

	warnOnUnsetBuildArgs(logger, mainNode, options.Args)

	for i, d := range dockerfilecontents[1:] {
		additionalNode, err := imagebuilder.ParseDockerfile(bytes.NewReader(d))
		if err != nil {
			containerFiles := containerFiles[1:]
			return "", nil, fmt.Errorf("parsing additional Dockerfile %s: %w", containerFiles[i], err)
		}
		mainNode.Children = append(mainNode.Children, additionalNode.Children...)
	}

	// Check if any modifications done to labels
	// add them to node-layer so it becomes regular
	// layer.
	// Reason: Docker adds label modification as
	// last step which can be processed as regular
	// steps and if no modification is done to layers
	// its easier to re-use cached layers.
	if len(options.Labels) > 0 {
		for _, labelSpec := range options.Labels {
			label := strings.SplitN(labelSpec, "=", 2)
			labelLine := ""
			key := label[0]
			value := ""
			if len(label) > 1 {
				value = label[1]
			}
			// check from only empty key since docker supports empty value
			if key != "" {
				labelLine = fmt.Sprintf("LABEL %q=%q\n", key, value)
				additionalNode, err := imagebuilder.ParseDockerfile(strings.NewReader(labelLine))
				if err != nil {
					return "", nil, fmt.Errorf("while adding additional LABEL steps: %w", err)
				}
				mainNode.Children = append(mainNode.Children, additionalNode.Children...)
			}
		}
	}

	exec, err := newExecutor(logger, logPrefix, store, options, mainNode, containerFiles)
	if err != nil {
		return "", nil, fmt.Errorf("creating build executor: %w", err)
	}
	b := imagebuilder.NewBuilder(options.Args)
	defaultContainerConfig, err := config.Default()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get container config: %w", err)
	}
	b.Env = append(defaultContainerConfig.GetDefaultEnv(), b.Env...)
	stages, err := imagebuilder.NewStages(mainNode, b)
	if err != nil {
		return "", nil, fmt.Errorf("reading multiple stages: %w", err)
	}
	if options.Target != "" {
		stagesTargeted, ok := stages.ThroughTarget(options.Target)
		if !ok {
			return "", nil, fmt.Errorf("The target %q was not found in the provided Dockerfile", options.Target)
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
func preprocessContainerfileContents(logger *logrus.Logger, containerfile string, r io.Reader, ctxDir string, cppFlags []string) (stdout io.Reader, err error) {
	cppCommand := "cpp"
	cppPath, err := exec.LookPath(cppCommand)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			err = fmt.Errorf("%v: .in support requires %s to be installed", err, cppCommand)
		}
		return nil, err
	}

	stdoutBuffer := bytes.Buffer{}
	stderrBuffer := bytes.Buffer{}

	cppArgs := []string{"-E", "-iquote", ctxDir, "-traditional", "-undef", "-"}
	if flags, ok := os.LookupEnv("BUILDAH_CPPFLAGS"); ok {
		args, err := shellwords.Parse(flags)
		if err != nil {
			return nil, fmt.Errorf("parsing BUILDAH_CPPFLAGS %q: %v", flags, err)
		}
		cppArgs = append(cppArgs, args...)
	}
	cppArgs = append(cppArgs, cppFlags...)
	cmd := exec.Command(cppPath, cppArgs...)
	cmd.Stdin = r
	cmd.Stdout = &stdoutBuffer
	cmd.Stderr = &stderrBuffer

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("preprocessing %s: %w", containerfile, err)
	}
	if err = cmd.Wait(); err != nil {
		if stderrBuffer.Len() != 0 {
			logger.Warnf("Ignoring %s\n", stderrBuffer.String())
		}
		if stdoutBuffer.Len() == 0 {
			return nil, fmt.Errorf("preprocessing %s: preprocessor produced no output: %w", containerfile, err)
		}
	}
	return &stdoutBuffer, nil
}

// platformsForBaseImages resolves the names of base images from the
// dockerfiles, and if they are all valid references to manifest lists, returns
// the list of platforms that are supported by all of the base images.
func platformsForBaseImages(ctx context.Context, logger *logrus.Logger, dockerfilepaths []string, dockerfiles [][]byte, from string, args map[string]string, additionalBuildContext map[string]*define.AdditionalBuildContext, systemContext *types.SystemContext) ([]struct{ OS, Arch, Variant string }, error) {
	baseImages, err := baseImages(dockerfilepaths, dockerfiles, from, args, additionalBuildContext)
	if err != nil {
		return nil, fmt.Errorf("determining list of base images: %w", err)
	}
	logrus.Debugf("unresolved base images: %v", baseImages)
	if len(baseImages) == 0 {
		return nil, fmt.Errorf("build uses no non-scratch base images: %w", err)
	}
	targetPlatforms := make(map[string]struct{})
	var platformList []struct{ OS, Arch, Variant string }
	for baseImageIndex, baseImage := range baseImages {
		resolved, err := shortnames.Resolve(systemContext, baseImage)
		if err != nil {
			return nil, fmt.Errorf("resolving image name %q: %w", baseImage, err)
		}
		var manifestBytes []byte
		var manifestType string
		for _, candidate := range resolved.PullCandidates {
			ref, err := docker.NewReference(candidate.Value)
			if err != nil {
				logrus.Debugf("parsing image reference %q: %v", candidate.Value.String(), err)
				continue
			}
			src, err := ref.NewImageSource(ctx, systemContext)
			if err != nil {
				logrus.Debugf("preparing to read image manifest for %q: %v", baseImage, err)
				continue
			}
			candidateBytes, candidateType, err := src.GetManifest(ctx, nil)
			_ = src.Close()
			if err != nil {
				logrus.Debugf("reading image manifest for %q: %v", baseImage, err)
				continue
			}
			if !manifest.MIMETypeIsMultiImage(candidateType) {
				logrus.Debugf("base image %q is not a reference to a manifest list: %v", baseImage, err)
				continue
			}
			if err := candidate.Record(); err != nil {
				logrus.Debugf("error recording name %q for base image %q: %v", candidate.Value.String(), baseImage, err)
				continue
			}
			baseImage = candidate.Value.String()
			manifestBytes, manifestType = candidateBytes, candidateType
			break
		}
		if len(manifestBytes) == 0 {
			if len(resolved.PullCandidates) > 0 {
				return nil, fmt.Errorf("base image name %q didn't resolve to a manifest list", baseImage)
			}
			return nil, fmt.Errorf("base image name %q didn't resolve to anything", baseImage)
		}
		if manifestType != v1.MediaTypeImageIndex {
			list, err := manifest.ListFromBlob(manifestBytes, manifestType)
			if err != nil {
				return nil, fmt.Errorf("parsing manifest list from base image %q: %w", baseImage, err)
			}
			list, err = list.ConvertToMIMEType(v1.MediaTypeImageIndex)
			if err != nil {
				return nil, fmt.Errorf("converting manifest list from base image %q to v2s2 list: %w", baseImage, err)
			}
			manifestBytes, err = list.Serialize()
			if err != nil {
				return nil, fmt.Errorf("encoding converted v2s2 manifest list for base image %q: %w", baseImage, err)
			}
		}
		index, err := manifest.OCI1IndexFromManifest(manifestBytes)
		if err != nil {
			return nil, fmt.Errorf("decoding manifest list for base image %q: %w", baseImage, err)
		}
		if baseImageIndex == 0 {
			// populate the list with the first image's normalized platforms
			for _, instance := range index.Manifests {
				if instance.Platform == nil {
					continue
				}
				platform := internalUtil.NormalizePlatform(*instance.Platform)
				targetPlatforms[platforms.Format(platform)] = struct{}{}
				logger.Debugf("image %q supports %q", baseImage, platforms.Format(platform))
			}
		} else {
			// prune the list of any normalized platforms this base image doesn't support
			imagePlatforms := make(map[string]struct{})
			for _, instance := range index.Manifests {
				if instance.Platform == nil {
					continue
				}
				platform := internalUtil.NormalizePlatform(*instance.Platform)
				imagePlatforms[platforms.Format(platform)] = struct{}{}
				logger.Debugf("image %q supports %q", baseImage, platforms.Format(platform))
			}
			var removed []string
			for platform := range targetPlatforms {
				if _, present := imagePlatforms[platform]; !present {
					removed = append(removed, platform)
					logger.Debugf("image %q does not support %q", baseImage, platform)
				}
			}
			for _, remove := range removed {
				delete(targetPlatforms, remove)
			}
		}
		if baseImageIndex == len(baseImages)-1 && len(targetPlatforms) > 0 {
			// extract the list
			for platform := range targetPlatforms {
				platform, err := platforms.Parse(platform)
				if err != nil {
					return nil, fmt.Errorf("parsing platform double/triple %q: %w", platform, err)
				}
				platformList = append(platformList, struct{ OS, Arch, Variant string }{
					OS:      platform.OS,
					Arch:    platform.Architecture,
					Variant: platform.Variant,
				})
				logger.Debugf("base images all support %q", platform)
			}
		}
	}
	if len(platformList) == 0 {
		return nil, errors.New("base images have no platforms in common")
	}
	return platformList, nil
}

// baseImages parses the dockerfilecontents, possibly replacing the first
// stage's base image with FROM, and returns the list of base images as
// provided.  Each entry in the dockerfilenames slice corresponds to a slice in
// dockerfilecontents.
func baseImages(dockerfilenames []string, dockerfilecontents [][]byte, from string, args map[string]string, additionalBuildContext map[string]*define.AdditionalBuildContext) ([]string, error) {
	mainNode, err := imagebuilder.ParseDockerfile(bytes.NewReader(dockerfilecontents[0]))
	if err != nil {
		return nil, fmt.Errorf("parsing main Dockerfile: %s: %w", dockerfilenames[0], err)
	}

	for i, d := range dockerfilecontents[1:] {
		additionalNode, err := imagebuilder.ParseDockerfile(bytes.NewReader(d))
		if err != nil {
			dockerfilenames := dockerfilenames[1:]
			return nil, fmt.Errorf("parsing additional Dockerfile %s: %w", dockerfilenames[i], err)
		}
		mainNode.Children = append(mainNode.Children, additionalNode.Children...)
	}

	b := imagebuilder.NewBuilder(args)
	defaultContainerConfig, err := config.Default()
	if err != nil {
		return nil, fmt.Errorf("failed to get container config: %w", err)
	}
	b.Env = defaultContainerConfig.GetDefaultEnv()
	stages, err := imagebuilder.NewStages(mainNode, b)
	if err != nil {
		return nil, fmt.Errorf("reading multiple stages: %w", err)
	}
	var baseImages []string
	nicknames := make(map[string]bool)
	for stageIndex, stage := range stages {
		node := stage.Node // first line
		for node != nil {  // each line
			for _, child := range node.Children { // tokens on this line, though we only care about the first
				switch strings.ToUpper(child.Value) { // first token - instruction
				case "FROM":
					if child.Next != nil { // second token on this line
						// If we have a fromOverride, replace the value of
						// image name for the first FROM in the Containerfile.
						if from != "" {
							child.Next.Value = from
							from = ""
						}
						if replaceBuildContext, ok := additionalBuildContext[child.Next.Value]; ok {
							if replaceBuildContext.IsImage {
								child.Next.Value = replaceBuildContext.Value
							} else {
								return nil, fmt.Errorf("build context %q is not an image, can not be used for FROM %q", child.Next.Value, child.Next.Value)
							}
						}
						base := child.Next.Value
						if base != "scratch" && !nicknames[base] {
							// TODO: this didn't undergo variable and arg
							// expansion, so if the AS clause in another
							// FROM instruction uses argument values,
							// we might not record the right value here.
							baseImages = append(baseImages, base)
						}
					}
				}
			}
			node = node.Next // next line
		}
		if stage.Name != strconv.Itoa(stageIndex) {
			nicknames[stage.Name] = true
		}
	}
	return baseImages, nil
}
