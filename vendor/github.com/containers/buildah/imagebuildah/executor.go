package imagebuildah

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/buildah/util"
	"github.com/containers/image/docker/reference"
	is "github.com/containers/image/storage"
	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerfile/parser"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	layers                         bool
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

// historyMatches returns true if a candidate history matches the history of our
// base image (if we have one), plus the current instruction.
// Used to verify whether a cache of the intermediate image exists and whether
// to run the build again.
func (b *Executor) historyMatches(baseHistory []v1.History, child *parser.Node, history []v1.History) bool {
	if len(baseHistory) >= len(history) {
		return false
	}
	if len(history)-len(baseHistory) != 1 {
		return false
	}
	for i := range baseHistory {
		if baseHistory[i].CreatedBy != history[i].CreatedBy {
			return false
		}
		if baseHistory[i].Comment != history[i].Comment {
			return false
		}
		if baseHistory[i].Author != history[i].Author {
			return false
		}
		if baseHistory[i].EmptyLayer != history[i].EmptyLayer {
			return false
		}
		if baseHistory[i].Created != nil && history[i].Created == nil {
			return false
		}
		if baseHistory[i].Created == nil && history[i].Created != nil {
			return false
		}
		if baseHistory[i].Created != nil && history[i].Created != nil && *baseHistory[i].Created != *history[i].Created {
			return false
		}
	}
	return history[len(baseHistory)].CreatedBy == b.getCreatedBy(child)
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
	sort.Strings(buildArgs)
	return strings.Join(buildArgs, " ")
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

	defer func() {
		if cleanupErr := cleanup(); cleanupErr != nil {
			if err == nil {
				err = cleanupErr
			} else {
				err = errors.Wrap(err, cleanupErr.Error())
			}
		}
	}()

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

	if len(b.additionalTags) > 0 {
		if dest, err := b.resolveNameToImageRef(b.output); err == nil {
			switch dest.Transport().Name() {
			case is.Transport.Name():
				img, err := is.Transport.GetStoreImage(b.store, dest)
				if err != nil {
					return imageID, ref, errors.Wrapf(err, "error locating just-written image %q", transports.ImageName(dest))
				}
				if err = util.AddImageNames(b.store, "", b.systemContext, img, b.additionalTags); err != nil {
					return imageID, ref, errors.Wrapf(err, "error setting image names to %v", append(img.Names, b.additionalTags...))
				}
				logrus.Debugf("assigned names %v to image %q", img.Names, img.ID)
			default:
				logrus.Warnf("don't know how to add tags to images stored in %q transport", dest.Transport().Name())
			}
		}
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
