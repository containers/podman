// +build !linux

package libpod

import (
	"context"

	"github.com/containers/podman/v3/libpod/define"
)

func (c *Container) stat(ctx context.Context, containerMountPoint string, containerPath string) (*define.FileInfo, string, string, error) {
	return nil, "", "", nil
}
