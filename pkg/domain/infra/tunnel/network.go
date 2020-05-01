package tunnel

import (
	"context"
	"errors"

	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) NetworkList(ctx context.Context, options entities.NetworkListOptions) ([]*entities.NetworkListReport, error) {
	return nil, errors.New("not implemented")
}

func (ic *ContainerEngine) NetworkInspect(ctx context.Context, namesOrIds []string, options entities.NetworkInspectOptions) ([]entities.NetworkInspectReport, error) {
	return nil, errors.New("not implemented")
}
func (ic *ContainerEngine) NetworkRm(ctx context.Context, namesOrIds []string, options entities.NetworkRmOptions) ([]*entities.NetworkRmReport, error) {
	return nil, errors.New("not implemented")
}

func (ic *ContainerEngine) NetworkCreate(ctx context.Context, name string, options entities.NetworkCreateOptions) (*entities.NetworkCreateReport, error) {
	return nil, errors.New("not implemented")
}
