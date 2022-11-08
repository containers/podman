package kube

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/pkg/auth"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/generate"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/sirupsen/logrus"
)

func Play(ctx context.Context, path string, options *PlayOptions) (*entities.KubePlayReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return PlayWithBody(ctx, f, options)
}

func PlayWithBody(ctx context.Context, body io.Reader, options *PlayOptions) (*entities.KubePlayReport, error) {
	var report entities.KubePlayReport
	if options == nil {
		options = new(PlayOptions)
	}

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	// SkipTLSVerify is special.  It's not being serialized by ToParams()
	// because we need to flip the boolean.
	if options.SkipTLSVerify != nil {
		params.Set("tlsVerify", strconv.FormatBool(!options.GetSkipTLSVerify()))
	}
	if options.Start != nil {
		params.Set("start", strconv.FormatBool(options.GetStart()))
	}

	header, err := auth.MakeXRegistryAuthHeader(&types.SystemContext{AuthFilePath: options.GetAuthfile()}, options.GetUsername(), options.GetPassword())
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, body, http.MethodPost, "/play/kube", params, header)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if err := response.Process(&report); err != nil {
		return nil, err
	}

	return &report, nil
}

func Down(ctx context.Context, path string, options DownOptions) (*entities.KubePlayReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.Warn(err)
		}
	}()

	return DownWithBody(ctx, f, options)
}

func DownWithBody(ctx context.Context, body io.Reader, options DownOptions) (*entities.KubePlayReport, error) {
	var report entities.KubePlayReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, body, http.MethodDelete, "/play/kube", params, nil)
	if err != nil {
		return nil, err
	}
	if err := response.Process(&report); err != nil {
		return nil, err
	}
	return &report, nil
}

// Kube generate Kubernetes YAML (v1 specification)
func Generate(ctx context.Context, nameOrIDs []string, options generate.KubeOptions) (*entities.GenerateKubeReport, error) {
	return generate.Kube(ctx, nameOrIDs, &options)
}

func Apply(ctx context.Context, path string, options *ApplyOptions) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.Warn(err)
		}
	}()

	return ApplyWithBody(ctx, f, options)
}

func ApplyWithBody(ctx context.Context, body io.Reader, options *ApplyOptions) error {
	if options == nil {
		options = new(ApplyOptions)
	}

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	params, err := options.ToParams()
	if err != nil {
		return err
	}

	response, err := conn.DoRequest(ctx, body, http.MethodPost, "/kube/apply", params, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return nil
}
