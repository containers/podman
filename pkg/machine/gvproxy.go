package machine

import (
	"fmt"
	"strconv"

	"github.com/containers/podman/v5/pkg/machine/define"
)

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
