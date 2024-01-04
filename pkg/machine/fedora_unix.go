//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package machine

import (
	"runtime"
)

func DetermineMachineArch() string {
	return runtime.GOARCH
}
