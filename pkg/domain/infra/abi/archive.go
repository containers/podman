package abi

import (
	"context"
	"io"

	"github.com/containers/podman/v3/pkg/domain/entities"
)

func (ic *ContainerEngine) ContainerCopyFromArchive(ctx context.Context, nameOrID string, containerPath string, reader io.Reader) (entities.ContainerCopyFunc, error) {
	container, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return nil, err
	}
	return container.CopyFromArchive(ctx, containerPath, reader)
}

func (ic *ContainerEngine) ContainerCopyToArchive(ctx context.Context, nameOrID string, containerPath string, writer io.Writer) (entities.ContainerCopyFunc, error) {
	container, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return nil, err
	}
	return container.CopyToArchive(ctx, containerPath, writer)
}
