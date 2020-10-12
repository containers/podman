package images

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/auth"
	"github.com/containers/podman/v2/pkg/bindings"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
)

// Exists a lightweight way to determine if an image exists in local storage.  It returns a
// boolean response.
func Exists(ctx context.Context, nameOrID string) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/exists", nil, nil, nameOrID)
	if err != nil {
		return false, err
	}
	return response.IsSuccess(), nil
}

// List returns a list of images in local storage.  The all boolean and filters parameters are optional
// ways to alter the image query.
func List(ctx context.Context, all *bool, filters map[string][]string) ([]*entities.ImageSummary, error) {
	var imageSummary []*entities.ImageSummary
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if all != nil {
		params.Set("all", strconv.FormatBool(*all))
	}
	if filters != nil {
		strFilters, err := bindings.FiltersToString(filters)
		if err != nil {
			return nil, err
		}
		params.Set("filters", strFilters)
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/json", params, nil)
	if err != nil {
		return imageSummary, err
	}
	return imageSummary, response.Process(&imageSummary)
}

// Get performs an image inspect.  To have the on-disk size of the image calculated, you can
// use the optional size parameter.
func GetImage(ctx context.Context, nameOrID string, size *bool) (*entities.ImageInspectReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if size != nil {
		params.Set("size", strconv.FormatBool(*size))
	}
	inspectedData := entities.ImageInspectReport{}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/json", params, nil, nameOrID)
	if err != nil {
		return &inspectedData, err
	}
	return &inspectedData, response.Process(&inspectedData)
}

// Tree retrieves a "tree" based representation of the given image
func Tree(ctx context.Context, nameOrID string, whatRequires *bool) (*entities.ImageTreeReport, error) {
	var report entities.ImageTreeReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if whatRequires != nil {
		params.Set("size", strconv.FormatBool(*whatRequires))
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/tree", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	return &report, response.Process(&report)
}

// History returns the parent layers of an image.
func History(ctx context.Context, nameOrID string) ([]*handlers.HistoryResponse, error) {
	var history []*handlers.HistoryResponse
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/history", nil, nil, nameOrID)
	if err != nil {
		return history, err
	}
	return history, response.Process(&history)
}

func Load(ctx context.Context, r io.Reader, name *string) (*entities.ImageLoadReport, error) {
	var report entities.ImageLoadReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if name != nil {
		params.Set("reference", *name)
	}
	response, err := conn.DoRequest(r, http.MethodPost, "/images/load", params, nil)
	if err != nil {
		return nil, err
	}
	return &report, response.Process(&report)
}

func MultiExport(ctx context.Context, namesOrIds []string, w io.Writer, format *string, compress *bool) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	if format != nil {
		params.Set("format", *format)
	}
	if compress != nil {
		params.Set("compress", strconv.FormatBool(*compress))
	}
	for _, ref := range namesOrIds {
		params.Add("references", ref)
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/export", params, nil)
	if err != nil {
		return err
	}

	if response.StatusCode/100 == 2 || response.StatusCode/100 == 3 {
		_, err = io.Copy(w, response.Body)
		return err
	}
	return response.Process(nil)

}

// Export saves an image from local storage as a tarball or image archive.  The optional format
// parameter is used to change the format of the output.
func Export(ctx context.Context, nameOrID string, w io.Writer, format *string, compress *bool) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	if format != nil {
		params.Set("format", *format)
	}
	if compress != nil {
		params.Set("compress", strconv.FormatBool(*compress))
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/get", params, nil, nameOrID)
	if err != nil {
		return err
	}

	if response.StatusCode/100 == 2 || response.StatusCode/100 == 3 {
		_, err = io.Copy(w, response.Body)
		return err
	}
	return response.Process(nil)
}

// Prune removes unused images from local storage.  The optional filters can be used to further
// define which images should be pruned.
func Prune(ctx context.Context, all *bool, filters map[string][]string) ([]string, error) {
	var (
		deleted []string
	)
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if all != nil {
		params.Set("all", strconv.FormatBool(*all))
	}
	if filters != nil {
		stringFilter, err := bindings.FiltersToString(filters)
		if err != nil {
			return nil, err
		}
		params.Set("filters", stringFilter)
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/images/prune", params, nil)
	if err != nil {
		return deleted, err
	}
	return deleted, response.Process(&deleted)
}

// Tag adds an additional name to locally-stored image. Both the tag and repo parameters are required.
func Tag(ctx context.Context, nameOrID, tag, repo string) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Set("tag", tag)
	params.Set("repo", repo)
	response, err := conn.DoRequest(nil, http.MethodPost, "/images/%s/tag", params, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Untag removes a name from locally-stored image. Both the tag and repo parameters are required.
func Untag(ctx context.Context, nameOrID, tag, repo string) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	params.Set("tag", tag)
	params.Set("repo", repo)
	response, err := conn.DoRequest(nil, http.MethodPost, "/images/%s/untag", params, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Imports adds the given image to the local image store.  This can be done by file and the given reader
// or via the url parameter.  Additional metadata can be associated with the image by using the changes and
// message parameters.  The image can also be tagged given a reference. One of url OR r must be provided.
func Import(ctx context.Context, changes []string, message, reference, u *string, r io.Reader) (*entities.ImageImportReport, error) {
	var report entities.ImageImportReport
	if r != nil && u != nil {
		return nil, errors.New("url and r parameters cannot be used together")
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	for _, change := range changes {
		params.Add("changes", change)
	}
	if message != nil {
		params.Set("message", *message)
	}
	if reference != nil {
		params.Set("reference", *reference)
	}
	if u != nil {
		params.Set("url", *u)
	}
	response, err := conn.DoRequest(r, http.MethodPost, "/images/import", params, nil)
	if err != nil {
		return nil, err
	}
	return &report, response.Process(&report)
}

// Push is the binding for libpod's v2 endpoints for push images.  Note that
// `source` must be a referring to an image in the remote's container storage.
// The destination must be a reference to a registry (i.e., of docker transport
// or be normalized to one).  Other transports are rejected as they do not make
// sense in a remote context.
func Push(ctx context.Context, source string, destination string, options entities.ImagePushOptions) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	// TODO: have a global system context we can pass around (1st argument)
	header, err := auth.Header(nil, auth.XRegistryAuthHeader, options.Authfile, options.Username, options.Password)
	if err != nil {
		return err
	}

	params := url.Values{}
	params.Set("destination", destination)
	if options.SkipTLSVerify != types.OptionalBoolUndefined {
		// Note: we have to verify if skipped is false.
		verifyTLS := bool(options.SkipTLSVerify == types.OptionalBoolFalse)
		params.Set("tlsVerify", strconv.FormatBool(verifyTLS))
	}

	path := fmt.Sprintf("/images/%s/push", source)
	response, err := conn.DoRequest(nil, http.MethodPost, path, params, header)
	if err != nil {
		return err
	}

	return response.Process(err)
}

// Search is the binding for libpod's v2 endpoints for Search images.
func Search(ctx context.Context, term string, opts entities.ImageSearchOptions) ([]entities.ImageSearchReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("term", term)
	params.Set("limit", strconv.Itoa(opts.Limit))
	params.Set("noTrunc", strconv.FormatBool(opts.NoTrunc))
	params.Set("listTags", strconv.FormatBool(opts.ListTags))
	for _, f := range opts.Filters {
		params.Set("filters", f)
	}

	if opts.SkipTLSVerify != types.OptionalBoolUndefined {
		// Note: we have to verify if skipped is false.
		verifyTLS := bool(opts.SkipTLSVerify == types.OptionalBoolFalse)
		params.Set("tlsVerify", strconv.FormatBool(verifyTLS))
	}

	// TODO: have a global system context we can pass around (1st argument)
	header, err := auth.Header(nil, auth.XRegistryAuthHeader, opts.Authfile, "", "")
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(nil, http.MethodGet, "/images/search", params, header)
	if err != nil {
		return nil, err
	}

	results := []entities.ImageSearchReport{}
	if err := response.Process(&results); err != nil {
		return nil, err
	}

	return results, nil
}
