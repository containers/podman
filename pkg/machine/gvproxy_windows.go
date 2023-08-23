//go:build windows
// +build windows

package machine

import "os"

func findProcess(pid int) (*os.Process, error) {
	return os.FindProcess(pid)
}
