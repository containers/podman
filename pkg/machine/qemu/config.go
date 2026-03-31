//go:build !darwin

package qemu

import (
	"fmt"

	"go.podman.io/common/pkg/config"
)

// setNewMachineCMDOpts are options needed to pass
// into setting up the qemu command line.  long term, this need
// should be eliminated
// TODO Podman5
type setNewMachineCMDOpts struct{}

// FindQEMUBinary locates and returns the QEMU binary by trying each name
// in qemuCommand in order. On most distros the first entry (e.g.
// "qemu-system-x86_64") will match; on RHEL/CentOS the binary is packaged
// as "qemu-kvm" instead, which is listed as a later entry.
func FindQEMUBinary() (string, error) {
	cfg, err := config.Default()
	if err != nil {
		return "", err
	}
	for _, name := range qemuCommand {
		if binary, e := cfg.FindHelperBinary(name, true); e == nil {
			return binary, nil
		}
	}
	return "", fmt.Errorf("unable to find any QEMU binary: tried %v", qemuCommand)
}
