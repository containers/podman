package utils

import (
	"fmt"
	"net/http"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/filters"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v3/libpod"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

// IsRegistryReference checks if the specified name points to the "docker://"
// transport.  If it points to no supported transport, we'll assume a
// non-transport reference pointing to an image (e.g., "fedora:latest").
func IsRegistryReference(name string) error {
	imageRef, err := alltransports.ParseImageName(name)
	if err != nil {
		// No supported transport -> assume a docker-stype reference.
		return nil
	}
	if imageRef.Transport().Name() == docker.Transport.Name() {
		return nil
	}
	return errors.Errorf("unsupport transport %s in %q: only docker transport is supported", imageRef.Transport().Name(), name)
}

// ParseStorageReference parses the specified image name to a
// `types.ImageReference` and enforces it to refer to a
// containers-storage-transport reference.
func ParseStorageReference(name string) (types.ImageReference, error) {
	storagePrefix := fmt.Sprintf("%s:", storage.Transport.Name())
	imageRef, err := alltransports.ParseImageName(name)
	if err == nil && imageRef.Transport().Name() != docker.Transport.Name() {
		return nil, errors.Errorf("reference %q must be a storage reference", name)
	} else if err != nil {
		origErr := err
		imageRef, err = alltransports.ParseImageName(fmt.Sprintf("%s%s", storagePrefix, name))
		if err != nil {
			return nil, errors.Wrapf(origErr, "reference %q must be a storage reference", name)
		}
	}
	return imageRef, nil
}

// GetImages is a common function used to get images for libpod and other compatibility
// mechanisms
func GetImages(w http.ResponseWriter, r *http.Request) ([]*libimage.Image, error) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	query := struct {
		All     bool
		Digests bool
		Filter  string // Docker 1.24 compatibility
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		return nil, err
	}
	if _, found := r.URL.Query()["digests"]; found && query.Digests {
		UnSupportedParameter("digests")
	}

	filterList, err := filters.FiltersFromRequest(r)
	if err != nil {
		return nil, err
	}
	if !IsLibpodRequest(r) && len(query.Filter) > 0 { // Docker 1.24 compatibility
		filterList = append(filterList, "reference="+query.Filter)
	}

	if !query.All {
		// Filter intermediate images unless we want to list *all*.
		// NOTE: it's a positive filter, so `intermediate=false` means
		// to display non-intermediate images.
		filterList = append(filterList, "intermediate=false")
	}
	listOptions := &libimage.ListImagesOptions{Filters: filterList}
	return runtime.LibimageRuntime().ListImages(r.Context(), nil, listOptions)
}

func GetImage(r *http.Request, name string) (*libimage.Image, error) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	lookupOptions := &libimage.LookupImageOptions{IgnorePlatform: true}
	image, _, err := runtime.LibimageRuntime().LookupImage(name, lookupOptions)
	if err != nil {
		return nil, err
	}
	return image, err
}
