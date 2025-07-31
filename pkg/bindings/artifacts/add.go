package artifacts

import (
	"context"
	"io"
	"net/http"

	"github.com/containers/podman/v5/pkg/bindings"
	entitiesTypes "github.com/containers/podman/v5/pkg/domain/entities/types"
)

func Add(ctx context.Context, artifactName string, blobName string, artifactBlob io.Reader, options *AddOptions) (*entitiesTypes.ArtifactAddReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	if options == nil {
		options = new(AddOptions)
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	params.Set("name", artifactName)
	params.Set("fileName", blobName)

	response, err := conn.DoRequest(ctx, artifactBlob, http.MethodPost, "/artifacts/add", params, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var artifactAddReport entitiesTypes.ArtifactAddReport
	if err := response.Process(&artifactAddReport); err != nil {
		return nil, err
	}

	return &artifactAddReport, nil
}
