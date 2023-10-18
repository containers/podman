package qemu

import (
	"os"
	"path/filepath"
)

var (
	QemuCommand = "qemu-system-aarch64"
)

func (v *MachineVM) addArchOptions(_ *setNewMachineCMDOpts) []string {
	opts := []string{
		"-accel", "kvm",
		"-cpu", "host",
		"-M", "virt,gic-version=max",
		"-bios", getQemuUefiFile("QEMU_EFI.fd"),
	}
	return opts
}

func (v *MachineVM) prepare() error {
	return nil
}

func (v *MachineVM) archRemovalFiles() []string {
	return []string{}
}

func getQemuUefiFile(name string) string {
	dirs := []string{
		"/usr/share/qemu-efi-aarch64",
		"/usr/share/edk2/aarch64",
	}
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			return filepath.Join(dir, name)
		}
	}
	return name
}
