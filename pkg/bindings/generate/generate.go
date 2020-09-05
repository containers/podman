package generate

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/containers/podman/v2/pkg/bindings"
	"github.com/containers/podman/v2/pkg/domain/entities"
)

func Systemd(ctx context.Context, nameOrID string, options entities.GenerateSystemdOptions) (*entities.GenerateSystemdReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params := url.Values{}

	params.Set("useName", strconv.FormatBool(options.Name))
	params.Set("new", strconv.FormatBool(options.New))
	if options.RestartPolicy != "" {
		params.Set("restartPolicy", options.RestartPolicy)
	}
	if options.StopTimeout != nil {
		params.Set("stopTimeout", strconv.FormatUint(uint64(*options.StopTimeout), 10))
	}
	params.Set("containerPrefix", options.ContainerPrefix)
	params.Set("podPrefix", options.PodPrefix)
	params.Set("separator", options.Separator)

	response, err := conn.DoRequest(nil, http.MethodGet, "/generate/%s/systemd", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	report := &entities.GenerateSystemdReport{}
	return report, response.Process(&report.Units)
}

func Kube(ctx context.Context, nameOrID string, options entities.GenerateKubeOptions) (*entities.GenerateKubeReport, error) {
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
