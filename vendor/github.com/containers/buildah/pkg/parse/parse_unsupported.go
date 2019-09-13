// +build !linux,!darwin

package parse

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func getDefaultProcessLimits() []string {
	return []string{}
}

func DeviceFromPath(device string) (configs.Device, error) {
	return configs.Device{}, fmt.Errorf("devices not supported")
}
