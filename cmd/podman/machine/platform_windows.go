package machine

import (
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/wsl"
)

func getSystemDefaultProvider() machine.Provider {
	return wsl.GetWSLProvider()
}
