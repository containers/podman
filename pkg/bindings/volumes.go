package bindings

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers"
)

func (c Connection) CreateVolume(config handlers.VolumeCreateConfig) (string, error) {
	var (
		volumeID string
	)
	response, err := c.newRequest(http.MethodPost, "/volumes/create", nil, nil)
	if err != nil {
		return volumeID, err
	}
	return volumeID, response.Process(&volumeID)
}

func (c Connection) InspectVolume(nameOrID string) (*libpod.InspectVolumeData, error) {
	var (
		inspect libpod.InspectVolumeData
	)
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/volumes/%s/json", nameOrID), nil, nil)
	if err != nil {
		return &inspect, err
	}
	return &inspect, response.Process(&inspect)
}

func (c Connection) ListVolumes() error {
	// TODO
	// The API side of things for this one does a lot in main and therefore
	// is not implemented yet.
	return ErrNotImplemented // nolint:typecheck
}

func (c Connection) PruneVolumes() ([]string, error) {
	var (
		pruned []string
	)
	response, err := c.newRequest(http.MethodPost, "/volumes/prune", nil, nil)
	if err != nil {
		return pruned, err
	}
	return pruned, response.Process(&pruned)
}

func (c Connection) RemoveVolume(nameOrID string, force bool) error {
	params := make(map[string]string)
	params["force"] = strconv.FormatBool(force)
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/volumes/prune", nameOrID), nil, params)
	if err != nil {
		return err
	}
	return response.Process(nil)
}
