//go:build linux

package reflink

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// LinkOrCopy attempts to reflink the source to the destination fd.
// If reflinking fails or is unsupported, it falls back to io.Copy().
func LinkOrCopy(src, dst *os.File) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, dst.Fd(), unix.FICLONE, src.Fd())
	if errno == 0 {
		return nil
	}

	_, err := io.Copy(dst, src)
	return err
}
