//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package qemu

import (
	"bytes"
	"fmt"
	"strings"
	"syscall"

	"github.com/containers/podman/v4/pkg/machine/define"
	"golang.org/x/sys/unix"
)

func isProcessAlive(pid int) (bool, error) {
	err := unix.Kill(pid, syscall.Signal(0))
	if err == nil || err == unix.EPERM {
		return true, nil
	}
	return false, err
}

func pingProcess(pid int) (int, error) {
	alive, err := isProcessAlive(pid)
	if !alive {
		if err == unix.ESRCH {
			return -1, nil
		}
		return -1, fmt.Errorf("pinging QEMU process: %w", err)
	}
	return pid, nil
}

func killProcess(pid int, force bool) error {
	if force {
		return unix.Kill(pid, unix.SIGKILL)
	}
	return unix.Kill(pid, unix.SIGTERM)
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

func podmanPipe(name string) *define.VMFile {
	return nil
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
