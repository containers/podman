// +build !linux,!darwin

package parse

import (
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/pkg/errors"
)

func getDefaultProcessLimits() []string {
	return []string{}
}

func DeviceFromPath(device string) ([]configs.Device, error) {
	return []configs.Device{}, errors.Errorf("devices not supported")
}
