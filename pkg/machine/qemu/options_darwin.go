package qemu

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containers/storage/pkg/homedir"
)

func getRuntimeDir() (string, error) {
	// Because MacOS can only support 104byte filenames, we abandon the
	// long directory names and simply use ~/.podman
	home := homedir.Get()
	podmanDir := filepath.Join(home, ".podman")
	if err := os.MkdirAll(podmanDir, 0755); err != nil {
		return "", err
	}
	return podmanDir, nil
}

func (v *MachineVM) getSocketandPid() (string, string, error) {
	rtPath, err := getRuntimeDir()
	if err != nil {
		return "", "", err
	}

	pidFile := filepath.Join(rtPath, fmt.Sprintf("%s.pid", v.Name))
	qemuSocket := filepath.Join(rtPath, fmt.Sprintf("qemu_%s.sock", v.Name))
	return qemuSocket, pidFile, nil
}

// Maintain old socket locations for compatibility, drop in 5.x
func createCompatTmpLink() error {
	rtDir, err := getRuntimeDir()
	if err != nil {
		return err
	}

	tmpDir, ok := os.LookupEnv("TMPDIR")
	if !ok {
		tmpDir = "/tmp"
	}
	tmpPodman := filepath.Join(rtDir, "podman")
	tmpPodmanLink := filepath.Join(tmpDir, "podman")
	_ = os.RemoveAll(tmpPodmanLink)

	return os.Symlink(tmpPodman, tmpPodmanLink)
}
