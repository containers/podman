// +build !linux,!darwin

package parse

import (
	"github.com/containers/buildah/define"
	"github.com/pkg/errors"
)

func getDefaultProcessLimits() []string {
	return []string{}
}

func DeviceFromPath(device string) (define.ContainerDevices, error) {
	return nil, errors.Errorf("devices not supported")
}
