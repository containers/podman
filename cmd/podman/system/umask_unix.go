// +build !windows

package system

import "syscall"

func umask(mask int) int {
	return syscall.Umask(mask)
}
