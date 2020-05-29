package containers

import (
	"context"
	"net/http"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/bindings"
)

// RunHealthCheck executes the container's healthcheck and returns the health status of the
// container.
func RunHealthCheck(ctx context.Context, nameOrID string) (*define.HealthCheckResults, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	var (
		status define.HealthCheckResults
	)
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/healthcheck", nil, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	return &status, response.Process(&status)
}
