package qemu

var (
	QemuCommand = "qemu-system-x86_64"
)

func (v *MachineVM) addArchOptions() []string {
	opts := []string{"-cpu", "host", "-accel", "hvf"}
	return opts
}

func (v *MachineVM) prepare() error {
	return nil
}

func (v *MachineVM) archRemovalFiles() []string {
	return []string{}
}
