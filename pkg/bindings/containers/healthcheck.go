package containers

import (
	"context"
	"net/http"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/bindings"
)

// RunHealthCheck executes the container's healthcheck and returns the health status of the
// container.
func RunHealthCheck(ctx context.Context, nameOrID string) (*libpod.HealthCheckStatus, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	var (
		status libpod.HealthCheckStatus
	)
	response, err := conn.DoRequest(nil, http.MethodGet, "/containers/%s/runhealthcheck", nil, nameOrID)
	if err != nil {
		return nil, err
	}
	return &status, response.Process(&status)
}
