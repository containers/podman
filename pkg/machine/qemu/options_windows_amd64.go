package qemu

var (
	QemuCommand = "qemu-system-x86_64w"
)

func (v *MachineVM) addArchOptions(_ *setNewMachineCMDOpts) []string {
	// "qemu64" level is used, because "host" is not supported with "whpx" acceleration.
	// It is a stable choice for running on bare metal and inside Hyper-V machine with nested virtualization.
	opts := []string{"-machine", "q35,accel=whpx:tcg", "-cpu", "qemu64"}
	return opts
}

func (v *MachineVM) prepare() error {
	return nil
}

func (v *MachineVM) archRemovalFiles() []string {
	return []string{}
}
