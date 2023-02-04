//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd
// +build darwin dragonfly freebsd linux netbsd openbsd

package qemu

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/containers/podman/v4/pkg/machine"
	"golang.org/x/sys/unix"
)

func isProcessAlive(pid int) bool {
	err := unix.Kill(pid, syscall.Signal(0))
	if err == nil || err == unix.EPERM {
		return true
	}
	return false
}

func checkProcessStatus(processHint string, pid int, stderrBuf *bytes.Buffer) error {
	var status syscall.WaitStatus
	pid, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
	if err != nil {
		return fmt.Errorf("failed to read %s process status: %w", processHint, err)
	}
	if pid > 0 {
		// child exited
		return fmt.Errorf("%s exited unexpectedly with exit code %d, stderr: %s", processHint, status.ExitStatus(), stderrBuf.String())
	}
	return nil
}

func forwardPipeArgs(cmd []string, name string, destPath string, identityPath string, user string) ([]string, error) {
	return cmd, nil
}

func podmanPipe(name string) *machine.VMFile {
	return nil
}

func useFdVLan() bool {
	return strings.ToUpper(os.Getenv("CONTAINERS_USE_SOCKET_VLAN")) != "TRUE"
}

func pathsFromVolume(volume string) []string {
	return strings.SplitN(volume, ":", 3)
}

func extractTargetPath(paths []string) string {
	if len(paths) > 1 {
		return paths[1]
	}
	return paths[0]
}
