package containers

import (
	"context"
	"net/http"
	"strings"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/domain/entities"
	jsoniter "github.com/json-iterator/go"
)

func Update(ctx context.Context, options *entities.ContainerUpdateOptions) (string, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}

	resources, err := jsoniter.MarshalToString(options.Specgen.ResourceLimits)
	if err != nil {
		return "", err
	}
	stringReader := strings.NewReader(resources)
	response, err := conn.DoRequest(ctx, stringReader, http.MethodPost, "/containers/%s/update", nil, nil, options.NameOrID)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	return options.NameOrID, response.Process(nil)
}
