package timezone

import (
	"golang.org/x/sys/unix"
)

func openDirectory(path string) (fd int, err error) {
	return unix.Open(path, unix.O_RDONLY|unix.O_PATH|unix.O_CLOEXEC, 0)
}
