// +build !linux,!darwin

package parse

import (
	"github.com/containers/buildah"
	"github.com/pkg/errors"
)

func getDefaultProcessLimits() []string {
	return []string{}
}

func DeviceFromPath(device string) (buildah.ContainerDevices, error) {
	return buildah.ContainerDevices{}, errors.Errorf("devices not supported")
}
