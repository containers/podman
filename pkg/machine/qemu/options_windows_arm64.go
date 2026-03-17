//go:build windows && arm64

package qemu

var qemuCommand = []string{"qemu-system-aarch64w"}

func (q *QEMUStubber) addArchOptions(_ *setNewMachineCMDOpts) []string {
	// stub to fix compilation issues
	opts := []string{}
	return opts
}
