package machine

import (
	"fmt"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

const (
	loops     = 5
	sleepTime = time.Millisecond * 1
)

// backoffForProcess checks if the process still exists, for something like
// sigterm. If the process still exists after loops and sleep time are exhausted,
// an error is returned
func backoffForProcess(pid int) error {
	sleepInterval := sleepTime
	for i := 0; i < loops; i++ {
		proxyProc, err := findProcess(pid)
		if proxyProc == nil && err != nil {
			// process is killed, gone
			return nil //nolint: nilerr
		}
		time.Sleep(sleepInterval)
		// double the time
		sleepInterval += sleepInterval
	}
	return fmt.Errorf("process %d has not ended", pid)
}

// waitOnProcess takes a pid and sends a sigterm to it. it then waits for the
// process to not exist.  if the sigterm does not end the process after an interval,
// then sigkill is sent.  it also waits for the process to exit after the sigkill too.
func waitOnProcess(processID int) error {
	proxyProc, err := findProcess(processID)
	if err != nil {
		return err
	}

	// Try to kill the pid with sigterm
	if runtime.GOOS != "windows" { // FIXME: temporary work around because signals are lame in windows
		if err := proxyProc.Signal(syscall.SIGTERM); err != nil {
			if err == syscall.ESRCH {
				return nil
			}
			return err
		}

		if err := backoffForProcess(processID); err == nil {
			return nil
		}
	}

	// sigterm has not killed it yet, lets send a sigkill
	proxyProc, err = findProcess(processID)
	if proxyProc == nil && err != nil {
		// process is killed, gone
		return nil //nolint: nilerr
	}
	if err := proxyProc.Signal(syscall.SIGKILL); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return err
	}
	return backoffForProcess(processID)
}

// CleanupGVProxy reads the --pid-file for gvproxy attempts to stop it
func CleanupGVProxy(f VMFile) error {
	gvPid, err := f.Read()
	if err != nil {
		return fmt.Errorf("unable to read gvproxy pid file %s: %v", f.GetPath(), err)
	}
	proxyPid, err := strconv.Atoi(string(gvPid))
	if err != nil {
		return fmt.Errorf("unable to convert pid to integer: %v", err)
	}
	if err := waitOnProcess(proxyPid); err != nil {
		return err
	}
	return f.Delete()
}
