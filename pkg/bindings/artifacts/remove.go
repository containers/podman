package artifacts

import (
	"context"
	"net/http"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/domain/entities"
)

// Remove removes an artifact from local storage.
func Remove(ctx context.Context, nameOrID string, options *RemoveOptions) (*entities.ArtifactRemoveReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodDelete, "/artifacts/%s", nil, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var artifactRemoveReport entities.ArtifactRemoveReport
	if err := response.Process(&artifactRemoveReport); err != nil {
		return nil, err
	}

	return &artifactRemoveReport, nil
}
