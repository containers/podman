//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd
// +build darwin dragonfly freebsd linux netbsd openbsd

package command

import (
	"os"
	"strings"
)

func UseFdVLan() bool {
	return strings.ToUpper(os.Getenv("CONTAINERS_USE_SOCKET_VLAN")) != "TRUE"
}
