package containers

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/bindings"
)

// Commit creates a container image from a container.  The container is defined by nameOrId.  Use
// the CommitOptions for finer grain control on characteristics of the resulting image.
func Commit(ctx context.Context, nameOrId string, options CommitOptions) (handlers.IDResponse, error) {
	id := handlers.IDResponse{}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return id, err
	}
	params := url.Values{}
	params.Set("container", nameOrId)
	if options.Author != nil {
		params.Set("author", *options.Author)
	}
	for _, change := range options.Changes {
		params.Set("changes", change)
	}
	if options.Comment != nil {
		params.Set("comment", *options.Comment)
	}
	if options.Format != nil {
		params.Set("format", *options.Format)
	}
	if options.Pause != nil {
		params.Set("pause", strconv.FormatBool(*options.Pause))
	}
	if options.Repo != nil {
		params.Set("repo", *options.Repo)
	}
	if options.Tag != nil {
		params.Set("tag", *options.Tag)
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/commit", params, nil)
	if err != nil {
		return id, err
	}
	return id, response.Process(&id)
}
