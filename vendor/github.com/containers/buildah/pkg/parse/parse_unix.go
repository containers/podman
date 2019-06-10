// +build linux darwin

package parse

import (
	"fmt"
	"golang.org/x/sys/unix"
)

func getDefaultProcessLimits() []string {
	rlim := unix.Rlimit{Cur: 1048576, Max: 1048576}
	defaultLimits := []string{}
	if err := unix.Setrlimit(unix.RLIMIT_NOFILE, &rlim); err == nil {
		defaultLimits = append(defaultLimits, fmt.Sprintf("nofile=%d:%d", rlim.Cur, rlim.Max))
	}
	if err := unix.Setrlimit(unix.RLIMIT_NPROC, &rlim); err == nil {
		defaultLimits = append(defaultLimits, fmt.Sprintf("nproc=%d:%d", rlim.Cur, rlim.Max))
	}
	return defaultLimits
}
