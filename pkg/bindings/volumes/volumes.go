package volumes

import (
	"context"
	"net/http"
	"strings"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/domain/entities/reports"
	jsoniter "github.com/json-iterator/go"
)

// Create creates a volume given its configuration.
func Create(ctx context.Context, config entities.VolumeCreateOptions, options *CreateOptions) (*entities.VolumeConfigResponse, error) {
	var (
		v entities.VolumeConfigResponse
	)
	if options == nil {
		options = new(CreateOptions)
	}
	_ = options
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	createString, err := jsoniter.MarshalToString(config)
	if err != nil {
		return nil, err
	}
	stringReader := strings.NewReader(createString)
	response, err := conn.DoRequest(stringReader, http.MethodPost, "/volumes/create", nil, nil)
	if err != nil {
		return nil, err
	}
	return &v, response.Process(&v)
}

// Inspect returns low-level information about a volume.
func Inspect(ctx context.Context, nameOrID string, options *InspectOptions) (*entities.VolumeConfigResponse, error) {
	var (
		inspect entities.VolumeConfigResponse
	)
	if options == nil {
		options = new(InspectOptions)
	}
	_ = options
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/volumes/%s/json", nil, nil, nameOrID)
	if err != nil {
		return &inspect, err
	}
	return &inspect, response.Process(&inspect)
}

// List returns the configurations for existing volumes in the form of a slice.  Optionally, filters
// can be used to refine the list of volumes.
func List(ctx context.Context, options *ListOptions) ([]*entities.VolumeListReport, error) {
	var (
		vols []*entities.VolumeListReport
	)
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/volumes/json", params, nil)
	if err != nil {
		return vols, err
	}
	return vols, response.Process(&vols)
}

// Prune removes unused volumes from the local filesystem.
func Prune(ctx context.Context, options *PruneOptions) ([]*reports.PruneReport, error) {
	var (
		pruned []*reports.PruneReport
	)
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/volumes/prune", params, nil)
	if err != nil {
		return nil, err
	}
	return pruned, response.Process(&pruned)
}

// Remove deletes the given volume from storage. The optional force parameter
// is used to remove a volume even if it is being used by a container.
func Remove(ctx context.Context, nameOrID string, options *RemoveOptions) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params, err := options.ToParams()
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodDelete, "/volumes/%s", params, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

// Exists returns true if a given volume exists
func Exists(ctx context.Context, nameOrID string, options *ExistsOptions) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/volumes/%s/exists", nil, nil, nameOrID)
	if err != nil {
		return false, err
	}
	return response.IsSuccess(), nil
}
