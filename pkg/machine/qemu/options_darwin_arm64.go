package qemu

import (
	"os"
	"os/exec"
	"path/filepath"
)

var (
	QemuCommand = "qemu-system-aarch64"
)

func (v *MachineVM) addArchOptions() []string {
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

func (v *MachineVM) prepare() error {
	ovmfDir := getOvmfDir(v.ImagePath, v.Name)
	cmd := []string{"/bin/dd", "if=/dev/zero", "conv=sync", "bs=1m", "count=64", "of=" + ovmfDir}
	return exec.Command(cmd[0], cmd[1:]...).Run()
}

func (v *MachineVM) archRemovalFiles() []string {
	ovmDir := getOvmfDir(v.ImagePath, v.Name)
	return []string{ovmDir}
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
