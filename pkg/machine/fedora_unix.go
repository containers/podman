//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd
// +build darwin dragonfly freebsd linux netbsd openbsd

package machine

import (
	"runtime"
)

func determineFedoraArch() string {
	return runtime.GOARCH
}
