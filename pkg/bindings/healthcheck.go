package bindings

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/containers/libpod/libpod"
)

/*
	All methods still need error handling defined based on the http response codes.
*/

func (c Connection) RunHealthCheck(nameOrID string) (*libpod.HealthCheckStatus, error) {
	var (
		status libpod.HealthCheckStatus
	)
	response, err := http.Get(c.makeEndpoint(fmt.Sprintf("/containers/%s/runhealthcheck", nameOrID)))
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &status)
	return &status, err
}
