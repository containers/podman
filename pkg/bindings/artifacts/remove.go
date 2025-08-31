package artifacts

import (
	"context"
	"net/http"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/domain/entities"
)

// Remove removes an artifact from local storage.
// TODO (6.0): nameOrID parameter should be removed
func Remove(ctx context.Context, nameOrID string, options *RemoveOptions) (*entities.ArtifactRemoveReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	if nameOrID != "" {
		options.Artifacts = append(options.Artifacts, nameOrID)
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodDelete, "/artifacts/remove", params, nil)
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
