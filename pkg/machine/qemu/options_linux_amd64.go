package qemu

var (
	QemuCommand = "qemu-kvm"
)

func (v *MachineVM) addArchOptions() []string {
	opts := []string{"-cpu", "host"}
	return opts
}

func (v *MachineVM) prepare() error {
	return nil
}

func (v *MachineVM) archRemovalFiles() []string {
	return []string{}
}
