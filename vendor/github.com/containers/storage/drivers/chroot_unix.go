// +build linux darwin freebsd solaris

package graphdriver

import (
	"fmt"
	"os"
	"syscall"
)

// chrootOrChdir() is either a chdir() to the specified path, or a chroot() to the
// specified path followed by chdir() to the new root directory
func chrootOrChdir(path string) error {
	if err := syscall.Chroot(path); err != nil {
		return fmt.Errorf("error chrooting to %q: %v", path, err)
	}
	if err := syscall.Chdir(string(os.PathSeparator)); err != nil {
		return fmt.Errorf("error changing to %q: %v", path, err)
	}
	return nil
}
