package containers

import (
	"context"
	"net/http"

	"github.com/containers/podman/v3/pkg/api/handlers"
	"github.com/containers/podman/v3/pkg/bindings"
)

// Commit creates a container image from a container.  The container is defined by nameOrID.  Use
// the CommitOptions for finer grain control on characteristics of the resulting image.
func Commit(ctx context.Context, nameOrID string, options *CommitOptions) (handlers.IDResponse, error) {
	if options == nil {
		options = new(CommitOptions)
	}
	id := handlers.IDResponse{}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return id, err
	}
	params, err := options.ToParams()
	if err != nil {
		return handlers.IDResponse{}, err
	}
	params.Set("container", nameOrID)
	response, err := conn.DoRequest(nil, http.MethodPost, "/commit", params, nil)
	if err != nil {
		return id, err
	}
	return id, response.Process(&id)
}
