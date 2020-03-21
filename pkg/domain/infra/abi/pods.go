// +build ABISupport

package abi

import (
	"context"
	"github.com/pkg/errors"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
)

func (ic *ContainerEngine) PodExists(ctx context.Context, nameOrId string) (*entities.BoolReport, error) {
	_, err := ic.Libpod.LookupPod(nameOrId)
	if err != nil && errors.Cause(err) != define.ErrNoSuchPod {
		return nil, err
	}
	return &entities.BoolReport{Value: err == nil}, nil
}
