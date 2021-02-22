package images

import (
	"context"
	"net/http"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/storage/pkg/archive"
)

// Diff provides the changes between two container layers
func Diff(ctx context.Context, nameOrID string, options *DiffOptions) ([]archive.Change, error) {
	if options == nil {
		options = new(DiffOptions)
	}
	_ = options
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/changes", nil, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	var changes []archive.Change
	return changes, response.Process(&changes)
}
