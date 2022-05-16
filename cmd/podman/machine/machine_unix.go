//go:build linux || aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || netbsd || openbsd || solaris
// +build linux aix android darwin dragonfly freebsd hurd illumos ios netbsd openbsd solaris

package machine

import (
	"os"
)

func isUnixSocket(file os.DirEntry) bool {
	return file.Type()&os.ModeSocket != 0
}
