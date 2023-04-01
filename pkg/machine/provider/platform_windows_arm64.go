package provider

import (
	"fmt"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/define"
)

func getQemuProvider() (machine.VirtProvider, error) {
	return nil, fmt.Errorf("unsupported virtualization provider: `%s`", define.QemuVirt.String())
}
