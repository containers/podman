package images

import (
	"context"
	"net/http"
	"strconv"

	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/bindings"
)

// Search looks for the given image (term) in container image registries.  The optional limit parameter sets
// a maximum number of results returned.  The optional filters parameter allow for more specific image
// searches.
func Search(ctx context.Context, term string, limit *int, filters map[string][]string) ([]image.SearchResult, error) {
	var (
		searchResults []image.SearchResult
	)
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	params["term"] = term
	if limit != nil {
		params["limit"] = strconv.Itoa(*limit)
	}
	if filters != nil {
		stringFilter, err := bindings.FiltersToHTML(filters)
		if err != nil {
			return nil, err
		}
		params["filters"] = stringFilter
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/search", params)
	if err != nil {
		return searchResults, nil
	}
	return searchResults, response.Process(&searchResults)
}
