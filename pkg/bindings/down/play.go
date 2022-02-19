package down

import (
	"context"
	"net/http"
	"os"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/sirupsen/logrus"
)

func KubeDown(ctx context.Context, path string) (*entities.PlayKubeReport, error) {
	// TODO: should this be entities.PlayKubeTeardown?
	var report entities.PlayKubeReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.Warn(err)
		}
	}()
	response, err := conn.DoRequest(ctx, f, http.MethodDelete, "/play/kube", nil, nil)
	if err != nil {
		return nil, err
	}
	if err := response.Process(&report); err != nil {
		return nil, err
	}

	return &report, nil
}
