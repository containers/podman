package utils

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	storageTransport "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage"
)

// NormalizeToDockerHub normalizes the specified nameOrID to Docker Hub if the
// request is for the compat API and if containers.conf set the specific mode.
// If nameOrID is a (short) ID for a local image, the full ID will be returned.
func NormalizeToDockerHub(r *http.Request, nameOrID string) (string, error) {
	if IsLibpodRequest(r) || !util.DefaultContainerConfig().Engine.CompatAPIEnforceDockerHub {
		return nameOrID, nil
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	// The candidate may resolve to a local non-Docker Hub image, such as
	// 'busybox' -> 'registry.com/busybox'.
	img, candidate, err := runtime.LibimageRuntime().LookupImage(nameOrID, nil)
	if err != nil {
		if !errors.Is(err, storage.ErrImageUnknown) {
			return "", fmt.Errorf("normalizing name for compat API: %v", err)
		}
		// If the image could not be resolved locally, set the
		// candidate back to the input.
		candidate = nameOrID
	} else if strings.HasPrefix(img.ID(), strings.TrimPrefix(nameOrID, "sha256:")) {
		return img.ID(), nil
	}

	// No ID, so we can normalize.
	named, err := reference.ParseNormalizedNamed(candidate)
	if err != nil {
		return "", fmt.Errorf("normalizing name %q (orig: %q) for compat API: %v", candidate, nameOrID, err)
	}

	return named.String(), nil
}

// PossiblyEnforceDockerHub sets fields in the system context to enforce
// resolving short names to Docker Hub if the request is for the compat API and
// if containers.conf set the specific mode.
func PossiblyEnforceDockerHub(r *http.Request, sys *types.SystemContext) {
	if IsLibpodRequest(r) || !util.DefaultContainerConfig().Engine.CompatAPIEnforceDockerHub {
		return
	}
	sys.PodmanOnlyShortNamesIgnoreRegistriesConfAndForceDockerHub = true
}

// IsRegistryReference checks if the specified name points to the "docker://"
// transport.  If it points to no supported transport, we'll assume a
// non-transport reference pointing to an image (e.g., "fedora:latest").
func IsRegistryReference(name string) error {
	imageRef, err := alltransports.ParseImageName(name)
	if err != nil {
		// No supported transport -> assume a docker-stype reference.
		return nil //nolint: nilerr
	}
	if imageRef.Transport().Name() == docker.Transport.Name() {
		return nil
	}
	return fmt.Errorf("unsupported transport %s in %q: only docker transport is supported", imageRef.Transport().Name(), name)
}

// ParseStorageReference parses the specified image name to a
// `types.ImageReference` and enforces it to refer to a
// containers-storage-transport reference.
func ParseStorageReference(name string) (types.ImageReference, error) {
	storagePrefix := storageTransport.Transport.Name()
	imageRef, err := alltransports.ParseImageName(name)
	if err == nil && imageRef.Transport().Name() != docker.Transport.Name() {
		return nil, fmt.Errorf("reference %q must be a storage reference", name)
	} else if err != nil {
		origErr := err
		imageRef, err = alltransports.ParseImageName(fmt.Sprintf("%s:%s", storagePrefix, name))
		if err != nil {
			return nil, fmt.Errorf("reference %q must be a storage reference: %w", name, origErr)
		}
	}
	return imageRef, nil
}

func GetImage(r *http.Request, name string) (*libimage.Image, error) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	image, _, err := runtime.LibimageRuntime().LookupImage(name, nil)
	if err != nil {
		return nil, err
	}
	return image, err
}
