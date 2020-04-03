package utils

import (
	"fmt"
	"net/http"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
)

// ParseDockerReference parses the specified image name to a
// `types.ImageReference` and enforces it to refer to a docker-transport
// reference.
func ParseDockerReference(name string) (types.ImageReference, error) {
	dockerPrefix := fmt.Sprintf("%s://", docker.Transport.Name())
	imageRef, err := alltransports.ParseImageName(name)
	if err == nil && imageRef.Transport().Name() != docker.Transport.Name() {
		return nil, errors.Errorf("reference %q must be a docker reference", name)
	} else if err != nil {
		origErr := err
		imageRef, err = alltransports.ParseImageName(fmt.Sprintf("%s%s", dockerPrefix, name))
		if err != nil {
			return nil, errors.Wrapf(origErr, "reference %q must be a docker reference", name)
		}
	}
	return imageRef, nil
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
func GetImages(w http.ResponseWriter, r *http.Request) ([]*image.Image, error) {
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	query := struct {
		All     bool
		Filters map[string][]string `schema:"filters"`
		Digests bool
	}{
		// This is where you can override the golang default value for one of fields
	}
	// TODO I think all is implemented with a filter?

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		return nil, err
	}
	var filters = []string{}
	if _, found := r.URL.Query()["digests"]; found && query.Digests {
		UnSupportedParameter("digests")
	}

	if len(query.Filters) > 0 {
		for k, v := range query.Filters {
			for _, val := range v {
				filters = append(filters, fmt.Sprintf("%s=%s", k, val))
			}
		}
		return runtime.ImageRuntime().GetImagesWithFilters(filters)
	} else {
		return runtime.ImageRuntime().GetImages()
	}

}

func GetImage(r *http.Request, name string) (*image.Image, error) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	return runtime.ImageRuntime().NewFromLocal(name)
}
