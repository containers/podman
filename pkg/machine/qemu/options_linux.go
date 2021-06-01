package qemu

import (
	"github.com/containers/podman/v3/pkg/rootless"
	"github.com/containers/podman/v3/pkg/util"
)

func getRuntimeDir() (string, error) {
	if !rootless.IsRootless() {
		return "/run", nil
	}
	return util.GetRuntimeDir()
}
