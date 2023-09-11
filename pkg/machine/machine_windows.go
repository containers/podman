//go:build windows
// +build windows

package machine

import (
	"os"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

func GetProcessState(pid int) (active bool, exitCode int) {
	const da = syscall.STANDARD_RIGHTS_READ | syscall.PROCESS_QUERY_INFORMATION | syscall.SYNCHRONIZE
	handle, err := syscall.OpenProcess(da, false, uint32(pid))
	if err != nil {
		logrus.Debugf("Error retrieving process %d: %v", pid, err)
		return false, int(syscall.ERROR_PROC_NOT_FOUND)
	}

	var code uint32
	if err := syscall.GetExitCodeProcess(handle, &code); err != nil {
		logrus.Errorf("Error retrieving process %d exit code: %v", pid, err)
	}
	return code == 259, int(code)
}

func PipeNameAvailable(pipeName string) bool {
	_, err := os.Stat(`\\.\pipe\` + pipeName)
	return os.IsNotExist(err)
}

func WaitPipeExists(pipeName string, retries int, checkFailure func() error) error {
	var err error
	for i := 0; i < retries; i++ {
		_, err = os.Stat(`\\.\pipe\` + pipeName)
		if err == nil {
			break
		}
		if fail := checkFailure(); fail != nil {
			return fail
		}
		time.Sleep(250 * time.Millisecond)
	}

	return err
}
