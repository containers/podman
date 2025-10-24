package main

import (
	"errors"
	"path/filepath"

	"github.com/containers/podman/v6/pkg/machine/define"
)

func extractConnectionString(_ *define.VMFile, podmanPipe *define.VMFile) (string, error) {
	if podmanPipe == nil {
		return "", errors.New("pipe of machine is not set")
	}
	return "npipe://" + filepath.ToSlash(podmanPipe.Path), nil
}
