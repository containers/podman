// +build !linux

package libpod

import (
	"context"

	"github.com/containers/podman/v2/libpod/define"
)

func (r *Runtime) removeVolume(ctx context.Context, v *Volume, force bool) error {
	return define.ErrNotImplemented
}

func (r *Runtime) newVolume(ctx context.Context, options ...VolumeCreateOption) (*Volume, error) {
	return nil, define.ErrNotImplemented
}

func (r *Runtime) NewVolume(ctx context.Context, options ...VolumeCreateOption) (*Volume, error) {
	return nil, define.ErrNotImplemented
}
