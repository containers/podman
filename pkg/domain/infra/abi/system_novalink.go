// +build !varlink

package abi

import (
	"context"

	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
)

func (ic *ContainerEngine) VarlinkService(_ context.Context, opts entities.ServiceOptions) error {
	return errors.Errorf("varlink is not supported")
}
