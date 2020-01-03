package bindings

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/containernetworking/cni/libcni"
)

/*
	All methods still need error handling defined based on the http response codes.
*/

func (c Connection) CreateNetwork() {}
func (c Connection) InspectNetwork(nameOrID string) (map[string]interface{}, error) {
	n := make(map[string]interface{})
	response, err := http.Get(c.makeEndpoint(fmt.Sprintf("/networks/%s/json", nameOrID)))
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &n)
	return n, err

}

func (c Connection) RemoveNetwork(nameOrID string) error {
	client := &http.Client{}
	request, err := http.NewRequest(http.MethodDelete, c.makeEndpoint(fmt.Sprintf("/networks/%s", nameOrID)), nil)
	if err != nil {
		return err
	}
	// TODO once we have error code handling, we need to take the http response and process it for success
	_, err = client.Do(request)
	return err
}

func (c Connection) ListNetworks() ([]*libcni.NetworkConfigList, error) {
	var (
		netList []*libcni.NetworkConfigList
	)
	response, err := http.Get(c.makeEndpoint("/networks/json"))
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &netList)
	return netList, err
}
