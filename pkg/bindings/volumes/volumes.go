package volumes

import (
	"context"
	"net/http"
	"strconv"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/bindings"
)

// Create creates a volume given its configuration.
func Create(ctx context.Context, config handlers.VolumeCreateConfig) (string, error) {
	// TODO This is incomplete.  The config needs to be sent via the body
	var (
		volumeID string
	)
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return "", err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/volumes/create", nil)
	if err != nil {
		return volumeID, err
	}
	return volumeID, response.Process(&volumeID)
}

// Inspect returns low-level information about a volume.
func Inspect(ctx context.Context, nameOrID string) (*libpod.InspectVolumeData, error) {
	var (
		inspect libpod.InspectVolumeData
	)
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/volumes/%s/json", nil, nameOrID)
	if err != nil {
		return &inspect, err
	}
	return &inspect, response.Process(&inspect)
}

func List() error {
	// TODO
	// The API side of things for this one does a lot in main and therefore
	// is not implemented yet.
	return bindings.ErrNotImplemented // nolint:typecheck
}

// Prune removes unused volumes from the local filesystem.
func Prune(ctx context.Context) ([]string, error) {
	var (
		pruned []string
	)
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/volumes/prune", nil)
	if err != nil {
		return pruned, err
	}
	return pruned, response.Process(&pruned)
}

// Remove deletes the given volume from storage. The optional force parameter
// is used to remove a volume even if it is being used by a container.
func Remove(ctx context.Context, nameOrID string, force *bool) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	params := make(map[string]string)
	if force != nil {
		params["force"] = strconv.FormatBool(*force)
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/volumes/%s/prune", params, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}
