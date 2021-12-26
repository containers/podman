package machine

import (
	"github.com/containers/podman/v3/pkg/machine"
	"github.com/containers/podman/v3/pkg/machine/wsl"
)

func getSystemDefaultProvider() machine.Provider {
	return wsl.GetWSLProvider()
}
