package artifacts

import (
	"context"
	"net/http"

	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
)

func Inspect(ctx context.Context, nameOrID string, _ *InspectOptions) (*types.ArtifactInspectReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/artifacts/%s/json", nil, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var inspectedData types.ArtifactInspectReport
	if err := response.Process(&inspectedData); err != nil {
		return nil, err
	}

	return &inspectedData, nil
}
