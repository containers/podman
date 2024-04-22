package containers

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	jsoniter "github.com/json-iterator/go"
)

func Update(ctx context.Context, options *types.ContainerUpdateOptions) (string, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}

	params := url.Values{}
	if options.Specgen.RestartPolicy != "" {
		params.Set("restartPolicy", options.Specgen.RestartPolicy)
		if options.Specgen.RestartRetries != nil {
			params.Set("restartRetries", strconv.Itoa(int(*options.Specgen.RestartRetries)))
		}
	}

	resources, err := jsoniter.MarshalToString(options.Specgen.ResourceLimits)
	if err != nil {
		return "", err
	}
	stringReader := strings.NewReader(resources)
	response, err := conn.DoRequest(ctx, stringReader, http.MethodPost, "/containers/%s/update", params, nil, options.NameOrID)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return options.NameOrID, response.Process(nil)
}
