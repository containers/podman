package bindings

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
)

/*
	All methods still need error handling defined based on the http response codes.
*/

func (c Connection) ListContainers(filter []string, last int, size, sync bool) ([]shared.PsContainerOutput, error) { // nolint:typecheck
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, c.makeEndpoint("/containers/json"), nil)
	if err != nil {
		return nil, err
	}
	// I dont remember how to do deal with []strings here
	//req.URL.Query().Add("filter", filter)
	req.URL.Query().Add("last", strconv.Itoa(last))
	req.URL.Query().Add("size", strconv.FormatBool(size))
	req.URL.Query().Add("sync", strconv.FormatBool(sync))
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	images := []shared.PsContainerOutput{}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &images)
	return images, err
}

func (c Connection) PruneContainers() {}

// TODO Remove can be done once a rebase is completed to pick up the new
// struct responses from remove.
func (c Connection) RemoveContainer() {}
func (c Connection) InspectContainer(nameOrID string, size bool) (*libpod.InspectContainerData, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, c.makeEndpoint(fmt.Sprintf("/containers/%s/json", nameOrID)), nil)
	if err != nil {
		return nil, err
	}
	req.URL.Query().Add("size", strconv.FormatBool(size))
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	inspect := libpod.InspectContainerData{}
	err = json.Unmarshal(data, &inspect)
	return &inspect, err
}

func (c Connection) KillContainer(nameOrID string, signal int) error {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, c.makeEndpoint(fmt.Sprintf("/containers/%s/kill", nameOrID)), nil)
	if err != nil {
		return err
	}
	req.URL.Query().Add("signal", strconv.Itoa(signal))
	_, err = client.Do(req)
	return err

}
func (c Connection) ContainerLogs() {}
func (c Connection) PauseContainer(nameOrID string) error {
	_, err := http.Post(c.makeEndpoint(fmt.Sprintf("/containers/%s/pause", nameOrID)), "application/json", nil)
	return err
}

func (c Connection) RestartContainer(nameOrID string, timeout int) error {
	// TODO how do we distinguish between an actual zero value and not wanting to change the timeout value
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, c.makeEndpoint(fmt.Sprintf("/containers/%s/stop", nameOrID)), nil)
	if err != nil {
		return err
	}
	req.URL.Query().Add("t", strconv.Itoa(timeout))
	_, err = client.Do(req)
	return err
}

func (c Connection) StartContainer(nameOrID, detachKeys string) error {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, c.makeEndpoint(fmt.Sprintf("/containers/%s/start", nameOrID)), nil)
	if err != nil {
		return err
	}
	if len(detachKeys) > 0 {
		req.URL.Query().Add("detachKeys", detachKeys)
	}

	_, err = client.Do(req)
	return err
}

func (c Connection) ContainerStats() {}
func (c Connection) ContainerTop()   {}

func (c Connection) UnpauseContainer(nameOrID string) error {
	_, err := http.Post(c.makeEndpoint(fmt.Sprintf("/containers/%s/unpause", nameOrID)), "application/json", nil)
	return err
}

func (c Connection) WaitContainer(nameOrID string) error {
	_, err := http.Post(c.makeEndpoint(fmt.Sprintf("containers/%s/wait", nameOrID)), "application/json", nil)
	return err
}

func (c Connection) ContainerExists(nameOrID string) (bool, error) {
	response, err := http.Get(c.makeEndpoint(fmt.Sprintf("/containers/%s/exists", nameOrID)))
	if err != nil {
		return false, err
	}
	if response.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, nil
}

func (c Connection) StopContainer(nameOrID string, timeout int) error {
	// TODO we might need to distinguish whether a timeout is desired; a zero, the int
	// zero value is valid; what do folks want to do?
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, c.makeEndpoint(fmt.Sprintf("/containers/%s/stop", nameOrID)), nil)
	if err != nil {
		return err
	}
	req.URL.Query().Add("t", strconv.Itoa(timeout))

	_, err = client.Do(req)
	return err
}
