package machine

import (
	"fmt"
	"strconv"
	"time"

	"github.com/containers/podman/v5/pkg/machine/define"
	psutil "github.com/shirou/gopsutil/v3/process"
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

// CleanupGVProxy reads the --pid-file for gvproxy attempts to stop it
func CleanupGVProxy(f define.VMFile) error {
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
