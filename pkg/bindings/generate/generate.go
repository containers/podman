package generate

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/domain/entities"
)

func GenerateKube(ctx context.Context, nameOrID string, options entities.GenerateKubeOptions) (*entities.GenerateKubeReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("service", strconv.FormatBool(options.Service))

	response, err := conn.DoRequest(nil, http.MethodGet, "/generate/%s/kube", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}

	if response.StatusCode == http.StatusOK {
		return &entities.GenerateKubeReport{Reader: response.Body}, nil
	}

	// Unpack the error.
	return nil, response.Process(nil)
}
