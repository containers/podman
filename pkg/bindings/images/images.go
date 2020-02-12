package images

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/inspect"
)

// Exists a lightweight way to determine if an image exists in local storage.  It returns a
// boolean response.
func Exists(ctx context.Context, nameOrID string) (bool, error) {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/exists", nil, nameOrID)
	if err != nil {
		return false, err
	}
	return response.IsSuccess(), nil
}

// List returns a list of images in local storage.  The all boolean and filters parameters are optional
// ways to alter the image query.
func List(ctx context.Context, all *bool, filters map[string][]string) ([]*handlers.ImageSummary, error) {
	var imageSummary []*handlers.ImageSummary
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	if all != nil {
		params["all"] = strconv.FormatBool(*all)
	}
	if filters != nil {
		strFilters, err := bindings.FiltersToString(filters)
		if err != nil {
			return nil, err
		}
		params["filters"] = strFilters
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/json", params)
	if err != nil {
		return imageSummary, err
	}
	return imageSummary, response.Process(&imageSummary)
}

// Get performs an image inspect.  To have the on-disk size of the image calculated, you can
// use the optional size parameter.
func GetImage(ctx context.Context, nameOrID string, size *bool) (*inspect.ImageData, error) {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	if size != nil {
		params["size"] = strconv.FormatBool(*size)
	}
	inspectedData := inspect.ImageData{}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/json", params, nameOrID)
	if err != nil {
		return &inspectedData, err
	}
	return &inspectedData, response.Process(&inspectedData)
}

func ImageTree(ctx context.Context, nameOrId string) error {
	return bindings.ErrNotImplemented
}

// History returns the parent layers of an image.
func History(ctx context.Context, nameOrID string) ([]*handlers.HistoryResponse, error) {
	var history []*handlers.HistoryResponse
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/history", nil, nameOrID)
	if err != nil {
		return history, err
	}
	return history, response.Process(&history)
}

func Load(ctx context.Context, r io.Reader) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	// TODO this still needs error handling added
	//_, err := http.Post(c.makeEndpoint("/images/loads"), "application/json", r) //nolint
	_ = conn
	return bindings.ErrNotImplemented
}

// Remove deletes an image from local storage.  The optional force parameter will forcibly remove
// the image by removing all all containers, including those that are Running, first.
func Remove(ctx context.Context, nameOrID string, force *bool) ([]map[string]string, error) {
	var deletes []map[string]string
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	if force != nil {
		params["force"] = strconv.FormatBool(*force)
	}
	response, err := conn.DoRequest(nil, http.MethodDelete, "/images/%s", params, nameOrID)
	if err != nil {
		return nil, err
	}
	return deletes, response.Process(&deletes)
}

// Export saves an image from local storage as a tarball or image archive.  The optional format
// parameter is used to change the format of the output.
func Export(ctx context.Context, nameOrID string, w io.Writer, format *string, compress *bool) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	if format != nil {
		params["format"] = *format
	}
	if compress != nil {
		params["compress"] = strconv.FormatBool(*compress)
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/get", params, nameOrID)
	if err != nil {
		return err
	}
	if err := response.Process(nil); err != nil {
		return err
	}
	_, err = io.Copy(w, response.Body)
	return err
}

// Prune removes unused images from local storage.  The optional filters can be used to further
// define which images should be pruned.
func Prune(ctx context.Context, filters map[string][]string) ([]string, error) {
	var (
		deleted []string
	)
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	params := make(map[string]string)
	if filters != nil {
		stringFilter, err := bindings.FiltersToString(filters)
		if err != nil {
			return nil, err
		}
		params["filters"] = stringFilter
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/images/prune", params)
	if err != nil {
		return deleted, err
	}
	return deleted, response.Process(nil)
}

// Tag adds an additional name to locally-stored image. Both the tag and repo parameters are required.
func Tag(ctx context.Context, nameOrID, tag, repo string) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	params["tag"] = tag
	params["repo"] = repo
	response, err := conn.DoRequest(nil, http.MethodPost, "/images/%s/tag", params, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func Build(nameOrId string) {}
