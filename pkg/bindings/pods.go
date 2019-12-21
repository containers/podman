package bindings

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/containers/libpod/libpod"
)

/*
	All methods still need error handling defined based on the http response codes. This needs to be designed
	and discussed and documented.
*/

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
	response, err := http.Get(c.makeEndpoint(fmt.Sprintf("/pods/%s/json", nameOrID)))
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	inspect := libpod.PodInspect{}
	err = json.Unmarshal(data, &inspect)
	return &inspect, err
}

func (c Connection) KillPod(nameOrID string, signal int) error {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("pods/%s/kill", nameOrID), nil)
	if err != nil {
		return err
	}
	req.URL.Query().Add("signal", strconv.Itoa(signal))
	_, err = client.Do(req)
	return err
}

func (c Connection) PausePod(nameOrID string) error {
	_, err := http.Post(c.makeEndpoint(fmt.Sprintf("/pods/%s/pause", nameOrID)), "application/json", nil)
	return err
}

func (c Connection) PrunePods(force bool) error {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("pods/prune"), nil)
	if err != nil {
		return err
	}
	req.URL.Query().Add("force", strconv.FormatBool(force))
	_, err = client.Do(req)
	return err
}

func (c Connection) ListPods(filters []string) (*[]libpod.PodInspect, error) {
	var (
		inspect []libpod.PodInspect
	)
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, "pods/json", nil)
	if err != nil {
		return nil, err
	}
	// TODO I dont remember how to do this for []string{}
	// FIXME
	//req.URL.Query().Add("filters", strconv.FormatBool(force))
	response, err := client.Do(req)
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

func (c Connection) RestartPod(nameOrID string) error {
	_, err := http.Post(c.makeEndpoint(fmt.Sprintf("/pods/%s/restart", nameOrID)), "application/json", nil)
	return err
}

func (c Connection) RemovePod(nameOrID string, force bool) error {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("pods/%s", nameOrID), nil)
	if err != nil {
		return err
	}
	req.URL.Query().Add("force", strconv.FormatBool(force))
	_, err = client.Do(req)
	return err
}

func (c Connection) StartPod(nameOrID string) error {
	_, err := http.Post(c.makeEndpoint(fmt.Sprintf("pods/%s/start", nameOrID)), "application/json", nil)
	return err
}

func (c Connection) PodStats() error {
	// TODO
	return ErrNotImplemented
}

func (c Connection) StopPod(nameOrID string, timeout int) error {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("pods/%s/stop", nameOrID), nil)
	if err != nil {
		return err
	}
	req.URL.Query().Add("t", strconv.Itoa(timeout))
	_, err = client.Do(req)
	return err
}

func (c Connection) PodTop() error {
	// TODO
	return ErrNotImplemented // nolint:typecheck
}

func (c Connection) UnpausePod(nameOrID string) error {
	_, err := http.Post(c.makeEndpoint(fmt.Sprintf("/pods/%s/unpause", nameOrID)), "application/json", nil)
	return err
}
