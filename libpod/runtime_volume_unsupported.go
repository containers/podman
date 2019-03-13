// +build !linux

package libpod

import (
	"context"
)

func (r *Runtime) removeVolume(ctx context.Context, v *Volume, force bool) error {
	return ErrNotImplemented
}

func (r *Runtime) newVolume(ctx context.Context, options ...VolumeCreateOption) (*Volume, error) {
	return nil, ErrNotImplemented
}

func (r *Runtime) NewVolume(ctx context.Context, options ...VolumeCreateOption) (*Volume, error) {
	return nil, ErrNotImplemented
}
