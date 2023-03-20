package machine

import (
	"os"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/hyperv"
	"github.com/containers/podman/v4/pkg/machine/wsl"
)

func GetSystemDefaultProvider() machine.VirtProvider {
	// This is a work-around for default provider on windows while
	// hyperv is one developer.
	// TODO this needs to be changed back
	if _, exists := os.LookupEnv("HYPERV"); exists {
		return hyperv.GetVirtualizationProvider()
	}
	return wsl.GetWSLProvider()
}
