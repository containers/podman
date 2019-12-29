package bindings

import (
	"encoding/json"
	"io/ioutil"
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

/*
	All methods still need error handling defined based on the http response codes.
*/

func (c Connection) SearchImages(term string, limit int, filters *ImageSearchFilters) ([]image.SearchResult, error) {
	var (
		searchResults []image.SearchResult
	)
	client := &http.Client{}
	request, err := http.NewRequest(http.MethodGet, c.makeEndpoint("/images/search"), nil)
	if err != nil {
		return nil, err
	}
	request.URL.Query().Add("term", term)
	if limit > 0 {
		request.URL.Query().Add("limit", strconv.Itoa(limit))
	}
	if filters != nil {
		request.URL.Query().Add("filters", filters.ToMapJSON())
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &searchResults)
	return searchResults, err
}
