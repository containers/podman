package bindings

import (
	"net/http"
	"strconv"

	"github.com/containers/libpod/libpod/image"
)

type ImageSearchFilters struct {
	Automated bool `json:"automated"`
	Official  bool `json:"official"`
	Stars     int  `json:"stars"`
}

// TODO This method can be concluded when we determine how we want the filters to work on the
// API end
func (i *ImageSearchFilters) ToMapJSON() string {
	return ""
}

func (c Connection) SearchImages(term string, limit int, filters *ImageSearchFilters) ([]image.SearchResult, error) {
	var (
		searchResults []image.SearchResult
	)
	params := make(map[string]string)
	params["term"] = term
	if limit > 0 {
		params["limit"] = strconv.Itoa(limit)
	}
	if filters != nil {
		params["filters"] = filters.ToMapJSON()
	}
	response, err := c.newRequest(http.MethodGet, "/images/search", nil, params)
	if err != nil {
		return searchResults, nil
	}
	return searchResults, response.Process(&searchResults)
}
