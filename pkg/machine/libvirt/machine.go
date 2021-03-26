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
