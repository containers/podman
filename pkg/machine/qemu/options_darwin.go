package qemu

import (
	"fmt"
	"os/user"
	"path/filepath"
)

func getRuntimeDir() (string, error) {
	// Because MacOS can only support 104byte filenames, we abandon the
	// long directory names and simply use ~/.podman
	systemUser, err := user.Current()
	if err != nil {
		return "", err
	}
	return systemUser.HomeDir, nil
}

func (v *MachineVM) getSocketandPid() (string, string, error) {
	rtPath, err := getRuntimeDir()
	if err != nil {
		return "", "", err
	}
	socketDir := filepath.Join(rtPath, ".podman")
	pidFile := filepath.Join(socketDir, fmt.Sprintf("%s.pid", v.Name))
	qemuSocket := filepath.Join(socketDir, fmt.Sprintf("qemu_%s.pid", v.Name))
	return qemuSocket, pidFile, nil
}
