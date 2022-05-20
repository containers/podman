package containers

import (
	"context"
	"net/http"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

// Commit creates a container image from a container.  The container is defined by nameOrID.  Use
// the CommitOptions for finer grain control on characteristics of the resulting image.
func Commit(ctx context.Context, nameOrID string, options *CommitOptions) (entities.IDResponse, error) {
	if options == nil {
		options = new(CommitOptions)
	}
	id := entities.IDResponse{}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return id, err
	}
	params, err := options.ToParams()
	if err != nil {
		return entities.IDResponse{}, err
	}
	params.Set("container", nameOrID)
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/commit", params, nil)
	if err != nil {
		return id, err
	}
	defer response.Body.Close()

	return id, response.Process(&id)
}
