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
	"github.com/containers/image/tarball"
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
	// TransportNames are the supported transports in string form
	TransportNames = [...]string{DefaultTransport, DockerArchive, OCIArchive, "ostree:", "dir:"}
	// TarballTransport is the transport for importing a tar archive
	// and creating a filesystem image
	TarballTransport = tarball.Transport.Name()
	// DockerTransport is the transport for docker registries
	DockerTransport = docker.Transport.Name() + "://"
	// AtomicTransport is the transport for atomic registries
	AtomicTransport = "atomic"
	// DefaultTransport is a prefix that we apply to an image name
	DefaultTransport = DockerTransport
	// DefaultLocalRepo is the default local repository for local image operations
	// Remote pulls will still use defined registries
	DefaultLocalRepo = "localhost"
)

// pullRefPair records a pair of prepared image references to try to pull (if not DockerArchive) or to pull all (if DockerArchive)
type pullRefPair struct {
	image  string
	srcRef types.ImageReference
	dstRef types.ImageReference
}

// pullRefName records a prepared source reference and a destination name to try to pull (if not DockerArchive) or to pull all (if DockerArchive)
type pullRefName struct {
	image   string
	srcRef  types.ImageReference
	dstName string
}

func getPullRefName(srcRef types.ImageReference, destName string) *pullRefName {
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
	return &pullRefName{
		image:   destName,
		srcRef:  srcRef,
		dstName: reference,
	}
}

// refNamesFromImageReference returns a list of pullRefName for a single ImageReference, depending on the used transport.
func refNamesFromImageReference(ctx context.Context, srcRef types.ImageReference, imgName string, sc *types.SystemContext) ([]*pullRefName, error) {
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
			return []*pullRefName{getPullRefName(srcRef, reference)}, nil
		}

		var dest []string
		if len(manifest[0].RepoTags) > 0 {
			dest = append(dest, manifest[0].RepoTags...)
		} else {
			// If the input image has no repotags, we need to feed it a dest anyways
			digest, err := getImageDigest(ctx, srcRef, sc)
			if err != nil {
				return nil, err
			}
			dest = append(dest, digest)
		}
		// Need to load in all the repo tags from the manifest
		res := []*pullRefName{}
		for _, dst := range dest {
			pullInfo := getPullRefName(srcRef, dst)
			res = append(res, pullInfo)
		}
		return res, nil

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
		return []*pullRefName{getPullRefName(srcRef, dest)}, nil

	case DirTransport:
		path := srcRef.StringWithinTransport()
		image := path
		// remove leading "/"
		if image[:1] == "/" {
			// Instead of removing the leading /, set localhost as the registry
			// so docker.io isn't prepended, and the path becomes the repository
			image = DefaultLocalRepo + image
		}
		return []*pullRefName{getPullRefName(srcRef, image)}, nil

	default:
		return []*pullRefName{getPullRefName(srcRef, imgName)}, nil
	}
}

// refPairsFromImageReference returns a list of pullRefPair for a single ImageReference, depending on the used transport.
func (ir *Runtime) refPairsFromImageReference(ctx context.Context, srcRef types.ImageReference, imgName string, sc *types.SystemContext) ([]*pullRefPair, error) {
	refNames, err := refNamesFromImageReference(ctx, srcRef, imgName, sc)
	if err != nil {
		return nil, err
	}

	return ir.pullRefPairsFromRefNames(refNames)
}

// pullImage pulls an image from configured registries
// By default, only the latest tag (or a specific tag if requested) will be
// pulled.
func (i *Image) pullImage(ctx context.Context, writer io.Writer, authfile, signaturePolicyPath string, signingOptions SigningOptions, dockerOptions *DockerRegistryOptions, forceSecure bool) ([]string, error) {
	// pullImage copies the image from the source to the destination
	var pullRefPairs []*pullRefPair
	sc := GetSystemContext(signaturePolicyPath, authfile, false)
	srcRef, err := alltransports.ParseImageName(i.InputName)
	if err != nil {
		// could be trying to pull from registry with short name
		pullRefPairs, err = i.refPairsFromPossiblyUnqualifiedName()
		if err != nil {
			return nil, errors.Wrap(err, "error getting default registries to try")
		}
	} else {
		pullRefPairs, err = i.imageruntime.refPairsFromImageReference(ctx, srcRef, i.InputName, sc)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting pullRefPair info to pull image %q", i.InputName)
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
	for _, imageInfo := range pullRefPairs {
		copyOptions := getCopyOptions(writer, signaturePolicyPath, dockerOptions, nil, signingOptions, authfile, "", false, nil)
		if strings.HasPrefix(DockerTransport, imageInfo.srcRef.Transport().Name()) {
			imgRef, err := reference.Parse(imageInfo.srcRef.DockerReference().String())
			if err != nil {
				return nil, err
			}
			registry := reference.Domain(imgRef.(reference.Named))

			if util.StringInSlice(registry, insecureRegistries) && !forceSecure {
				copyOptions.SourceCtx.DockerInsecureSkipTLSVerify = true
				logrus.Info(fmt.Sprintf("%s is an insecure registry; pulling with tls-verify=false", registry))
			}
		}
		// Print the following statement only when pulling from a docker or atomic registry
		if writer != nil && (strings.HasPrefix(DockerTransport, imageInfo.srcRef.Transport().Name()) || imageInfo.srcRef.Transport().Name() == AtomicTransport) {
			io.WriteString(writer, fmt.Sprintf("Trying to pull %s...", imageInfo.image))
		}
		if err = cp.Image(ctx, policyContext, imageInfo.dstRef, imageInfo.srcRef, copyOptions); err != nil {
			if writer != nil {
				io.WriteString(writer, "Failed\n")
			}
		} else {
			if imageInfo.srcRef.Transport().Name() != DockerArchive {
				return []string{imageInfo.image}, nil
			}
			images = append(images, imageInfo.image)
		}
	}
	// If no image was found, we should handle.  Lets be nicer to the user and see if we can figure out why.
	if len(images) == 0 {
		registryPath := sysregistries.RegistriesConfPath(&types.SystemContext{})
		searchRegistries, err := registries.GetRegistries()
		if err != nil {
			return nil, err
		}
		hasRegistryInName, err := i.hasRegistry()
		if err != nil {
			return nil, err
		}
		if !hasRegistryInName && len(searchRegistries) == 0 {
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

// refNamesFromPossiblyUnqualifiedName looks at a decomposed image and determines the possible
// image names to try pulling in combination with the registries.conf file as well
func refNamesFromPossiblyUnqualifiedName(inputName string) ([]*pullRefName, error) {
	var (
		pullNames []*pullRefName
		imageName string
	)

	decomposedImage, err := decompose(inputName)
	if err != nil {
		return nil, err
	}
	if decomposedImage.hasRegistry {
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
		pullNames = append(pullNames, &ps)

	} else {
		searchRegistries, err := registries.GetRegistries()
		if err != nil {
			return nil, err
		}
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
			pullNames = append(pullNames, &ps)
		}
	}
	return pullNames, nil
}

// refPairsFromPossiblyUnqualifiedName looks at a decomposed image and determines the possible
// image references to try pulling in combination with the registries.conf file as well
func (i *Image) refPairsFromPossiblyUnqualifiedName() ([]*pullRefPair, error) {
	refNames, err := refNamesFromPossiblyUnqualifiedName(i.InputName)
	if err != nil {
		return nil, err
	}
	return i.imageruntime.pullRefPairsFromRefNames(refNames)
}

// pullRefPairsFromNames converts a []*pullRefName to []*pullRefPair
func (ir *Runtime) pullRefPairsFromRefNames(refNames []*pullRefName) ([]*pullRefPair, error) {
	// Here we construct the destination references
	res := make([]*pullRefPair, len(refNames))
	for i, rn := range refNames {
		destRef, err := is.Transport.ParseStoreReference(ir.store, rn.dstName)
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing dest reference name %#v", rn.dstName)
		}
		res[i] = &pullRefPair{
			image:  rn.image,
			srcRef: rn.srcRef,
			dstRef: destRef,
		}
	}
	return res, nil
}
