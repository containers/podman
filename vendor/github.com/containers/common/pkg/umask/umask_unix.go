// +build linux darwin

package umask

import (
	"syscall"

	"github.com/sirupsen/logrus"
)

func Check() {
	oldUmask := syscall.Umask(0022) //nolint
	if (oldUmask & ^0022) != 0 {
		logrus.Debugf("umask value too restrictive.  Forcing it to 022")
	}
}

func Set(value int) int {
	return syscall.Umask(value)
}
