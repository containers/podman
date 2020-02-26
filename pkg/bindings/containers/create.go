package containers

import (
	"context"
	"net/http"
	"strings"

	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/libpod/pkg/bindings"
	"github.com/containers/libpod/pkg/specgen"
	jsoniter "github.com/json-iterator/go"
)

func CreateWithSpec(ctx context.Context, s *specgen.SpecGenerator) (utils.ContainerCreateResponse, error) {
	var ccr utils.ContainerCreateResponse
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return ccr, err
	}
	specgenString, err := jsoniter.MarshalToString(s)
	if err != nil {
		return ccr, err
	}
	stringReader := strings.NewReader(specgenString)
	response, err := conn.DoRequest(stringReader, http.MethodPost, "/containers/create", nil)
	if err != nil {
		return ccr, err
	}
	return ccr, response.Process(&ccr)
}
