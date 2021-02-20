package system

import (
	"context"
	"net/http"

	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/bindings"
)

// Info returns information about the libpod environment and its stores
func Info(ctx context.Context, options *InfoOptions) (*define.Info, error) {
	if options == nil {
		options = new(InfoOptions)
	}
	_ = options
	info := define.Info{}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/info", nil, nil)
	if err != nil {
		return nil, err
	}
	return &info, response.Process(&info)
}
