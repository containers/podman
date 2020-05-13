package volumes

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/domain/entities"
	jsoniter "github.com/json-iterator/go"
)

// Create creates a volume given its configuration.
func Create(ctx context.Context, config entities.VolumeCreateOptions) (*entities.VolumeConfigResponse, error) {
	var (
		v entities.VolumeConfigResponse
	)
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
func Inspect(ctx context.Context, nameOrID string) (*entities.VolumeConfigResponse, error) {
	var (
		inspect entities.VolumeConfigResponse
	)
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
func List(ctx context.Context, filters map[string][]string) ([]*entities.VolumeListReport, error) {
	var (
		vols []*entities.VolumeListReport
	)
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if len(filters) > 0 {
		strFilters, err := bindings.FiltersToString(filters)
		if err != nil {
			return nil, err
		}
		params.Set("filters", strFilters)
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/volumes/json", params, nil)
	if err != nil {
		return vols, err
	}
	return vols, response.Process(&vols)
}

// Prune removes unused volumes from the local filesystem.
func Prune(ctx context.Context) ([]*entities.VolumePruneReport, error) {
	var (
		pruned []*entities.VolumePruneReport
	)
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodPost, "/volumes/prune", nil, nil)
	if err != nil {
		return nil, err
	}
	return pruned, response.Process(&pruned)
}

// Remove deletes the given volume from storage. The optional force parameter
// is used to remove a volume even if it is being used by a container.
func Remove(ctx context.Context, nameOrID string, force *bool) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params := url.Values{}
	if force != nil {
		params.Set("force", strconv.FormatBool(*force))
	}
	response, err := conn.DoRequest(nil, http.MethodDelete, "/volumes/%s", params, nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}
