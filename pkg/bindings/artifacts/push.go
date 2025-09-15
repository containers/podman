package artifacts

import (
	"context"
	"net/http"

	imageTypes "github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/pkg/auth"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/domain/entities"
)

func Push(ctx context.Context, name string, options *PushOptions) (*entities.ArtifactPushReport, error) {
	if options == nil {
		options = new(PushOptions)
	}

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	header, err := auth.MakeXRegistryAuthHeader(&imageTypes.SystemContext{AuthFilePath: options.GetAuthfile()}, options.GetUsername(), options.GetPassword())
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/artifacts/%s/push", params, header, name)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var report entities.ArtifactPushReport
	if err := response.Process(&report); err != nil {
		return nil, err
	}

	return &report, nil
}
