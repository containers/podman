package qemu

var (
	QemuCommand = "qemu-system-x86_64w"
)

func (v *MachineVM) addArchOptions() []string {
	// "max" level is used, because "host" is not supported with "whpx" acceleration
	// "vmx=off" disabled nested virtualization (not needed for podman)
	// QEMU issue to track nested virtualization: https://gitlab.com/qemu-project/qemu/-/issues/628
	// "monitor=off" needed to support hosts, which have mwait calls disabled in BIOS/UEFI
	opts := []string{"-machine", "q35,accel=whpx:tcg", "-cpu", "max,vmx=off,monitor=off"}
	return opts
}

func (v *MachineVM) prepare() error {
	return nil
}

func (v *MachineVM) archRemovalFiles() []string {
	return []string{}
}
