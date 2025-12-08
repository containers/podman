//go:build !darwin

package qemu

import (
	"errors"
	"io/fs"

	"go.podman.io/common/pkg/config"
)

// setNewMachineCMDOpts are options needed to pass
// into setting up the qemu command line.  long term, this need
// should be eliminated
// TODO Podman5
type setNewMachineCMDOpts struct{}

// FindQEMUBinary locates and returns the QEMU binary
func FindQEMUBinary() (string, error) {
	cfg, err := config.Default()
	if err != nil {
		return "", err
	}

	path, err := cfg.FindHelperBinary(QemuCommand, true)
	if errors.Is(err, fs.ErrNotExist) {
		// if the qemu-system-<arch> binary doesn't exist, check if we have the arch
		// agnostic binary installed
		return cfg.FindHelperBinary(GenericQemuCommand, true)
	}
	return path, err
}
