package play

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"

	"github.com/containers/podman/v3/pkg/auth"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/domain/entities"
)

func Kube(ctx context.Context, path string, options *KubeOptions) (*entities.PlayKubeReport, error) {
	var report entities.PlayKubeReport
	if options == nil {
		options = new(KubeOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	if options.SkipTLSVerify != nil {
		params.Set("tlsVerify", strconv.FormatBool(options.GetSkipTLSVerify()))
	}
	if options.Start != nil {
		params.Set("start", strconv.FormatBool(options.GetStart()))
	}

	// TODO: have a global system context we can pass around (1st argument)
	header, err := auth.Header(nil, auth.XRegistryAuthHeader, options.GetAuthfile(), options.GetUsername(), options.GetPassword())
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(f, http.MethodPost, "/play/kube", params, header)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if err := response.Process(&report); err != nil {
		return nil, err
	}

	return &report, nil
}

func KubeDown(ctx context.Context, path string) (*entities.PlayKubeReport, error) {
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
	response, err := conn.DoRequest(f, http.MethodDelete, "/play/kube", nil, nil)
	if err != nil {
		return nil, err
	}
	if err := response.Process(&report); err != nil {
		return nil, err
	}

	return &report, nil
}
