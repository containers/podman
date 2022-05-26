package abi

import (
	"context"
	"io"

	"github.com/containers/podman/v4/pkg/domain/entities"
)

func (ic *ContainerEngine) ContainerCopyFromArchive(ctx context.Context, nameOrID, containerPath string, reader io.Reader, options entities.CopyOptions) (entities.ContainerCopyFunc, error) {
	container, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return nil, err
	}
	return container.CopyFromArchive(ctx, containerPath, options.Chown, options.NoOverwriteDirNonDir, options.Rename, reader)
}

func (ic *ContainerEngine) ContainerCopyToArchive(ctx context.Context, nameOrID, containerPath string, writer io.Writer) (entities.ContainerCopyFunc, error) {
	container, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return nil, err
	}
	return container.CopyToArchive(ctx, containerPath, writer)
}
