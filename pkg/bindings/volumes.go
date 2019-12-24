package bindings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers"
)

/*
	All methods still need error handling defined based on the http response codes.
*/

func (c Connection) CreateVolume(config handlers.VolumeCreateConfig) (string, error) {
	var (
		volumeID string
	)
	b, err := json.Marshal(config)
	if err != nil {
		return "", nil
	}
	response, err := http.Post(c.makeEndpoint("/volumes/create"), "application/json", bytes.NewBuffer(b))
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(data, &volumeID)
	return volumeID, err
}

func (c Connection) InspectVolume(nameOrID string) (*libpod.InspectVolumeData, error) {
	var (
		inspect libpod.InspectVolumeData
	)
	response, err := http.Post(c.makeEndpoint(fmt.Sprintf("/volumes/%s/json", nameOrID)), "application/json", nil)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &inspect)
	return &inspect, err
}

func (c Connection) ListVolumes() error {
	// TODO
	// The API side of things for this one does a lot in main and therefore
	// is not implemented there yet.
	return ErrNotImplemented // nolint:typecheck
}

func (c Connection) PruneVolumes() ([]string, error) {
	var (
		pruned []string
	)
	response, err := http.Post(c.makeEndpoint("/volumes/prune"), "application/json", nil)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &pruned)
	return pruned, err
}

func (c Connection) RemoveVolume(nameOrID string, force bool) error {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodDelete, c.makeEndpoint(fmt.Sprintf("volumes/%s", nameOrID)), nil)
	if err != nil {
		return err
	}
	req.URL.Query().Add("force", strconv.FormatBool(force))
	_, err = client.Do(req)
	return err
}
