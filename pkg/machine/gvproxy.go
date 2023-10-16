package machine

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"syscall"
	"time"

	psutil "github.com/shirou/gopsutil/v3/process"
	"github.com/sirupsen/logrus"
)

const (
	loops     = 8
	sleepTime = time.Millisecond * 1
)

// backoffForProcess checks if the process still exists, for something like
// sigterm. If the process still exists after loops and sleep time are exhausted,
// an error is returned
func backoffForProcess(p *psutil.Process) error {
	sleepInterval := sleepTime
	for i := 0; i < loops; i++ {
		running, err := p.IsRunning()
		if err != nil {
			return fmt.Errorf("checking if process running: %w", err)
		}
		if !running {
			return nil
		}

		time.Sleep(sleepInterval)
		// double the time
		sleepInterval += sleepInterval
	}
	return fmt.Errorf("process %d has not ended", p.Pid)
}

// waitOnProcess takes a pid and sends a sigterm to it. it then waits for the
// process to not exist.  if the sigterm does not end the process after an interval,
// then sigkill is sent.  it also waits for the process to exit after the sigkill too.
func waitOnProcess(processID int) error {
	logrus.Infof("Going to stop gvproxy (PID %d)", processID)

	p, err := psutil.NewProcess(int32(processID))
	if err != nil {
		return fmt.Errorf("looking up PID %d: %w", processID, err)
	}

	// Try to kill the pid with sigterm
	if runtime.GOOS != "windows" { // FIXME: temporary work around because signals are lame in windows
		if err := p.SendSignal(syscall.SIGTERM); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return nil
			}
			return fmt.Errorf("sending SIGTERM to grproxy: %w", err)
		}

		if err := backoffForProcess(p); err == nil {
			return nil
		}
	}

	running, err := p.IsRunning()
	if err != nil {
		return fmt.Errorf("checking if gvproxy is running: %w", err)
	}
	if !running {
		return nil
	}

	if err := p.Kill(); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			logrus.Debugf("Gvproxy already dead, exiting cleanly")
			return nil
		}
		return err
	}
	return backoffForProcess(p)
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
