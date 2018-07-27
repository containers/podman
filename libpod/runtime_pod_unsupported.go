// +build !linux

package libpod

import (
	"context"
)

// NewPod makes a new, empty pod
func (r *Runtime) NewPod(ctx context.Context, options ...PodCreateOption) (*Pod, error) {
	return nil, ErrOSNotSupported
}

func (r *Runtime) removePod(ctx context.Context, p *Pod, removeCtrs, force bool) error {
	return ErrOSNotSupported
}
