//go:build !linux && !freebsd
// +build !linux,!freebsd

package libpod

import (
	"errors"

	"github.com/containers/podman/v4/libpod/define"
)

func (c *Container) stat(containerMountPoint string, containerPath string) (*define.FileInfo, string, string, error) {
	return nil, "", "", errors.New("Containers stat not supported on this platform")
}
