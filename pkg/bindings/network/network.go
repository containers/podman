package network

import (
	"context"
	"net/http"

	"github.com/containernetworking/cni/libcni"
	"github.com/containers/libpod/pkg/bindings"
)

func Create() {}
func Inspect(ctx context.Context, nameOrID string) (map[string]interface{}, error) {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	n := make(map[string]interface{})
	response, err := conn.DoRequest(nil, http.MethodGet, "/networks/%s/json", nil, nameOrID)
	if err != nil {
		return n, err
	}
	return n, response.Process(&n)
}

func Remove(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(nil, http.MethodDelete, "/networks/%s", nil, nameOrID)
	if err != nil {
		return err
	}
	return response.Process(nil)
}

func List(ctx context.Context) ([]*libcni.NetworkConfigList, error) {
	var (
		netList []*libcni.NetworkConfigList
	)
	conn, err := bindings.GetConnectionFromContext(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/networks/json", nil)
	if err != nil {
		return netList, err
	}
	return netList, response.Process(&netList)
}
