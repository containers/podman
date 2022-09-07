package qemu

import (
	"bytes"
	"fmt"

	"github.com/containers/podman/v4/pkg/machine"
)

func isProcessAlive(pid int) bool {
	if checkProcessStatus("process", pid, nil) == nil {
		return true
	}
	return false
}

func checkProcessStatus(processHint string, pid int, stderrBuf *bytes.Buffer) error {
	active, exitCode := machine.GetProcessState(pid)
	if !active {
		if stderrBuf != nil {
			return fmt.Errorf("%s exited unexpectedly, exit code: %d stderr: %s", processHint, exitCode, stderrBuf.String())
		} else {
			return fmt.Errorf("%s exited unexpectedly, exit code: %d", processHint, exitCode)
		}
	}
	return nil
}
