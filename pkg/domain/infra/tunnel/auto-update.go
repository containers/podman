package tunnel

import (
	"context"
	"errors"

	"github.com/containers/podman/v4/pkg/domain/entities"
)

func (ic *ContainerEngine) AutoUpdate(ctx context.Context, options entities.AutoUpdateOptions) ([]*entities.AutoUpdateReport, []error) {
	return nil, []error{errors.New("not implemented")}
}
