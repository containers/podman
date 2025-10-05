package artifacts

import (
	"context"
	"net/http"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/domain/entities"
)

// List returns a list of artifacts in local storage.
func List(ctx context.Context, _ *ListOptions) ([]*entities.ArtifactListReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/artifacts/json", nil, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var artifactSummary []*entities.ArtifactListReport
	if err := response.Process(&artifactSummary); err != nil {
		return nil, err
	}

	return artifactSummary, nil
}
