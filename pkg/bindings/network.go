package bindings

import (
	"fmt"
	"net/http"

	"github.com/containernetworking/cni/libcni"
)

func (c Connection) CreateNetwork() {}
func (c Connection) InspectNetwork(nameOrID string) (map[string]interface{}, error) {
	n := make(map[string]interface{})
	response, err := c.newRequest(http.MethodGet, fmt.Sprintf("/networks/%s/json", nameOrID), nil, nil)
	if err != nil {
		return n, err
	}
	return n, response.Process(&n)
}

func (c Connection) RemoveNetwork(nameOrID string) error {
	response, err := c.newRequest(http.MethodDelete, fmt.Sprintf("/networks/%s", nameOrID), nil, nil)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func (c Connection) ListNetworks() ([]*libcni.NetworkConfigList, error) {
	var (
		netList []*libcni.NetworkConfigList
	)
	response, err := c.newRequest(http.MethodGet, "/networks/json", nil, nil)
	if err != nil {
		return netList, err
	}
	return netList, response.Process(&netList)
}
