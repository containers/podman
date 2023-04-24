package qemu

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containers/common/pkg/config"
)

var (
	QemuCommand = "qemu-system-aarch64"
)

func (v *MachineVM) addArchOptions() []string {
	ovmfDir := getOvmfDir(v.ImagePath.GetPath(), v.Name)
	opts := []string{
		"-accel", "hvf",
		"-accel", "tcg",
		"-cpu", "host",
		"-M", "virt,highmem=on",
		"-drive", "file=" + getEdk2CodeFd("edk2-aarch64-code.fd") + ",if=pflash,format=raw,readonly=on",
		"-drive", "file=" + ovmfDir + ",if=pflash,format=raw"}
	return opts
}

func (v *MachineVM) prepare() error {
	ovmfDir := getOvmfDir(v.ImagePath.GetPath(), v.Name)
	cmd := []string{"/bin/dd", "if=/dev/zero", "conv=sync", "bs=1m", "count=64", "of=" + ovmfDir}
	return exec.Command(cmd[0], cmd[1:]...).Run()
}

func (v *MachineVM) archRemovalFiles() []string {
	ovmDir := getOvmfDir(v.ImagePath.GetPath(), v.Name)
	return []string{ovmDir}
}

func getOvmfDir(imagePath, vmName string) string {
	return filepath.Join(filepath.Dir(imagePath), vmName+"_ovmf_vars.fd")
}

/*
 * When QEmu is installed in a non-default location in the system
 * we can use the qemu-system-* binary path to figure the install
 * location for Qemu and use it to look for edk2-code-fd
 */
func getEdk2CodeFdPathFromQemuBinaryPath() string {
	cfg, err := config.Default()
	if err == nil {
		execPath, err := cfg.FindHelperBinary(QemuCommand, true)
		if err == nil {
			return filepath.Clean(filepath.Join(filepath.Dir(execPath), "..", "share", "qemu"))
		}
	}
	return ""
}

/*
 *  QEmu can be installed in multiple locations on MacOS, especially on
 *  Apple Silicon systems.  A build from source will likely install it in
 *  /usr/local/bin, whereas Homebrew package management standard is to
 *  install in /opt/homebrew
 */
func getEdk2CodeFd(name string) string {
	dirs := []string{
		getEdk2CodeFdPathFromQemuBinaryPath(),
		"/opt/homebrew/opt/podman/libexec/share/qemu",
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
