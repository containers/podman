// +build !linux,!freebsd

package sdk

import (
	"errors"
	"net"
)

var (
	errOnlySupportedOnLinuxAndFreeBSD = errors.New("unix socket creation is only supported on Linux and FreeBSD")
)

func newUnixListener(pluginName string, gid int) (net.Listener, string, error) {
	return nil, "", errOnlySupportedOnLinuxAndFreeBSD
}
