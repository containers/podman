package qemu

var (
	QemuCommand = "qemu-system-x86_64"
)

func (q *QEMUStubber) addArchOptions(_ *setNewMachineCMDOpts) []string {
	opts := []string{
		"-accel", "kvm",
		"-cpu", "host",
	}
	return opts
}
