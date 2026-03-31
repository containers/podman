//go:build freebsd && amd64

package qemu

var qemuCommand = []string{"qemu-system-x86_64"}

func (q *QEMUStubber) addArchOptions(_ *setNewMachineCMDOpts) []string {
	opts := []string{"-machine", "q35,accel=hvf:tcg", "-cpu", "host"}
	return opts
}
