//go:build !windows && !linux

package timezone

import (
	"golang.org/x/sys/unix"
)

func openDirectory(path string) (fd int, err error) {
	const O_PATH = 0x00400000
	return unix.Open(path, unix.O_RDONLY|O_PATH|unix.O_CLOEXEC, 0)
}
