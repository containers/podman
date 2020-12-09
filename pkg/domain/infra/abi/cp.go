package abi

import (
	"context"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/copy"
	"github.com/containers/podman/v2/pkg/domain/entities"
)

func (ic *ContainerEngine) ContainerCp(ctx context.Context, source, dest string, options entities.ContainerCpOptions) error {
	// Parse user input.
	sourceContainerStr, sourcePath, destContainerStr, destPath, err := copy.ParseSourceAndDestination(source, dest)
	if err != nil {
		return err
	}

	// Look up containers.
	var sourceContainer, destContainer *libpod.Container
	if len(sourceContainerStr) > 0 {
		sourceContainer, err = ic.Libpod.LookupContainer(sourceContainerStr)
		if err != nil {
			return err
		}
	}
	if len(destContainerStr) > 0 {
		destContainer, err = ic.Libpod.LookupContainer(destContainerStr)
		if err != nil {
			return err
		}
	}

	var sourceItem, destinationItem copy.CopyItem

	// Source ... container OR host.
	if sourceContainer != nil {
		sourceItem, err = copy.CopyItemForContainer(sourceContainer, sourcePath, options.Pause, true)
		defer sourceItem.CleanUp()
		if err != nil {
			return err
		}
	} else {
		sourceItem, err = copy.CopyItemForHost(sourcePath, true)
		if err != nil {
			return err
		}
	}

	// Destination ... container OR host.
	if destContainer != nil {
		destinationItem, err = copy.CopyItemForContainer(destContainer, destPath, options.Pause, false)
		defer destinationItem.CleanUp()
		if err != nil {
			return err
		}
	} else {
		destinationItem, err = copy.CopyItemForHost(destPath, false)
		defer destinationItem.CleanUp()
		if err != nil {
			return err
		}
	}

	// Copy from the host to the container.
	copier, err := copy.GetCopier(&sourceItem, &destinationItem, options.Extract)
	if err != nil {
		return err
	}
	return copier.Copy()
}
