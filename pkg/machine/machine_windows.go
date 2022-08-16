//go:build windows
// +build windows

package machine

import (
	"syscall"
)

func GetProcessState(pid int) (active bool, exitCode int) {
	const da = syscall.STANDARD_RIGHTS_READ | syscall.PROCESS_QUERY_INFORMATION | syscall.SYNCHRONIZE
	handle, err := syscall.OpenProcess(da, false, uint32(pid))
	if err != nil {
		return false, int(syscall.ERROR_PROC_NOT_FOUND)
	}

	var code uint32
	syscall.GetExitCodeProcess(handle, &code)
	return code == 259, int(code)
}
