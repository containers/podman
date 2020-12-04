package abi

import (
	"context"
	"strings"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/copy"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
)

func (ic *ContainerEngine) ContainerCp(ctx context.Context, source, dest string, options entities.ContainerCpOptions) error {
	srcCtr, srcPath := parsePath(ic.Libpod, source)
	destCtr, destPath := parsePath(ic.Libpod, dest)

	if srcCtr != nil && destCtr != nil {
		return errors.Errorf("invalid arguments %q, %q: you must use just one container", source, dest)
	}
	if srcCtr == nil && destCtr == nil {
		return errors.Errorf("invalid arguments %q, %q: you must specify one container", source, dest)
	}
	if len(srcPath) == 0 || len(destPath) == 0 {
		return errors.Errorf("invalid arguments %q, %q: you must specify paths", source, dest)
	}

	var sourceItem, destinationItem copy.CopyItem
	var err error
	// Copy from the container to the host.
	if srcCtr != nil {
		sourceItem, err = copy.CopyItemForContainer(srcCtr, srcPath, options.Pause, true)
		defer sourceItem.CleanUp()
		if err != nil {
			return err
		}
	} else {
		sourceItem, err = copy.CopyItemForHost(srcPath, true)
		if err != nil {
			return err
		}
	}

	if destCtr != nil {
		destinationItem, err = copy.CopyItemForContainer(destCtr, destPath, options.Pause, false)
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
	return copy.Copy(&sourceItem, &destinationItem, options.Extract)
}

func parsePath(runtime *libpod.Runtime, path string) (*libpod.Container, string) {
	if len(path) == 0 {
		return nil, ""
	}
	if path[0] == '.' || path[0] == '/' { // A path cannot point to a container.
		return nil, path
	}
	pathArr := strings.SplitN(path, ":", 2)
	if len(pathArr) == 2 {
		ctr, err := runtime.LookupContainer(pathArr[0])
		if err == nil {
			return ctr, pathArr[1]
		}
	}
	return nil, path
}
