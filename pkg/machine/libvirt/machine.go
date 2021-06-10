// +build amd64,linux arm64,linux amd64,darwin arm64,darwin

package libvirt

import "github.com/containers/podman/v3/pkg/machine"

func (v *MachineVM) Init(name string, opts machine.InitOptions) error {
	return nil
}

func (v *MachineVM) Start(name string) error {
	return nil
}

func (v *MachineVM) Stop(name string) error {
	return nil
}
