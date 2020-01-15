package bindings

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/inspect"
)

func (c Connection) ImageExists(nameOrID string) (bool, error) {
	response, err := http.Get(c.makeEndpoint(fmt.Sprintf("/images/%s/exists", nameOrID))) // nolint
	defer closeResponseBody(response)
	if err != nil {
		return false, err
	}
	if response.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, nil
}

func (c Connection) ListImages() ([]handlers.ImageSummary, error) {
	imageSummary := []handlers.ImageSummary{}
	response, err := c.newRequest(http.MethodGet, "/images/json", nil, nil)
	if err != nil {
		return imageSummary, err
	}
	return imageSummary, response.Process(&imageSummary)
}

func (c Connection) GetImage(nameOrID string) (*inspect.ImageData, error) {
	inspectedData := inspect.ImageData{}
	response, err := c.newRequest(http.MethodGet, fmt.Sprintf("/images/%s/json", nameOrID), nil, nil)
	if err != nil {
		return &inspectedData, err
	}
	return &inspectedData, response.Process(&inspectedData)
}

func (c Connection) ImageTree(nameOrId string) error {
	return ErrNotImplemented
}

func (c Connection) ImageHistory(nameOrID string) ([]handlers.HistoryResponse, error) {
	history := []handlers.HistoryResponse{}
	response, err := c.newRequest(http.MethodGet, fmt.Sprintf("/images/%s/history", nameOrID), nil, nil)
	if err != nil {
		return history, err
	}
	return history, response.Process(&history)
}

func (c Connection) LoadImage(r io.Reader) error {
	// TODO this still needs error handling added
	_, err := http.Post(c.makeEndpoint("/images/loads"), "application/json", r) //nolint
	return err
}

func (c Connection) RemoveImage(nameOrID string, force bool) ([]map[string]string, error) {
	deletes := []map[string]string{}
	params := make(map[string]string)
	params["force"] = strconv.FormatBool(force)
	response, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/images/%s", nameOrID), nil, params)
	if err != nil {
		return nil, err
	}
	return deletes, response.Process(&deletes)
}

func (c Connection) ExportImage(nameOrID string, w io.Writer, format string, compress bool) error {
	params := make(map[string]string)
	params["format"] = format
	params["compress"] = strconv.FormatBool(compress)
	response, err := c.newRequest(http.MethodGet, fmt.Sprintf("/images/%s/get", nameOrID), nil, params)
	if err != nil {
		return err
	}
	if err := response.Process(nil); err != nil {
		return err
	}
	_, err = io.Copy(w, response.Body)
	return err
}

func (c Connection) PruneImages(all bool, filters []string) ([]string, error) {
	var (
		deleted []string
	)
	params := make(map[string]string)
	// FIXME How do we do []strings?
	//params["filters"] = format
	response, err := c.newRequest(http.MethodPost, "/images/prune", nil, params)
	if err != nil {
		return deleted, err
	}
	return deleted, response.Process(nil)
}

func (c Connection) TagImage(nameOrID string) error {
	var ()
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/images/%s/tag", nameOrID), nil, nil)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) BuildImage(nameOrId string) {}
