package images

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/containers/podman/v3/pkg/api/handlers/types"
	"github.com/containers/podman/v3/pkg/auth"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/entities/reports"
	"github.com/pkg/errors"
)

// Exists a lightweight way to determine if an image exists in local storage.  It returns a
// boolean response.
func Exists(ctx context.Context, nameOrID string, options *ExistsOptions) (bool, error) {
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
func List(ctx context.Context, options *ListOptions) ([]*entities.ImageSummary, error) {
	if options == nil {
		options = new(ListOptions)
	}
	var imageSummary []*entities.ImageSummary
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/json", params, nil)
	if err != nil {
		return imageSummary, err
	}
	return imageSummary, response.Process(&imageSummary)
}

// Get performs an image inspect.  To have the on-disk size of the image calculated, you can
// use the optional size parameter.
func GetImage(ctx context.Context, nameOrID string, options *GetOptions) (*entities.ImageInspectReport, error) {
	if options == nil {
		options = new(GetOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	inspectedData := entities.ImageInspectReport{}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/json", params, nil, nameOrID)
	if err != nil {
		return &inspectedData, err
	}
	return &inspectedData, response.Process(&inspectedData)
}

// Tree retrieves a "tree" based representation of the given image
func Tree(ctx context.Context, nameOrID string, options *TreeOptions) (*entities.ImageTreeReport, error) {
	if options == nil {
		options = new(TreeOptions)
	}
	var report entities.ImageTreeReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/tree", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	return &report, response.Process(&report)
}

// History returns the parent layers of an image.
func History(ctx context.Context, nameOrID string, options *HistoryOptions) ([]*types.HistoryResponse, error) {
	if options == nil {
		options = new(HistoryOptions)
	}
	_ = options
	var history []*types.HistoryResponse
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

func Load(ctx context.Context, r io.Reader) (*entities.ImageLoadReport, error) {
	var report entities.ImageLoadReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(r, http.MethodPost, "/images/load", nil, nil)
	if err != nil {
		return nil, err
	}
	return &report, response.Process(&report)
}

// Export saves images from local storage as a tarball or image archive.  The optional format
// parameter is used to change the format of the output.
func Export(ctx context.Context, nameOrIDs []string, w io.Writer, options *ExportOptions) error {
	if options == nil {
		options = new(ExportOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params, err := options.ToParams()
	if err != nil {
		return err
	}
	for _, ref := range nameOrIDs {
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

// Prune removes unused images from local storage.  The optional filters can be used to further
// define which images should be pruned.
func Prune(ctx context.Context, options *PruneOptions) ([]*reports.PruneReport, error) {
	var (
		deleted []*reports.PruneReport
	)
	if options == nil {
		options = new(PruneOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/images/prune", params, nil)
	if err != nil {
		return deleted, err
	}
	err = response.Process(&deleted)
	return deleted, err
}

// Tag adds an additional name to locally-stored image. Both the tag and repo parameters are required.
func Tag(ctx context.Context, nameOrID, tag, repo string, options *TagOptions) error {
	if options == nil {
		options = new(TagOptions)
	}
	_ = options
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
func Untag(ctx context.Context, nameOrID, tag, repo string, options *UntagOptions) error {
	if options == nil {
		options = new(UntagOptions)
	}
	_ = options
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
func Import(ctx context.Context, r io.Reader, options *ImportOptions) (*entities.ImageImportReport, error) {
	if options == nil {
		options = new(ImportOptions)
	}
	var report entities.ImageImportReport
	if r != nil && options.URL != nil {
		return nil, errors.New("url and r parameters cannot be used together")
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
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
func Push(ctx context.Context, source string, destination string, options *PushOptions) error {
	if options == nil {
		options = new(PushOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	// TODO: have a global system context we can pass around (1st argument)
	header, err := auth.Header(nil, auth.XRegistryAuthHeader, options.GetAuthfile(), options.GetUsername(), options.GetPassword())
	if err != nil {
		return err
	}

	params, err := options.ToParams()
	if err != nil {
		return err
	}
	//SkipTLSVerify is special.  We need to delete the param added by
	//toparams and change the key and flip the bool
	if options.SkipTLSVerify != nil {
		params.Del("SkipTLSVerify")
		params.Set("tlsVerify", strconv.FormatBool(!options.GetSkipTLSVerify()))
	}
	params.Set("destination", destination)

	path := fmt.Sprintf("/images/%s/push", source)
	response, err := conn.DoRequest(nil, http.MethodPost, path, params, header)
	if err != nil {
		return err
	}

	return response.Process(err)
}

// Search is the binding for libpod's v2 endpoints for Search images.
func Search(ctx context.Context, term string, options *SearchOptions) ([]entities.ImageSearchReport, error) {
	if options == nil {
		options = new(SearchOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	params.Set("term", term)

	// Note: we have to verify if skipped is false.
	if options.SkipTLSVerify != nil {
		params.Del("SkipTLSVerify")
		params.Set("tlsVerify", strconv.FormatBool(!options.GetSkipTLSVerify()))
	}

	// TODO: have a global system context we can pass around (1st argument)
	header, err := auth.Header(nil, auth.XRegistryAuthHeader, options.GetAuthfile(), "", "")
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
