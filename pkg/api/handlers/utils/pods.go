package utils

import (
	"fmt"
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/gorilla/schema"
)

func GetPods(w http.ResponseWriter, r *http.Request) ([]*libpod.Pod, error) {
	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	decoder := r.Context().Value("decoder").(*schema.Decoder)

	query := struct {
		All     bool
		Filters map[string][]string `schema:"filters"`
		Digests bool
	}{}

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
		return runtime.GetPodsWithFilters(filters)
	}

	return runtime.GetAllPods()

}
