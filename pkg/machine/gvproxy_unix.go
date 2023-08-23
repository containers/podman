//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd
// +build darwin dragonfly freebsd linux netbsd openbsd

package machine

import (
	"os"
	"syscall"
)

func findProcess(pid int) (*os.Process, error) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}
	// On unix, findprocess will always return a process even
	// if the process is not found.  you must send a 0 signal
	// to the process to see if it is alive.
	// https://pkg.go.dev/os#FindProcess
	if err := p.Signal(syscall.Signal(0)); err != nil {
		return nil, err
	}
	return p, nil
}
