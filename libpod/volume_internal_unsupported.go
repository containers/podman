// +build !linux

package libpod

import (
	"context"
	"github.com/containers/podman/v2/libpod/define"
)

func (v *Volume) mount(ctx context.Context) error {
	return define.ErrNotImplemented
}

func (v *Volume) unmount(ctx context.Context, force bool) error {
	return define.ErrNotImplemented
}
