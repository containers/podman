package play

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/containers/podman/v2/pkg/auth"
	"github.com/containers/podman/v2/pkg/bindings"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
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

	var f *os.File

	if path == "-" {
		fi, err := os.Stdin.Stat()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to read from stdin")
		}

		if fi.Mode()&os.ModeNamedPipe == 0 {
			return nil, errors.New("No data is supplied by a shell pipe but '-' was used as filename")
		}

		f = os.Stdin

	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
	}

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
	if err := response.Process(&report); err != nil {
		return nil, err
	}

	return &report, nil
}
