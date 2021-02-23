package generate

import (
	"context"
	"errors"
	"net/http"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/domain/entities"
)

func Systemd(ctx context.Context, nameOrID string, options *SystemdOptions) (*entities.GenerateSystemdReport, error) {
	if options == nil {
		options = new(SystemdOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(nil, http.MethodGet, "/generate/%s/systemd", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	report := &entities.GenerateSystemdReport{}
	return report, response.Process(&report.Units)
}

func Kube(ctx context.Context, nameOrIDs []string, options *KubeOptions) (*entities.GenerateKubeReport, error) {
	if options == nil {
		options = new(KubeOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	if len(nameOrIDs) < 1 {
		return nil, errors.New("must provide the name or ID of one container or pod")
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	for _, name := range nameOrIDs {
		params.Add("names", name)
	}
	response, err := conn.DoRequest(nil, http.MethodGet, "/generate/kube", params, nil)
	if err != nil {
		return nil, err
	}

	if response.StatusCode == http.StatusOK {
		return &entities.GenerateKubeReport{Reader: response.Body}, nil
	}

	// Unpack the error.
	return nil, response.Process(nil)
}
