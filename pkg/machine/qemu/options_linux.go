package qemu

import "github.com/containers/podman/v3/pkg/util"

func getSocketDir() (string, error) {
	return util.GetRuntimeDir()
}
