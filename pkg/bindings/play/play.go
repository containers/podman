package play

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/pkg/auth"
	"github.com/containers/podman/v2/pkg/bindings"
	"github.com/containers/podman/v2/pkg/domain/entities"
)

func Kube(ctx context.Context, path string, options entities.PlayKubeOptions) (*entities.PlayKubeReport, error) {
	var report entities.PlayKubeReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	params := url.Values{}
	params.Set("network", options.Network)
	params.Set("logDriver", options.LogDriver)
	if options.SkipTLSVerify != types.OptionalBoolUndefined {
		params.Set("tlsVerify", strconv.FormatBool(options.SkipTLSVerify != types.OptionalBoolTrue))
	}
	if options.Start != types.OptionalBoolUndefined {
		params.Set("start", strconv.FormatBool(options.Start == types.OptionalBoolTrue))
	}

	// TODO: have a global system context we can pass around (1st argument)
	header, err := auth.Header(nil, auth.XRegistryAuthHeader, options.Authfile, options.Username, options.Password)
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
