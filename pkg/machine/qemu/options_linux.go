package qemu

import (
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/util"
)

func (v *MachineVM) addArchOptions(fromArch string, toArch string) []string {
	// both host and the vm are amd64
	if fromArch == "amd64" && fromArch == toArch {
		opts := []string{
			"-accel", "kvm",
			"-cpu", "host",
		}
		return opts
	}

	// both host and the vm are arm64
	if fromArch == "arm64" && fromArch == toArch {
		opts := []string{
			"-accel", "kvm",
			"-cpu", "host",
			"-M", "virt,gic-version=max",
			"-bios", getQemuUefiFile("QEMU_EFI.fd"),
		}
		return opts
	}

	if fromArch == "amd64" && toArch == "arm64" {
		// TODO
	}

	if fromArch == "arm64" && toArch == "amd64" {
		// TODO
	}

	panic(fmt.Sprintf("unsupported combination of host and VM architectures: host: %s, vm: %s", fromArch, toArch))
}

func getRuntimeDir() (string, error) {
	if !rootless.IsRootless() {
		return "/run", nil
	}
	return util.GetRuntimeDir()
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
