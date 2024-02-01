package containers

import (
	"context"
	"net/http"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/storage/pkg/archive"
)

// Diff provides the changes between two container layers
func Diff(ctx context.Context, nameOrID string, options *DiffOptions) ([]archive.Change, error) {
	if options == nil {
		options = new(DiffOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/containers/%s/changes", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var changes []archive.Change
	return changes, response.Process(&changes)
}
