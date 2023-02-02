package imagebuildah

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/pkg/sshagent"
	"github.com/containers/buildah/util"
	"github.com/containers/common/libimage"
	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	is "github.com/containers/image/v5/storage"
	storageTransport "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	encconfig "github.com/containers/ocicrypt/config"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerfile/parser"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

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

// Executor is a buildah-based implementation of the imagebuilder.Executor
// interface.  It coordinates the entire build by using one or more
// StageExecutors to handle each stage of the build.
type Executor struct {
	cacheFrom                      reference.Named
	cacheTo                        reference.Named
	cacheTTL                       time.Duration
	containerSuffix                string
	logger                         *logrus.Logger
	stages                         map[string]*StageExecutor
	store                          storage.Store
	contextDir                     string
	pullPolicy                     define.PullPolicy
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
	log                            func(format string, args ...interface{}) // can be nil
	in                             io.Reader
	out                            io.Writer
	err                            io.Writer
	signaturePolicyPath            string
	skipUnusedStages               types.OptionalBool
	systemContext                  *types.SystemContext
	reportWriter                   io.Writer
	isolation                      define.Isolation
	namespaceOptions               []define.NamespaceOption
	configureNetwork               define.NetworkConfigurationPolicy
	cniPluginPath                  string
	cniConfigDir                   string
	// NetworkInterface is the libnetwork network interface used to setup CNI or netavark networks.
	networkInterface        nettypes.ContainerNetwork
	idmappingOptions        *define.IDMappingOptions
	commonBuildOptions      *define.CommonBuildOptions
	defaultMountsFilePath   string
	iidfile                 string
	squash                  bool
	labels                  []string
	annotations             []string
	layers                  bool
	noHosts                 bool
	useCache                bool
	removeIntermediateCtrs  bool
	forceRmIntermediateCtrs bool
	imageMap                map[string]string           // Used to map images that we create to handle the AS construct.
	containerMap            map[string]*buildah.Builder // Used to map from image names to only-created-for-the-rootfs containers.
	baseMap                 map[string]bool             // Holds the names of every base image, as given.
	rootfsMap               map[string]bool             // Holds the names of every stage whose rootfs is referenced in a COPY or ADD instruction.
	blobDirectory           string
	excludes                []string
	ignoreFile              string
	unusedArgs              map[string]struct{}
	capabilities            []string
	devices                 define.ContainerDevices
	signBy                  string
	architecture            string
	timestamp               *time.Time
	os                      string
	maxPullPushRetries      int
	retryPullPushDelay      time.Duration
	ociDecryptConfig        *encconfig.DecryptConfig
	lastError               error
	terminatedStage         map[string]error
	stagesLock              sync.Mutex
	stagesSemaphore         *semaphore.Weighted
	logRusage               bool
	rusageLogFile           io.Writer
	imageInfoLock           sync.Mutex
	imageInfoCache          map[string]imageTypeAndHistoryAndDiffIDs
	fromOverride            string
	additionalBuildContexts map[string]*define.AdditionalBuildContext
	manifest                string
	secrets                 map[string]define.Secret
	sshsources              map[string]*sshagent.Source
	logPrefix               string
	unsetEnvs               []string
	processLabel            string // Shares processLabel of first stage container with containers of other stages in same build
	mountLabel              string // Shares mountLabel of first stage container with containers of other stages in same build
	buildOutput             string // Specifies instructions for any custom build output
	osVersion               string
	osFeatures              []string
	envs                    []string
}

type imageTypeAndHistoryAndDiffIDs struct {
	manifestType string
	history      []v1.History
	diffIDs      []digest.Digest
	err          error
}

// newExecutor creates a new instance of the imagebuilder.Executor interface.
func newExecutor(logger *logrus.Logger, logPrefix string, store storage.Store, options define.BuildOptions, mainNode *parser.Node, containerFiles []string) (*Executor, error) {
	defaultContainerConfig, err := config.Default()
	if err != nil {
		return nil, fmt.Errorf("failed to get container config: %w", err)
	}

	excludes := options.Excludes
	if len(excludes) == 0 {
		excludes, options.IgnoreFile, err = parse.ContainerIgnoreFile(options.ContextDirectory, options.IgnoreFile, containerFiles)
		if err != nil {
			return nil, err
		}
	}
	capabilities, err := defaultContainerConfig.Capabilities("", options.AddCapabilities, options.DropCapabilities)
	if err != nil {
		return nil, err
	}

	devices := define.ContainerDevices{}
	for _, device := range append(defaultContainerConfig.Containers.Devices, options.Devices...) {
		dev, err := parse.DeviceFromPath(device)
		if err != nil {
			return nil, err
		}
		devices = append(dev, devices...)
	}

	transientMounts := []Mount{}
	for _, volume := range append(defaultContainerConfig.Containers.Volumes, options.TransientMounts...) {
		mount, err := parse.Volume(volume)
		if err != nil {
			return nil, err
		}
		transientMounts = append([]Mount{mount}, transientMounts...)
	}

	secrets, err := parse.Secrets(options.CommonBuildOpts.Secrets)
	if err != nil {
		return nil, err
	}
	sshsources, err := parse.SSH(options.CommonBuildOpts.SSHSources)
	if err != nil {
		return nil, err
	}

	writer := options.ReportWriter
	if options.Quiet {
		writer = ioutil.Discard
	}

	var rusageLogFile io.Writer

	if options.LogRusage && !options.Quiet {
		if options.RusageLogFile == "" {
			rusageLogFile = options.Out
		} else {
			rusageLogFile, err = os.OpenFile(options.RusageLogFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return nil, err
			}
		}
	}

	exec := Executor{
		cacheFrom:                      options.CacheFrom,
		cacheTo:                        options.CacheTo,
		cacheTTL:                       options.CacheTTL,
		containerSuffix:                options.ContainerSuffix,
		logger:                         logger,
		stages:                         make(map[string]*StageExecutor),
		store:                          store,
		contextDir:                     options.ContextDirectory,
		excludes:                       excludes,
		ignoreFile:                     options.IgnoreFile,
		pullPolicy:                     options.PullPolicy,
		registry:                       options.Registry,
		ignoreUnrecognizedInstructions: options.IgnoreUnrecognizedInstructions,
		quiet:                          options.Quiet,
		runtime:                        options.Runtime,
		runtimeArgs:                    options.RuntimeArgs,
		transientMounts:                transientMounts,
		compression:                    options.Compression,
		output:                         options.Output,
		outputFormat:                   options.OutputFormat,
		additionalTags:                 options.AdditionalTags,
		signaturePolicyPath:            options.SignaturePolicyPath,
		skipUnusedStages:               options.SkipUnusedStages,
		systemContext:                  options.SystemContext,
		log:                            options.Log,
		in:                             options.In,
		out:                            options.Out,
		err:                            options.Err,
		reportWriter:                   writer,
		isolation:                      options.Isolation,
		namespaceOptions:               options.NamespaceOptions,
		configureNetwork:               options.ConfigureNetwork,
		cniPluginPath:                  options.CNIPluginPath,
		cniConfigDir:                   options.CNIConfigDir,
		networkInterface:               options.NetworkInterface,
		idmappingOptions:               options.IDMappingOptions,
		commonBuildOptions:             options.CommonBuildOpts,
		defaultMountsFilePath:          options.DefaultMountsFilePath,
		iidfile:                        options.IIDFile,
		squash:                         options.Squash,
		labels:                         append([]string{}, options.Labels...),
		annotations:                    append([]string{}, options.Annotations...),
		layers:                         options.Layers,
		noHosts:                        options.CommonBuildOpts.NoHosts,
		useCache:                       !options.NoCache,
		removeIntermediateCtrs:         options.RemoveIntermediateCtrs,
		forceRmIntermediateCtrs:        options.ForceRmIntermediateCtrs,
		imageMap:                       make(map[string]string),
		containerMap:                   make(map[string]*buildah.Builder),
		baseMap:                        make(map[string]bool),
		rootfsMap:                      make(map[string]bool),
		blobDirectory:                  options.BlobDirectory,
		unusedArgs:                     make(map[string]struct{}),
		capabilities:                   capabilities,
		devices:                        devices,
		signBy:                         options.SignBy,
		architecture:                   options.Architecture,
		timestamp:                      options.Timestamp,
		os:                             options.OS,
		maxPullPushRetries:             options.MaxPullPushRetries,
		retryPullPushDelay:             options.PullPushRetryDelay,
		ociDecryptConfig:               options.OciDecryptConfig,
		terminatedStage:                make(map[string]error),
		stagesSemaphore:                options.JobSemaphore,
		logRusage:                      options.LogRusage,
		rusageLogFile:                  rusageLogFile,
		imageInfoCache:                 make(map[string]imageTypeAndHistoryAndDiffIDs),
		fromOverride:                   options.From,
		additionalBuildContexts:        options.AdditionalBuildContexts,
		manifest:                       options.Manifest,
		secrets:                        secrets,
		sshsources:                     sshsources,
		logPrefix:                      logPrefix,
		unsetEnvs:                      append([]string{}, options.UnsetEnvs...),
		buildOutput:                    options.BuildOutput,
		osVersion:                      options.OSVersion,
		osFeatures:                     append([]string{}, options.OSFeatures...),
		envs:                           append([]string{}, options.Envs...),
	}
	if exec.err == nil {
		exec.err = os.Stderr
	}
	if exec.out == nil {
		exec.out = os.Stdout
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
					delete(exec.unusedArgs, list[0])
				}
			}
			break
		}
	}
	return &exec, nil
}

// startStage creates a new stage executor that will be referenced whenever a
// COPY or ADD statement uses a --from=NAME flag.
func (b *Executor) startStage(ctx context.Context, stage *imagebuilder.Stage, stages imagebuilder.Stages, output string) *StageExecutor {
	stageExec := &StageExecutor{
		ctx:             ctx,
		executor:        b,
		log:             b.log,
		index:           stage.Position,
		stages:          stages,
		name:            stage.Name,
		volumeCache:     make(map[string]string),
		volumeCacheInfo: make(map[string]os.FileInfo),
		output:          output,
		stage:           stage,
	}
	b.stages[stage.Name] = stageExec
	if idx := strconv.Itoa(stage.Position); idx != stage.Name {
		b.stages[idx] = stageExec
	}
	return stageExec
}

// resolveNameToImageRef creates a types.ImageReference for the output name in local storage
func (b *Executor) resolveNameToImageRef(output string) (types.ImageReference, error) {
	if imageRef, err := alltransports.ParseImageName(output); err == nil {
		return imageRef, nil
	}
	runtime, err := libimage.RuntimeFromStore(b.store, &libimage.RuntimeOptions{SystemContext: b.systemContext})
	if err != nil {
		return nil, err
	}
	resolved, err := runtime.ResolveName(output)
	if err != nil {
		return nil, err
	}
	imageRef, err := storageTransport.Transport.ParseStoreReference(b.store, resolved)
	if err == nil {
		return imageRef, nil
	}

	return imageRef, err
}

// waitForStage waits for an entry to be added to terminatedStage indicating
// that the specified stage has finished.  If there is no stage defined by that
// name, then it will return (false, nil).  If there is a stage defined by that
// name, it will return true along with any error it encounters.
func (b *Executor) waitForStage(ctx context.Context, name string, stages imagebuilder.Stages) (bool, error) {
	found := false
	for _, otherStage := range stages {
		if otherStage.Name == name || strconv.Itoa(otherStage.Position) == name {
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	for {
		if b.lastError != nil {
			return true, b.lastError
		}

		b.stagesLock.Lock()
		terminationError, terminated := b.terminatedStage[name]
		b.stagesLock.Unlock()

		if terminationError != nil {
			return false, terminationError
		}
		if terminated {
			return true, nil
		}

		b.stagesSemaphore.Release(1)
		time.Sleep(time.Millisecond * 10)
		if err := b.stagesSemaphore.Acquire(ctx, 1); err != nil {
			return true, fmt.Errorf("reacquiring job semaphore: %w", err)
		}
	}
}

// getImageTypeAndHistoryAndDiffIDs returns the manifest type, history, and diff IDs list of imageID.
func (b *Executor) getImageTypeAndHistoryAndDiffIDs(ctx context.Context, imageID string) (string, []v1.History, []digest.Digest, error) {
	b.imageInfoLock.Lock()
	imageInfo, ok := b.imageInfoCache[imageID]
	b.imageInfoLock.Unlock()
	if ok {
		return imageInfo.manifestType, imageInfo.history, imageInfo.diffIDs, imageInfo.err
	}
	imageRef, err := is.Transport.ParseStoreReference(b.store, "@"+imageID)
	if err != nil {
		return "", nil, nil, fmt.Errorf("getting image reference %q: %w", imageID, err)
	}
	ref, err := imageRef.NewImage(ctx, nil)
	if err != nil {
		return "", nil, nil, fmt.Errorf("creating new image from reference to image %q: %w", imageID, err)
	}
	defer ref.Close()
	oci, err := ref.OCIConfig(ctx)
	if err != nil {
		return "", nil, nil, fmt.Errorf("getting possibly-converted OCI config of image %q: %w", imageID, err)
	}
	manifestBytes, manifestFormat, err := ref.Manifest(ctx)
	if err != nil {
		return "", nil, nil, fmt.Errorf("getting manifest of image %q: %w", imageID, err)
	}
	if manifestFormat == "" && len(manifestBytes) > 0 {
		manifestFormat = manifest.GuessMIMEType(manifestBytes)
	}
	b.imageInfoLock.Lock()
	b.imageInfoCache[imageID] = imageTypeAndHistoryAndDiffIDs{
		manifestType: manifestFormat,
		history:      oci.History,
		diffIDs:      oci.RootFS.DiffIDs,
		err:          nil,
	}
	b.imageInfoLock.Unlock()
	return manifestFormat, oci.History, oci.RootFS.DiffIDs, nil
}

func (b *Executor) buildStage(ctx context.Context, cleanupStages map[int]*StageExecutor, stages imagebuilder.Stages, stageIndex int) (imageID string, ref reference.Canonical, err error) {
	stage := stages[stageIndex]
	ib := stage.Builder
	node := stage.Node
	base, err := ib.From(node)

	// If this is the last stage, then the image that we produce at
	// its end should be given the desired output name.
	output := ""
	if stageIndex == len(stages)-1 {
		output = b.output
	}

	if err != nil {
		logrus.Debugf("buildStage(node.Children=%#v)", node.Children)
		return "", nil, err
	}

	b.stagesLock.Lock()
	stageExecutor := b.startStage(ctx, &stage, stages, output)
	if stageExecutor.log == nil {
		stepCounter := 0
		stageExecutor.log = func(format string, args ...interface{}) {
			prefix := b.logPrefix
			if len(stages) > 1 {
				prefix += fmt.Sprintf("[%d/%d] ", stageIndex+1, len(stages))
			}
			if !strings.HasPrefix(format, "COMMIT") {
				stepCounter++
				prefix += fmt.Sprintf("STEP %d", stepCounter)
				if stepCounter <= len(stage.Node.Children)+1 {
					prefix += fmt.Sprintf("/%d", len(stage.Node.Children)+1)
				}
				prefix += ": "
			}
			suffix := "\n"
			fmt.Fprintf(stageExecutor.executor.out, prefix+format+suffix, args...)
		}
	}
	b.stagesLock.Unlock()

	// If this a single-layer build, or if it's a multi-layered
	// build and b.forceRmIntermediateCtrs is set, make sure we
	// remove the intermediate/build containers, regardless of
	// whether or not the stage's build fails.
	// Skip cleanup if the stage has no instructions.
	if b.forceRmIntermediateCtrs || !b.layers && len(stage.Node.Children) > 0 {
		b.stagesLock.Lock()
		cleanupStages[stage.Position] = stageExecutor
		b.stagesLock.Unlock()
	}

	// Build this stage.
	if imageID, ref, err = stageExecutor.Execute(ctx, base); err != nil {
		return "", nil, err
	}

	// The stage succeeded, so remove its build container if we're
	// told to delete successful intermediate/build containers for
	// multi-layered builds.
	// Skip cleanup if the stage has no instructions.
	if b.removeIntermediateCtrs && len(stage.Node.Children) > 0 {
		b.stagesLock.Lock()
		cleanupStages[stage.Position] = stageExecutor
		b.stagesLock.Unlock()
	}

	return imageID, ref, nil
}

type stageDependencyInfo struct {
	Name           string
	Position       int
	Needs          []string
	NeededByTarget bool
}

// Marks `NeededByTarget` as true for the given stage and all its dependency stages as true recursively.
func markDependencyStagesForTarget(dependencyMap map[string]*stageDependencyInfo, stage string) {
	if stageDependencyInfo, ok := dependencyMap[stage]; ok {
		if !stageDependencyInfo.NeededByTarget {
			stageDependencyInfo.NeededByTarget = true
			for _, need := range stageDependencyInfo.Needs {
				markDependencyStagesForTarget(dependencyMap, need)
			}
		}
	}
}

// Build takes care of the details of running Prepare/Execute/Commit/Delete
// over each of the one or more parsed Dockerfiles and stages.
func (b *Executor) Build(ctx context.Context, stages imagebuilder.Stages) (imageID string, ref reference.Canonical, err error) {
	if len(stages) == 0 {
		return "", nil, errors.New("building: no stages to build")
	}
	var cleanupImages []string
	cleanupStages := make(map[int]*StageExecutor)

	stdout := b.out
	if b.quiet {
		b.out = ioutil.Discard
	}

	cleanup := func() error {
		var lastErr error
		// Clean up any containers associated with the final container
		// built by a stage, for stages that succeeded, since we no
		// longer need their filesystem contents.

		b.stagesLock.Lock()
		for _, stage := range cleanupStages {
			if err := stage.Delete(); err != nil {
				logrus.Debugf("Failed to cleanup stage containers: %v", err)
				lastErr = err
			}
		}
		cleanupStages = nil
		b.stagesLock.Unlock()

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
				if b.forceRmIntermediateCtrs || !errors.Is(err, storage.ErrImageUsedByContainer) {
					lastErr = err
				}
			}
		}
		cleanupImages = nil

		if b.rusageLogFile != nil && b.rusageLogFile != b.out {
			// we deliberately ignore the error here, as this
			// function can be called multiple times
			if closer, ok := b.rusageLogFile.(interface{ Close() error }); ok {
				closer.Close()
			}
		}
		return lastErr
	}

	defer func() {
		if cleanupErr := cleanup(); cleanupErr != nil {
			if err == nil {
				err = cleanupErr
			} else {
				err = fmt.Errorf("%v: %w", cleanupErr.Error(), err)
			}
		}
	}()

	// dependencyMap contains dependencyInfo for each stage,
	// dependencyInfo is used later to mark if a particular
	// stage is needed by target or not.
	dependencyMap := make(map[string]*stageDependencyInfo)
	// Build maps of every named base image and every referenced stage root
	// filesystem.  Individual stages can use them to determine whether or
	// not they can skip certain steps near the end of their stages.
	for stageIndex, stage := range stages {
		dependencyMap[stage.Name] = &stageDependencyInfo{Name: stage.Name, Position: stage.Position}
		node := stage.Node // first line
		for node != nil {  // each line
			for _, child := range node.Children { // tokens on this line, though we only care about the first
				switch strings.ToUpper(child.Value) { // first token - instruction
				case "FROM":
					if child.Next != nil { // second token on this line
						// If we have a fromOverride, replace the value of
						// image name for the first FROM in the Containerfile.
						if b.fromOverride != "" {
							child.Next.Value = b.fromOverride
							b.fromOverride = ""
						}
						base := child.Next.Value
						if base != "scratch" {
							if replaceBuildContext, ok := b.additionalBuildContexts[child.Next.Value]; ok {
								if replaceBuildContext.IsImage {
									child.Next.Value = replaceBuildContext.Value
									base = child.Next.Value
								}
							}
							userArgs := argsMapToSlice(stage.Builder.Args)
							baseWithArg, err := imagebuilder.ProcessWord(base, userArgs)
							if err != nil {
								return "", nil, fmt.Errorf("while replacing arg variables with values for format %q: %w", base, err)
							}
							b.baseMap[baseWithArg] = true
							logrus.Debugf("base for stage %d: %q", stageIndex, base)
							// Check if selected base is not an additional
							// build context and if base is a valid stage
							// add it to current stage's dependency tree.
							if _, ok := b.additionalBuildContexts[baseWithArg]; !ok {
								if _, ok := dependencyMap[baseWithArg]; ok {
									// update current stage's dependency info
									currentStageInfo := dependencyMap[stage.Name]
									currentStageInfo.Needs = append(currentStageInfo.Needs, baseWithArg)
								}
							}
						}
					}
				case "ADD", "COPY":
					for _, flag := range child.Flags { // flags for this instruction
						if strings.HasPrefix(flag, "--from=") {
							// TODO: this didn't undergo variable and
							// arg expansion, so if the previous stage
							// was named using argument values, we might
							// not record the right value here.
							rootfs := strings.TrimPrefix(flag, "--from=")
							b.rootfsMap[rootfs] = true
							logrus.Debugf("rootfs needed for COPY in stage %d: %q", stageIndex, rootfs)
							// Populate dependency tree and check
							// if following ADD or COPY needs any other
							// stage.
							stageName := rootfs
							// If --from=<index> convert index to name
							if index, err := strconv.Atoi(stageName); err == nil {
								stageName = stages[index].Name
							}
							// Check if selected base is not an additional
							// build context and if base is a valid stage
							// add it to current stage's dependency tree.
							if _, ok := b.additionalBuildContexts[stageName]; !ok {
								if _, ok := dependencyMap[stageName]; ok {
									// update current stage's dependency info
									currentStageInfo := dependencyMap[stage.Name]
									currentStageInfo.Needs = append(currentStageInfo.Needs, stageName)
								}
							}
						}
					}
				case "RUN":
					for _, flag := range child.Flags { // flags for this instruction
						// We need to populate dependency tree of stages
						// if it is using `--mount` and `from=` field is set
						// and `from=` points to a stage consider it in
						// dependency calculation.
						if strings.HasPrefix(flag, "--mount=") && strings.Contains(flag, "from") {
							mountFlags := strings.TrimPrefix(flag, "--mount=")
							fields := strings.Split(mountFlags, ",")
							for _, field := range fields {
								if strings.HasPrefix(field, "from=") {
									fromField := strings.SplitN(field, "=", 2)
									if len(fromField) > 1 {
										mountFrom := fromField[1]
										// Check if this base is a stage if yes
										// add base to current stage's dependency tree
										// but also confirm if this is not in additional context.
										if _, ok := b.additionalBuildContexts[mountFrom]; !ok {
											if _, ok := dependencyMap[mountFrom]; ok {
												// update current stage's dependency info
												currentStageInfo := dependencyMap[stage.Name]
												currentStageInfo.Needs = append(currentStageInfo.Needs, mountFrom)
											}
										}
									} else {
										return "", nil, fmt.Errorf("invalid value for field `from=`: %q", fromField[1])
									}
								}
							}
						}
					}
				}
			}
			node = node.Next // next line
		}
		// Last stage is always target stage.
		// Since last/target stage is processed
		// let's calculate dependency map of stages
		// so we can mark stages which can be skipped.
		if stage.Position == (len(stages) - 1) {
			markDependencyStagesForTarget(dependencyMap, stage.Name)
		}
	}

	type Result struct {
		Index   int
		ImageID string
		Ref     reference.Canonical
		Error   error
	}

	ch := make(chan Result, len(stages))

	if b.stagesSemaphore == nil {
		b.stagesSemaphore = semaphore.NewWeighted(int64(len(stages)))
	}

	var wg sync.WaitGroup
	wg.Add(len(stages))

	go func() {
		cancel := false
		for stageIndex := range stages {
			index := stageIndex
			// Acquire the semaphore before creating the goroutine so we are sure they
			// run in the specified order.
			if err := b.stagesSemaphore.Acquire(ctx, 1); err != nil {
				cancel = true
				b.lastError = err
				ch <- Result{
					Index: index,
					Error: err,
				}
				wg.Done()
				continue
			}
			b.stagesLock.Lock()
			cleanupStages := cleanupStages
			b.stagesLock.Unlock()
			go func() {
				defer b.stagesSemaphore.Release(1)
				defer wg.Done()
				if cancel || cleanupStages == nil {
					var err error
					if stages[index].Name != strconv.Itoa(index) {
						err = fmt.Errorf("not building stage %d: build canceled", index)
					} else {
						err = fmt.Errorf("not building stage %d (%s): build canceled", index, stages[index].Name)
					}
					ch <- Result{
						Index: index,
						Error: err,
					}
					return
				}
				// Skip stage if it is not needed by TargetStage
				// or any of its dependency stages and `SkipUnusedStages`
				// is not set to `false`.
				if stageDependencyInfo, ok := dependencyMap[stages[index].Name]; ok {
					if !stageDependencyInfo.NeededByTarget && b.skipUnusedStages != types.OptionalBoolFalse {
						logrus.Debugf("Skipping stage with Name %q and index %d since its not needed by the target stage", stages[index].Name, index)
						ch <- Result{
							Index: index,
							Error: nil,
						}
						return
					}
				}
				stageID, stageRef, stageErr := b.buildStage(ctx, cleanupStages, stages, index)
				if stageErr != nil {
					cancel = true
					ch <- Result{
						Index: index,
						Error: stageErr,
					}
					return
				}

				ch <- Result{
					Index:   index,
					ImageID: stageID,
					Ref:     stageRef,
					Error:   nil,
				}
			}()
		}
	}()
	go func() {
		wg.Wait()
		close(ch)
	}()

	for r := range ch {
		stage := stages[r.Index]

		b.stagesLock.Lock()
		b.terminatedStage[stage.Name] = r.Error
		b.terminatedStage[strconv.Itoa(stage.Position)] = r.Error

		if r.Error != nil {
			b.stagesLock.Unlock()
			b.lastError = r.Error
			return "", nil, r.Error
		}

		// If this is an intermediate stage, make a note of the ID, so
		// that we can look it up later.
		if r.Index < len(stages)-1 && r.ImageID != "" {
			b.imageMap[stage.Name] = r.ImageID
			// We're not populating the cache with intermediate
			// images, so add this one to the list of images that
			// we'll remove later.
			if !b.layers {
				cleanupImages = append(cleanupImages, r.ImageID)
			}
		}
		if r.Index == len(stages)-1 {
			imageID = r.ImageID
			ref = r.Ref
		}
		b.stagesLock.Unlock()
	}

	if len(b.unusedArgs) > 0 {
		unusedList := make([]string, 0, len(b.unusedArgs))
		for k := range b.unusedArgs {
			unusedList = append(unusedList, k)
		}
		sort.Strings(unusedList)
		fmt.Fprintf(b.out, "[Warning] one or more build args were not consumed: %v\n", unusedList)
	}

	// Add additional tags and print image names recorded in storage
	if dest, err := b.resolveNameToImageRef(b.output); err == nil {
		switch dest.Transport().Name() {
		case is.Transport.Name():
			img, err := is.Transport.GetStoreImage(b.store, dest)
			if err != nil {
				return imageID, ref, fmt.Errorf("locating just-written image %q: %w", transports.ImageName(dest), err)
			}
			if len(b.additionalTags) > 0 {
				if err = util.AddImageNames(b.store, "", b.systemContext, img, b.additionalTags); err != nil {
					return imageID, ref, fmt.Errorf("setting image names to %v: %w", append(img.Names, b.additionalTags...), err)
				}
				logrus.Debugf("assigned names %v to image %q", img.Names, img.ID)
			}
			// Report back the caller the tags applied, if any.
			img, err = is.Transport.GetStoreImage(b.store, dest)
			if err != nil {
				return imageID, ref, fmt.Errorf("locating just-written image %q: %w", transports.ImageName(dest), err)
			}
			for _, name := range img.Names {
				fmt.Fprintf(b.out, "Successfully tagged %s\n", name)
			}

		default:
			if len(b.additionalTags) > 0 {
				b.logger.Warnf("don't know how to add tags to images stored in %q transport", dest.Transport().Name())
			}
		}
	}

	if err := cleanup(); err != nil {
		return "", nil, err
	}
	logrus.Debugf("printing final image id %q", imageID)
	if b.iidfile != "" {
		if err = ioutil.WriteFile(b.iidfile, []byte("sha256:"+imageID), 0644); err != nil {
			return imageID, ref, fmt.Errorf("failed to write image ID to file %q: %w", b.iidfile, err)
		}
	} else {
		if _, err := stdout.Write([]byte(imageID + "\n")); err != nil {
			return imageID, ref, fmt.Errorf("failed to write image ID to stdout: %w", err)
		}
	}
	return imageID, ref, nil
}

// deleteSuccessfulIntermediateCtrs goes through the container IDs in each
// stage's containerIDs list and deletes the containers associated with those
// IDs.
func (b *Executor) deleteSuccessfulIntermediateCtrs() error {
	var lastErr error
	for _, s := range b.stages {
		for _, ctr := range s.containerIDs {
			if err := b.store.DeleteContainer(ctr); err != nil {
				b.logger.Errorf("error deleting build container %q: %v\n", ctr, err)
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
