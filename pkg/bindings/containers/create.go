package containers

import (
	"context"
	"net/http"
	"strings"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/specgen"
	jsoniter "github.com/json-iterator/go"
)

func CreateWithSpec(ctx context.Context, s *specgen.SpecGenerator, _ *CreateOptions) (entities.ContainerCreateResponse, error) {
	var ccr entities.ContainerCreateResponse
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return ccr, err
	}
	specgenString, err := jsoniter.MarshalToString(s)
	if err != nil {
		return ccr, err
	}
	stringReader := strings.NewReader(specgenString)
	response, err := conn.DoRequest(ctx, stringReader, http.MethodPost, "/containers/create", nil, nil)
	if err != nil {
		return ccr, err
	}
	defer response.Body.Close()

	return ccr, response.Process(&ccr)
}
