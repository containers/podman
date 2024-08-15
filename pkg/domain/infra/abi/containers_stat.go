//go:build !remote

package abi

import (
	"context"

	"github.com/containers/podman/v5/pkg/domain/entities"
)

func (ic *ContainerEngine) ContainerStat(ctx context.Context, nameOrID string, containerPath string) (*entities.ContainerStatReport, error) {
	container, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return nil, err
	}

	info, err := container.Stat(ctx, containerPath)

	if info != nil {
		return &entities.ContainerStatReport{FileInfo: *info}, err
	}
	return nil, err
}
