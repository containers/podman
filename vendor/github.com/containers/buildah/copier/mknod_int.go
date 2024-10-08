//go:build !windows && !freebsd
// +build !windows,!freebsd

package copier

import (
	"golang.org/x/sys/unix"
)

func mknod(path string, mode uint32, dev int) error {
	return unix.Mknod(path, mode, dev)
}
