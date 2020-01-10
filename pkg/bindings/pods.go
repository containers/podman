package bindings

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/containers/libpod/libpod"
)

func (c Connection) CreatePod() error {
	// TODO
	return ErrNotImplemented
}

func (c Connection) PodExists(nameOrID string) (bool, error) {
	response, err := http.Get(c.makeEndpoint(fmt.Sprintf("/pods/%s/exists", nameOrID)))
	if err != nil {
		return false, err
	}
	return response.StatusCode == http.StatusOK, err
}

func (c Connection) InspectPod(nameOrID string) (*libpod.PodInspect, error) {
	inspect := libpod.PodInspect{}
	response, err := c.newRequest(http.MethodGet, fmt.Sprintf("/pods/%s/json", nameOrID), nil, nil)
	if err != nil {
		return &inspect, err
	}
	return &inspect, response.Process(&inspect)
}

func (c Connection) KillPod(nameOrID string, signal int) error {
	params := make(map[string]string)
	params["signal"] = strconv.Itoa(signal)
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/pods/%s/kill", nameOrID), nil, params)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) PausePod(nameOrID string) error {
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/pods/%s/pause", nameOrID), nil, nil)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) PrunePods(force bool) error {
	params := make(map[string]string)
	params["force"] = strconv.FormatBool(force)
	response, err := c.newRequest(http.MethodPost, "/pods/prune", nil, params)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) ListPods(filters []string) (*[]libpod.PodInspect, error) {
	var (
		inspect []libpod.PodInspect
	)
	params := make(map[string]string)
	// TODO I dont remember how to do this for []string{}
	// FIXME
	//params["filters"] = strconv.FormatBool(force)
	response, err := c.newRequest(http.MethodPost, "/pods/json", nil, params)
	if err != nil {
		return &inspect, err
	}
	return &inspect, response.Process(&inspect)
}

func (c Connection) RestartPod(nameOrID string) error {
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/pods/%s/restart", nameOrID), nil, nil)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) RemovePod(nameOrID string, force bool) error {
	params := make(map[string]string)
	params["force"] = strconv.FormatBool(force)
	response, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/pods/%s", nameOrID), nil, params)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) StartPod(nameOrID string) error {
	response, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/pods/%s/start", nameOrID), nil, nil)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) PodStats() error {
	// TODO
	return ErrNotImplemented
}

func (c Connection) StopPod(nameOrID string, timeout int) error {
	params := make(map[string]string)
	params["t"] = strconv.Itoa(timeout)
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/pods/%s/stop", nameOrID), nil, params)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) PodTop() error {
	// TODO
	return ErrNotImplemented // nolint:typecheck
}

func (c Connection) UnpausePod(nameOrID string) error {
	response, err := c.newRequest(http.MethodPost, fmt.Sprintf("/pods/%s/unpause", nameOrID), nil, nil)
	if err != nil {
		return err
	}
	return response.Process(nil)
}
