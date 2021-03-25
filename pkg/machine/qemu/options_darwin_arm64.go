package qemu

import (
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
		"-cpu", "cortex-a57",
		"-M", "virt,highmem=off",
		"-drive", "file=/usr/local/share/qemu/edk2-aarch64-code.fd,if=pflash,format=raw,readonly=on",
		"-drive", "file=" + ovmfDir + ",if=pflash,format=raw"}
	return opts
}

func (v *MachineVM) prepare() error {
	ovmfDir := getOvmfDir(v.ImagePath, v.Name)
	cmd := []string{"dd", "if=/dev/zero", "conv=sync", "bs=1m", "count=64", "of=" + ovmfDir}
	return exec.Command(cmd[0], cmd[1:]...).Run()
}

func (v *MachineVM) archRemovalFiles() []string {
	ovmDir := getOvmfDir(v.ImagePath, v.Name)
	return []string{ovmDir}
}

func getOvmfDir(imagePath, vmName string) string {
	return filepath.Join(filepath.Dir(imagePath), vmName+"_ovmf_vars.fd")
}
