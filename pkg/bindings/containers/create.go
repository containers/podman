package containers

import (
	"context"
	"net/http"
	"strings"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/specgen"
	jsoniter "github.com/json-iterator/go"
)

func CreateWithSpec(ctx context.Context, s *specgen.SpecGenerator, options *CreateOptions) (entities.ContainerCreateResponse, error) {
	var ccr entities.ContainerCreateResponse
	if options == nil {
		options = new(CreateOptions)
	}
	_ = options
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return ccr, err
	}
	specgenString, err := jsoniter.MarshalToString(s)
	if err != nil {
		return ccr, err
	}
	stringReader := strings.NewReader(specgenString)
	response, err := conn.DoRequest(stringReader, http.MethodPost, "/containers/create", nil, nil)
	if err != nil {
		return ccr, err
	}
	return ccr, response.Process(&ccr)
}
