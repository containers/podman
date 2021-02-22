package containers

import (
	"context"
	"net/http"

	"github.com/containers/podman/v3/pkg/bindings"
)

// Rename an existing container.
func Rename(ctx context.Context, nameOrID string, options *RenameOptions) error {
	if options == nil {
		options = new(RenameOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params, err := options.ToParams()
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/containers/%s/rename", params, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}
