package main

import (
	"errors"
	"path/filepath"

	"github.com/containers/podman/v5/pkg/machine/define"
)

func extractConnectionString(podmanSocket *define.VMFile, podmanPipe *define.VMFile) (string, error) {
	if podmanPipe == nil {
		return "", errors.New("pipe of machine is not set")
	}
	return "npipe://" + filepath.ToSlash(podmanPipe.Path), nil
}
