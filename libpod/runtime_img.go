package libpod

import (
	"fmt"
	"io"
	"os"

	"github.com/containers/image/directory"
	"github.com/containers/image/docker"
	dockerarchive "github.com/containers/image/docker/archive"
	ociarchive "github.com/containers/image/oci/archive"
	"github.com/containers/image/pkg/sysregistries"
	"github.com/containers/image/tarball"
	"github.com/containers/image/types"
	"github.com/containers/storage/pkg/archive"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod/common"
	"github.com/projectatomic/libpod/libpod/image"
)

// Runtime API

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
	// Docker is the transport for docker registries
	Docker = docker.Transport.Name()
	// Atomic is the transport for atomic registries
	Atomic = "atomic"
)

// CopyOptions contains the options given when pushing or pulling images
type CopyOptions struct {
	// Compression specifies the type of compression which is applied to
	// layer blobs.  The default is to not use compression, but
	// archive.Gzip is recommended.
	Compression archive.Compression
	// DockerRegistryOptions encapsulates settings that affect how we
	// connect or authenticate to a remote registry to which we want to
	// push the image.
	common.DockerRegistryOptions
	// SigningOptions encapsulates settings that control whether or not we
	// strip or add signatures to the image when pushing (uploading) the
	// image to a registry.
	common.SigningOptions

	// SigningPolicyPath this points to a alternative signature policy file, used mainly for testing
	SignaturePolicyPath string
	// AuthFile is the path of the cached credentials file defined by the user
	AuthFile string
	// Writer is the reportWriter for the output
	Writer io.Writer
	// Reference is the name for the image created when a tar archive is imported
	Reference string
	// ImageConfig is the Image spec for the image created when a tar archive is imported
	ImageConfig ociv1.Image
	// ManifestMIMEType is the manifest type of the image when saving to a directory
	ManifestMIMEType string
	// ForceCompress compresses the image layers when saving to a directory using the dir transport if true
	ForceCompress bool
}

// RemoveImage deletes an image from local storage
// Images being used by running containers can only be removed if force=true
func (r *Runtime) RemoveImage(image *image.Image, force bool) (string, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return "", ErrRuntimeStopped
	}

	// Get all containers, filter to only those using the image, and remove those containers
	ctrs, err := r.state.AllContainers()
	if err != nil {
		return "", err
	}
	imageCtrs := []*Container{}
	for _, ctr := range ctrs {
		if ctr.config.RootfsImageID == image.ID() {
			imageCtrs = append(imageCtrs, ctr)
		}
	}
	if len(imageCtrs) > 0 && len(image.Names()) <= 1 {
		if force {
			for _, ctr := range imageCtrs {
				if err := r.removeContainer(ctr, true); err != nil {
					return "", errors.Wrapf(err, "error removing image %s: container %s using image could not be removed", image.ID, ctr.ID())
				}
			}
		} else {
			return "", fmt.Errorf("could not remove image %s as it is being used by %d containers", image.ID, len(imageCtrs))
		}
	}

	if len(image.Names()) > 1 && !image.InputIsID() {
		// If the image has multiple reponames, we do not technically delete
		// the image. we figure out which repotag the user is trying to refer
		// to and untag it.
		repoName, err := image.MatchRepoTag(image.InputName)
		if err != nil {
			return "", err
		}
		if err := image.UntagImage(repoName); err != nil {
			return "", err
		}
		return fmt.Sprintf("Untagged: %s", repoName), nil
	} else if len(image.Names()) > 1 && image.InputIsID() && !force {
		// If the user requests to delete an image by ID and the image has multiple
		// reponames and no force is applied, we error out.
		return "", fmt.Errorf("unable to delete %s (must force) - image is referred to in multiple tags", image.ID())
	}

	return image.ID(), image.Remove(force)
}

// GetRegistries gets the searchable registries from the global registration file.
func GetRegistries() ([]string, error) {
	registryConfigPath := ""
	envOverride := os.Getenv("REGISTRIES_CONFIG_PATH")
	if len(envOverride) > 0 {
		registryConfigPath = envOverride
	}
	searchRegistries, err := sysregistries.GetRegistries(&types.SystemContext{SystemRegistriesConfPath: registryConfigPath})
	if err != nil {
		return nil, errors.Errorf("unable to parse the registries.conf file")
	}
	return searchRegistries, nil
}

// GetInsecureRegistries obtains the list of inseure registries from the global registration file.
func GetInsecureRegistries() ([]string, error) {
	registryConfigPath := ""
	envOverride := os.Getenv("REGISTRIES_CONFIG_PATH")
	if len(envOverride) > 0 {
		registryConfigPath = envOverride
	}
	registries, err := sysregistries.GetInsecureRegistries(&types.SystemContext{SystemRegistriesConfPath: registryConfigPath})
	if err != nil {
		return nil, errors.Errorf("unable to parse the registries.conf file")
	}
	return registries, nil
}
