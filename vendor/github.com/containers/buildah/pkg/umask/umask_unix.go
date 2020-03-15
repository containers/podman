// +build linux darwin

package umask

import (
	"syscall"

	"github.com/sirupsen/logrus"
)

func CheckUmask() {
	oldUmask := syscall.Umask(0022)
	if (oldUmask & ^0022) != 0 {
		logrus.Debugf("umask value too restrictive.  Forcing it to 022")
	}
}

func SetUmask(value int) int {
	return syscall.Umask(value)
}
