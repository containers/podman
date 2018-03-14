package image

import (
	"fmt"
	"io"
	"os"
	"strings"

	cp "github.com/containers/image/copy"
	"github.com/containers/image/directory"
	"github.com/containers/image/docker"
	dockerarchive "github.com/containers/image/docker/archive"
	"github.com/containers/image/docker/tarfile"
	ociarchive "github.com/containers/image/oci/archive"
	"github.com/containers/image/pkg/sysregistries"
	is "github.com/containers/image/storage"
	"github.com/containers/image/tarball"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
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
)

type pullStruct struct {
	image  string
	srcRef types.ImageReference
	dstRef types.ImageReference
}

func (ir *Runtime) getPullStruct(srcRef types.ImageReference, destName string) (*pullStruct, error) {
	reference := destName
	if srcRef.DockerReference() != nil {
		reference = srcRef.DockerReference().String()
	}
	destRef, err := is.Transport.ParseStoreReference(ir.store, reference)
	if err != nil {
		return nil, errors.Errorf("error parsing dest reference name: %v", err)
	}
	return &pullStruct{
		image:  destName,
		srcRef: srcRef,
		dstRef: destRef,
	}, nil
}

// returns a list of pullStruct with the srcRef and DstRef based on the transport being used
func (ir *Runtime) getPullListFromRef(srcRef types.ImageReference, imgName string, sc *types.SystemContext) ([]*pullStruct, error) {
	var pullStructs []*pullStruct
	splitArr := strings.Split(imgName, ":")
	archFile := splitArr[len(splitArr)-1]

	// supports pulling from docker-archive, oci, and registries
	if srcRef.Transport().Name() == DockerArchive {
		tarSource, err := tarfile.NewSourceFromFile(archFile)
		if err != nil {
			return nil, err
		}
		manifest, err := tarSource.LoadTarManifest()

		if err != nil {
			return nil, errors.Errorf("error retrieving manifest.json: %v", err)
		}
		// to pull the first image stored in the tar file
		if len(manifest) == 0 {
			// use the hex of the digest if no manifest is found
			reference, err := getImageDigest(srcRef, sc)
			if err != nil {
				return nil, err
			}
			pullInfo, err := ir.getPullStruct(srcRef, reference)
			if err != nil {
				return nil, err
			}
			pullStructs = append(pullStructs, pullInfo)
		} else {
			var dest string
			if len(manifest[0].RepoTags) > 0 {
				dest = manifest[0].RepoTags[0]
			} else {
				// If the input image has no repotags, we need to feed it a dest anyways
				dest, err = getImageDigest(srcRef, sc)
				if err != nil {
					return nil, err
				}
			}
			pullInfo, err := ir.getPullStruct(srcRef, dest)
			if err != nil {
				return nil, err
			}
			pullStructs = append(pullStructs, pullInfo)
		}
	} else if srcRef.Transport().Name() == OCIArchive {
		// retrieve the manifest from index.json to access the image name
		manifest, err := ociarchive.LoadManifestDescriptor(srcRef)
		if err != nil {
			return nil, errors.Wrapf(err, "error loading manifest for %q", srcRef)
		}

		if manifest.Annotations == nil || manifest.Annotations["org.opencontainers.image.ref.name"] == "" {
			return nil, errors.Errorf("error, archive doesn't have a name annotation. Cannot store image with no name")
		}
		pullInfo, err := ir.getPullStruct(srcRef, manifest.Annotations["org.opencontainers.image.ref.name"])
		if err != nil {
			return nil, err
		}
		pullStructs = append(pullStructs, pullInfo)
	} else if srcRef.Transport().Name() == DirTransport {
		// supports pull from a directory
		image := splitArr[1]
		// remove leading "/"
		if image[:1] == "/" {
			image = image[1:]
		}
		pullInfo, err := ir.getPullStruct(srcRef, image)
		if err != nil {
			return nil, err
		}
		pullStructs = append(pullStructs, pullInfo)
	} else {
		pullInfo, err := ir.getPullStruct(srcRef, imgName)
		if err != nil {
			return nil, err
		}
		pullStructs = append(pullStructs, pullInfo)
	}
	return pullStructs, nil
}

// pullImage pulls an image from configured registries
// By default, only the latest tag (or a specific tag if requested) will be
// pulled.
func (i *Image) pullImage(writer io.Writer, authfile, signaturePolicyPath string, signingOptions SigningOptions, dockerOptions *DockerRegistryOptions) (string, error) {
	// pullImage copies the image from the source to the destination
	var pullStructs []*pullStruct
	sc := GetSystemContext(signaturePolicyPath, authfile, false)
	srcRef, err := alltransports.ParseImageName(i.InputName)
	if err != nil {
		// could be trying to pull from registry with short name
		pullStructs, err = i.createNamesToPull()
		if err != nil {
			return "", errors.Wrap(err, "error getting default registries to try")
		}
	} else {
		pullStructs, err = i.imageruntime.getPullListFromRef(srcRef, i.InputName, sc)
		if err != nil {
			return "", errors.Wrapf(err, "error getting pullStruct info to pull image %q", i.InputName)
		}
	}
	policyContext, err := getPolicyContext(sc)
	if err != nil {
		return "", err
	}
	defer policyContext.Destroy()

	copyOptions := getCopyOptions(writer, signaturePolicyPath, dockerOptions, nil, signingOptions, authfile, "", false)
	for _, imageInfo := range pullStructs {
		// Print the following statement only when pulling from a docker or atomic registry
		if writer != nil && (imageInfo.srcRef.Transport().Name() == DockerTransport || imageInfo.srcRef.Transport().Name() == AtomicTransport) {
			io.WriteString(writer, fmt.Sprintf("Trying to pull %s...\n", imageInfo.image))
		}
		if err = cp.Image(policyContext, imageInfo.dstRef, imageInfo.srcRef, copyOptions); err != nil {
			if writer != nil {
				io.WriteString(writer, "Failed\n")
			}
		} else {
			return imageInfo.image, nil
		}
	}
	return "", errors.Wrapf(err, "error pulling image from")
}

// createNamesToPull looks at a decomposed image and determines the possible
// images names to try pulling in combination with the registries.conf file as well
func (i *Image) createNamesToPull() ([]*pullStruct, error) {
	var pullNames []*pullStruct
	decomposedImage, err := decompose(i.InputName)
	if err != nil {
		return nil, err
	}
	if decomposedImage.hasRegistry {
		srcRef, err := alltransports.ParseImageName(decomposedImage.assembleWithTransport())
		if err != nil {
			return nil, errors.Errorf("unable to parse '%s'", i.InputName)
		}
		ps := pullStruct{
			image:  i.InputName,
			srcRef: srcRef,
		}
		pullNames = append(pullNames, &ps)

	} else {
		registryConfigPath := ""
		envOverride := os.Getenv("REGISTRIES_CONFIG_PATH")
		if len(envOverride) > 0 {
			registryConfigPath = envOverride
		}
		searchRegistries, err := sysregistries.GetRegistries(&types.SystemContext{SystemRegistriesConfPath: registryConfigPath})
		if err != nil {
			return nil, err
		}
		for _, registry := range searchRegistries {
			decomposedImage.registry = registry
			srcRef, err := alltransports.ParseImageName(decomposedImage.assembleWithTransport())
			if err != nil {
				return nil, errors.Errorf("unable to parse '%s'", i.InputName)
			}
			ps := pullStruct{
				image:  decomposedImage.assemble(),
				srcRef: srcRef,
			}
			pullNames = append(pullNames, &ps)
		}
	}

	for _, pStruct := range pullNames {
		destRef, err := is.Transport.ParseStoreReference(i.imageruntime.store, pStruct.image)
		if err != nil {
			return nil, errors.Errorf("error parsing dest reference name: %v", err)
		}
		pStruct.dstRef = destRef
	}

	return pullNames, nil
}
