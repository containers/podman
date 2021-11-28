package qemu

import (
	"fmt"
	"os"
	"path/filepath"
)

func (v *MachineVM) addArchOptions(fromArch string, toArch string) []string {
	// both host and the vm are amd64
	if fromArch == "amd64" && fromArch == toArch {
		opts := []string{"-machine", "q35,accel=hvf:tcg", "-cpu", "host"}
		return opts
	}

	// both host and the vm are arm64
	if fromArch == "arm64" && fromArch == toArch {
		ovmfDir := getOvmfDir(v.ImagePath, v.Name)
		opts := []string{
			"-accel", "hvf",
			"-accel", "tcg",
			"-cpu", "cortex-a57",
			"-M", "virt,highmem=off",
			"-drive", "file=" + getEdk2CodeFd("edk2-aarch64-code.fd") + ",if=pflash,format=raw,readonly=on",
			"-drive", "file=" + ovmfDir + ",if=pflash,format=raw"}
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
	tmpDir, ok := os.LookupEnv("TMPDIR")
	if !ok {
		tmpDir = "/tmp"
	}
	return tmpDir, nil
}

func getOvmfDir(imagePath, vmName string) string {
	return filepath.Join(filepath.Dir(imagePath), vmName+"_ovmf_vars.fd")
}

/*
 *  QEmu can be installed in multiple locations on MacOS, especially on
 *  Apple Silicon systems.  A build from source will likely install it in
 *  /usr/local/bin, whereas Homebrew package management standard is to
 *  install in /opt/homebrew
 */
func getEdk2CodeFd(name string) string {
	dirs := []string{
		"/usr/local/share/qemu",
		"/opt/homebrew/share/qemu",
	}
	for _, dir := range dirs {
		fullpath := filepath.Join(dir, name)
		if _, err := os.Stat(fullpath); err == nil {
			return fullpath
		}
	}
	return name
}
