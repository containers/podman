package utils

import (
	"fmt"
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
)

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
	if _, found := mux.Vars(r)["digests"]; found && query.Digests {
		UnSupportedParameter("digests")
	}

	if _, found := r.URL.Query()["filters"]; found {
		filters = append(filters, fmt.Sprintf("reference=%s", ""))
	}
	return runtime.ImageRuntime().GetImagesWithFilters(filters)
}
