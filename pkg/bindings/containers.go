package bindings

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
)

func (c Connection) ListContainers(filter []string, last int, size, sync bool) ([]shared.PsContainerOutput, error) { // nolint:typecheck
	images := []shared.PsContainerOutput{}
	params := make(map[string]string)
	params["last"] = strconv.Itoa(last)
	params["size"] = strconv.FormatBool(size)
	params["sync"] = strconv.FormatBool(sync)
	response, err := c.newRequest(http.MethodGet, "/containers/json", nil, params)
	if err != nil {
		return images, err
	}
	return images, response.Process(nil)
}

func (c Connection) PruneContainers() ([]string, error) {
	var (
		pruned []string
	)
	response, err := c.newRequest(http.MethodPost, "/containers/prune", nil, nil)
	if err != nil {
		return pruned, err
	}
	return pruned, response.Process(nil)
}

func (c Connection) RemoveContainer(nameOrID string, force, volumes bool) error {
	params := make(map[string]string)
	params["force"] = strconv.FormatBool(force)
	params["vols"] = strconv.FormatBool(volumes)
	response, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/containers/%s", nameOrID), nil, params)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) InspectContainer(nameOrID string, size bool) (*libpod.InspectContainerData, error) {
	params := make(map[string]string)
	params["size"] = strconv.FormatBool(size)
	response, err := c.newRequest(http.MethodGet, fmt.Sprintf("/containers/%s/json", nameOrID), nil, params)
	if err != nil {
		return nil, err
	}
	inspect := libpod.InspectContainerData{}
	return &inspect, response.Process(&inspect)
}

func (c Connection) KillContainer(nameOrID string, signal int) error {
	params := make(map[string]string)
	params["signal"] = strconv.Itoa(signal)
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/containers/%s/kill", nameOrID), nil, params)
	if err != nil {
		return err
	}
	return response.Process(nil)

}
func (c Connection) ContainerLogs() {}
func (c Connection) PauseContainer(nameOrID string) error {
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/containers/%s/pause", nameOrID), nil, nil)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) RestartContainer(nameOrID string, timeout int) error {
	// TODO how do we distinguish between an actual zero value and not wanting to change the timeout value
	params := make(map[string]string)
	params["timeout"] = strconv.Itoa(timeout)
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/containers/%s/restart", nameOrID), nil, params)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) StartContainer(nameOrID, detachKeys string) error {
	params := make(map[string]string)
	if len(detachKeys) > 0 {
		params["detachKeys"] = detachKeys
	}
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/containers/%s/start", nameOrID), nil, params)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) ContainerStats() {}
func (c Connection) ContainerTop()   {}

func (c Connection) UnpauseContainer(nameOrID string) error {
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/containers/%s/unpause", nameOrID), nil, nil)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) WaitContainer(nameOrID string) error {
	// TODO when returns are ironed out, we can should use the newRequest approach
	_, err := http.Post(c.makeEndpoint(fmt.Sprintf("containers/%s/wait", nameOrID)), "application/json", nil) // nolint
	return err
}

func (c Connection) ContainerExists(nameOrID string) (bool, error) {
	response, err := http.Get(c.makeEndpoint(fmt.Sprintf("/containers/%s/exists", nameOrID))) // nolint
	defer closeResponseBody(response)
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
	params := make(map[string]string)
	params["t"] = strconv.Itoa(timeout)
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/containers/%s/stop", nameOrID), nil, params)
	if err != nil {
		return err
	}
	return response.Process(nil)
}
