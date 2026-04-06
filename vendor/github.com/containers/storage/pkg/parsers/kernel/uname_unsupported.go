//go:build openbsd
// +build openbsd

package kernel

import (
	"fmt"
	"runtime"
)

// A stub called by kernel_unix.go .
func uname() (*Utsname, error) {
	return nil, fmt.Errorf("Kernel version detection is not available on %s", runtime.GOOS)
}
