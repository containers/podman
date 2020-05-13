package images

import (
	"context"
	"net/http"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/storage/pkg/archive"
)

// Diff provides the changes between two container layers
func Diff(ctx context.Context, nameOrId string) ([]archive.Change, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(nil, http.MethodGet, "/images/%s/changes", nil, nil, nameOrId)
	if err != nil {
		return nil, err
	}
	var changes []archive.Change
	return changes, response.Process(&changes)
}
