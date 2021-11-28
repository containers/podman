package qemu

var (
	QemuCommand = "qemu-system-x86_64"
)

func (v *MachineVM) prepare() error {
	return nil
}

func (v *MachineVM) archRemovalFiles() []string {
	return []string{}
}
