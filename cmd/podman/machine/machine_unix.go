//go:build linux || ignore || aix || ignore || android || ignore || darwin || ignore || freebsd || ignore || hurd || ignore || illumos || ignore || ios || ignore || netbsd || ignore || openbsd || ignore || solaris
// +build linux ignore aix ignore android ignore darwin ignore freebsd ignore hurd ignore illumos ignore ios ignore netbsd ignore openbsd ignore solaris

package machine

import (
	"os"
)

func isUnixSocket(file os.DirEntry) bool {
	return file.Type()&os.ModeSocket != 0
}
