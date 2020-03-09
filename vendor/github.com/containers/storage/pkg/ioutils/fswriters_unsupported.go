// +build !linux

package ioutils

import (
	"os"
)

func fdatasync(f *os.File) error {
	return f.Sync()
}
