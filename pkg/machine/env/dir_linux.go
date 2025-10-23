package env

import (
	"github.com/containers/podman/v6/pkg/rootless"
	"github.com/containers/podman/v6/pkg/util"
)

func getRuntimeDir() (string, error) {
	if !rootless.IsRootless() {
		return "/run", nil
	}
	return util.GetRootlessRuntimeDir()
}
