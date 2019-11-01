package bindings

import (
	"fmt"
	"net/http"

	"github.com/containers/libpod/libpod"
)

func (c Connection) RunHealthCheck(nameOrID string) (*libpod.HealthCheckStatus, error) {
	var (
		status libpod.HealthCheckStatus
	)
	response, err := c.newRequest(http.MethodGet, fmt.Sprintf("/containers/%s/runhealthcheck", nameOrID), nil, nil)
	if err != nil {
		return nil, err
	}
	return &status, response.Process(&status)
}
