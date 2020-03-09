package ioutils

import (
	"os"

	"golang.org/x/sys/unix"
)

func fdatasync(f *os.File) error {
	return unix.Fdatasync(int(f.Fd()))
}
