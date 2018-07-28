package image

import (
	"context"
	"fmt"
	"io"
	"strings"

	cp "github.com/containers/image/copy"
	"github.com/containers/image/directory"
	"github.com/containers/image/docker"
	dockerarchive "github.com/containers/image/docker/archive"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/docker/tarfile"
	ociarchive "github.com/containers/image/oci/archive"
	"github.com/containers/image/pkg/sysregistries"
	is "github.com/containers/image/storage"
	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/pkg/registries"
	"github.com/projectatomic/libpod/pkg/util"
	"github.com/sirupsen/logrus"
)

var (
	// DockerArchive is the transport we prepend to an image name
	// when saving to docker-archive
	DockerArchive = dockerarchive.Transport.Name()
	// OCIArchive is the transport we prepend to an image name
	// when saving to oci-archive
	OCIArchive = ociarchive.Transport.Name()
	// DirTransport is the transport for pushing and pulling
	// images to and from a directory
	DirTransport = directory.Transport.Name()
	// DockerTransport is the transport for docker registries
	DockerTransport = docker.Transport.Name()
	// AtomicTransport is the transport for atomic registries
	AtomicTransport = "atomic"
	// DefaultTransport is a prefix that we apply to an image name
	// NOTE: This is a string prefix, not actually a transport name usable for transports.Get();
	// and because syntaxes of image names are transport-dependent, the prefix is not really interchangeable;
	// each user implicitly assumes the appended string is a Docker-like reference.
	DefaultTransport = DockerTransport + "://"
	// DefaultLocalRepo is the default local repository for local image operations
	// Remote pulls will still use defined registries
	DefaultLocalRepo = "localhost"
)

// pullRefPair records a pair of prepared image references to pull.
type pullRefPair struct {
	image  string
	srcRef types.ImageReference
	dstRef types.ImageReference
}

// pullGoal represents the prepared image references and decided behavior to be executed by imagePull
type pullGoal struct {
	refPairs             []pullRefPair
	pullAllPairs         bool     // Pull all refPairs instead of stopping on first success.
	usedSearchRegistries bool     // refPairs construction has depended on registries.GetRegistries()
	searchedRegistries   []string // The list of search registries used; set only if usedSearchRegistries
}

// pullRefName records a prepared source reference and a destination name to pull.
type pullRefName struct {
	image   string
	srcRef  types.ImageReference
	dstName string
}

// pullGoalNames is an intermediate variant of pullGoal which uses pullRefName instead of pullRefPair.
type pullGoalNames struct {
	refNames             []pullRefName
	pullAllPairs         bool     // Pull all refNames instead of stopping on first success.
	usedSearchRegistries bool     // refPairs construction has depended on registries.GetRegistries()
	searchedRegistries   []string // The list of search registries used; set only if usedSearchRegistries
}

func singlePullRefNameGoal(rn pullRefName) *pullGoalNames {
	return &pullGoalNames{
		refNames:             []pullRefName{rn},
		pullAllPairs:         false, // Does not really make a difference.
		usedSearchRegistries: false,
		searchedRegistries:   nil,
	}
}

func getPullRefName(srcRef types.ImageReference, destName string) pullRefName {
	imgPart, err := decompose(destName)
	if err == nil && !imgPart.hasRegistry {
		// If the image doesn't have a registry, set it as the default repo
		imgPart.registry = DefaultLocalRepo
		imgPart.hasRegistry = true
		destName = imgPart.assemble()
	}

	reference := destName
	if srcRef.DockerReference() != nil {
		reference = srcRef.DockerReference().String()
	}
	return pullRefName{
		image:   destName,
		srcRef:  srcRef,
		dstName: reference,
	}
}

// pullGoalNamesFromImageReference returns a pullGoalNames for a single ImageReference, depending on the used transport.
func pullGoalNamesFromImageReference(ctx context.Context, srcRef types.ImageReference, imgName string, sc *types.SystemContext) (*pullGoalNames, error) {
	// supports pulling from docker-archive, oci, and registries
	switch srcRef.Transport().Name() {
	case DockerArchive:
		archivePath := srcRef.StringWithinTransport()
		tarSource, err := tarfile.NewSourceFromFile(archivePath)
		if err != nil {
			return nil, err
		}
		manifest, err := tarSource.LoadTarManifest()

		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving manifest.json")
		}
		// to pull the first image stored in the tar file
		if len(manifest) == 0 {
			// use the hex of the digest if no manifest is found
			reference, err := getImageDigest(ctx, srcRef, sc)
			if err != nil {
				return nil, err
			}
			return singlePullRefNameGoal(getPullRefName(srcRef, reference)), nil
		}

		if len(manifest[0].RepoTags) == 0 {
			// If the input image has no repotags, we need to feed it a dest anyways
			digest, err := getImageDigest(ctx, srcRef, sc)
			if err != nil {
				return nil, err
			}
			return singlePullRefNameGoal(getPullRefName(srcRef, digest)), nil
		}

		// Need to load in all the repo tags from the manifest
		res := []pullRefName{}
		for _, dst := range manifest[0].RepoTags {
			pullInfo := getPullRefName(srcRef, dst)
			res = append(res, pullInfo)
		}
		return &pullGoalNames{
			refNames:             res,
			pullAllPairs:         true,
			usedSearchRegistries: false,
			searchedRegistries:   nil,
		}, nil

	case OCIArchive:
		// retrieve the manifest from index.json to access the image name
		manifest, err := ociarchive.LoadManifestDescriptor(srcRef)
		if err != nil {
			return nil, errors.Wrapf(err, "error loading manifest for %q", srcRef)
		}

		var dest string
		if manifest.Annotations == nil || manifest.Annotations["org.opencontainers.image.ref.name"] == "" {
			// If the input image has no image.ref.name, we need to feed it a dest anyways
			// use the hex of the digest
			dest, err = getImageDigest(ctx, srcRef, sc)
			if err != nil {
				return nil, errors.Wrapf(err, "error getting image digest; image reference not found")
			}
		} else {
			dest = manifest.Annotations["org.opencontainers.image.ref.name"]
		}
		return singlePullRefNameGoal(getPullRefName(srcRef, dest)), nil

	case DirTransport:
		path := srcRef.StringWithinTransport()
		image := path
		// remove leading "/"
		if image[:1] == "/" {
			// Instead of removing the leading /, set localhost as the registry
			// so docker.io isn't prepended, and the path becomes the repository
			image = DefaultLocalRepo + image
		}
		return singlePullRefNameGoal(getPullRefName(srcRef, image)), nil

	default:
		return singlePullRefNameGoal(getPullRefName(srcRef, imgName)), nil
	}
}

// pullGoalFromImageReference returns a pull goal for a single ImageReference, depending on the used transport.
func (ir *Runtime) pullGoalFromImageReference(ctx context.Context, srcRef types.ImageReference, imgName string, sc *types.SystemContext) (pullGoal, error) {
	goalNames, err := pullGoalNamesFromImageReference(ctx, srcRef, imgName, sc)
	if err != nil {
		return pullGoal{}, err
	}

	return ir.pullGoalFromGoalNames(goalNames)
}

// pullImage pulls an image from configured registries based on inputName.
// By default, only the latest tag (or a specific tag if requested) will be
// pulled.
func (ir *Runtime) pullImage(ctx context.Context, inputName string, writer io.Writer, authfile, signaturePolicyPath string, signingOptions SigningOptions, dockerOptions *DockerRegistryOptions, forceSecure bool) ([]string, error) {
	// pullImage copies the image from the source to the destination
	var goal pullGoal
	sc := GetSystemContext(signaturePolicyPath, authfile, false)
	srcRef, err := alltransports.ParseImageName(inputName)
	if err != nil {
		// could be trying to pull from registry with short name
		goal, err = ir.pullGoalFromPossiblyUnqualifiedName(inputName)
		if err != nil {
			return nil, errors.Wrap(err, "error getting default registries to try")
		}
	} else {
		goal, err = ir.pullGoalFromImageReference(ctx, srcRef, inputName, sc)
		if err != nil {
			return nil, errors.Wrapf(err, "error determining pull goal for image %q", inputName)
		}
	}
	policyContext, err := getPolicyContext(sc)
	if err != nil {
		return nil, err
	}
	defer policyContext.Destroy()

	insecureRegistries, err := registries.GetInsecureRegistries()
	if err != nil {
		return nil, err
	}
	var images []string
	for _, imageInfo := range goal.refPairs {
		copyOptions := getCopyOptions(sc, writer, dockerOptions, nil, signingOptions, "", false, nil)
		if imageInfo.srcRef.Transport().Name() == DockerTransport {
			imgRef := imageInfo.srcRef.DockerReference()
			if imgRef == nil { // This should never happen; such references can’t be created.
				return nil, fmt.Errorf("internal error: DockerTransport reference %s does not have a DockerReference",
					transports.ImageName(imageInfo.srcRef))
			}
			registry := reference.Domain(imgRef)

			if util.StringInSlice(registry, insecureRegistries) && !forceSecure {
				copyOptions.SourceCtx.DockerInsecureSkipTLSVerify = true
				logrus.Info(fmt.Sprintf("%s is an insecure registry; pulling with tls-verify=false", registry))
			}
		}
		// Print the following statement only when pulling from a docker or atomic registry
		if writer != nil && (imageInfo.srcRef.Transport().Name() == DockerTransport || imageInfo.srcRef.Transport().Name() == AtomicTransport) {
			io.WriteString(writer, fmt.Sprintf("Trying to pull %s...", imageInfo.image))
		}
		if err = cp.Image(ctx, policyContext, imageInfo.dstRef, imageInfo.srcRef, copyOptions); err != nil {
			if writer != nil {
				io.WriteString(writer, "Failed\n")
			}
		} else {
			if !goal.pullAllPairs {
				return []string{imageInfo.image}, nil
			}
			images = append(images, imageInfo.image)
		}
	}
	// If no image was found, we should handle.  Lets be nicer to the user and see if we can figure out why.
	if len(images) == 0 {
		registryPath := sysregistries.RegistriesConfPath(&types.SystemContext{})
		if goal.usedSearchRegistries && len(goal.searchedRegistries) == 0 {
			return nil, errors.Errorf("image name provided is a short name and no search registries are defined in %s.", registryPath)
		}
		return nil, errors.Errorf("unable to find image in the registries defined in %q", registryPath)
	}
	return images, nil
}

// hasShaInInputName returns a bool as to whether the user provided an image name that includes
// a reference to a specific sha
func hasShaInInputName(inputName string) bool {
	return strings.Contains(inputName, "@sha256:")
}

// pullGoalNamesFromPossiblyUnqualifiedName looks at a decomposed image and determines the possible
// image names to try pulling in combination with the registries.conf file as well
func pullGoalNamesFromPossiblyUnqualifiedName(inputName string) (*pullGoalNames, error) {
	decomposedImage, err := decompose(inputName)
	if err != nil {
		return nil, err
	}
	if decomposedImage.hasRegistry {
		var imageName string
		if hasShaInInputName(inputName) {
			imageName = fmt.Sprintf("%s%s", decomposedImage.transport, inputName)
		} else {
			imageName = decomposedImage.assembleWithTransport()
		}
		srcRef, err := alltransports.ParseImageName(imageName)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to parse '%s'", inputName)
		}
		ps := pullRefName{
			image:  inputName,
			srcRef: srcRef,
		}
		if hasShaInInputName(inputName) {
			ps.dstName = decomposedImage.assemble()
		} else {
			ps.dstName = ps.image
		}
		return singlePullRefNameGoal(ps), nil
	}

	searchRegistries, err := registries.GetRegistries()
	if err != nil {
		return nil, err
	}
	var pullNames []pullRefName
	for _, registry := range searchRegistries {
		decomposedImage.registry = registry
		imageName := decomposedImage.assembleWithTransport()
		if hasShaInInputName(inputName) {
			imageName = fmt.Sprintf("%s%s/%s", decomposedImage.transport, registry, inputName)
		}
		srcRef, err := alltransports.ParseImageName(imageName)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to parse '%s'", inputName)
		}
		ps := pullRefName{
			image:  decomposedImage.assemble(),
			srcRef: srcRef,
		}
		ps.dstName = ps.image
		pullNames = append(pullNames, ps)
	}
	return &pullGoalNames{
		refNames:             pullNames,
		pullAllPairs:         false,
		usedSearchRegistries: true,
		searchedRegistries:   searchRegistries,
	}, nil
}

// pullGoalFromPossiblyUnqualifiedName looks at inputName and determines the possible
// image references to try pulling in combination with the registries.conf file as well
func (ir *Runtime) pullGoalFromPossiblyUnqualifiedName(inputName string) (pullGoal, error) {
	goalNames, err := pullGoalNamesFromPossiblyUnqualifiedName(inputName)
	if err != nil {
		return pullGoal{}, err
	}
	return ir.pullGoalFromGoalNames(goalNames)
}

// pullGoalFromGoalNames converts a pullGoalNames to a pullGoal
func (ir *Runtime) pullGoalFromGoalNames(goalNames *pullGoalNames) (pullGoal, error) {
	if goalNames == nil { // The value is a pointer only to make (return nil, err) possible in callers; they should never return nil on success
		return pullGoal{}, errors.New("internal error: pullGoalFromGoalNames(nil)")
	}
	// Here we construct the destination references
	res := make([]pullRefPair, len(goalNames.refNames))
	for i, rn := range goalNames.refNames {
		destRef, err := is.Transport.ParseStoreReference(ir.store, rn.dstName)
		if err != nil {
			return pullGoal{}, errors.Wrapf(err, "error parsing dest reference name %#v", rn.dstName)
		}
		res[i] = pullRefPair{
			image:  rn.image,
			srcRef: rn.srcRef,
			dstRef: destRef,
		}
	}
	return pullGoal{
		refPairs:             res,
		pullAllPairs:         goalNames.pullAllPairs,
		usedSearchRegistries: goalNames.usedSearchRegistries,
		searchedRegistries:   goalNames.searchedRegistries,
	}, nil
}
