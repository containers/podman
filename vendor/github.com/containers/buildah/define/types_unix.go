// +build darwin linux

package define

import (
	"github.com/opencontainers/runc/libcontainer/devices"
)

type ContainerDevices = []devices.Device
