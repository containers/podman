package tunnel

import (
	"context"

	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
)

func (ic *ContainerEngine) AutoUpdate(ctx context.Context) (*entities.AutoUpdateReport, []error) {
	return nil, []error{errors.New("not implemented")}
}
