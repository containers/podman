// +build !linux

package libpod

import (
	"context"
)

// NewPod makes a new, empty pod
func (r *Runtime) NewPod(options ...PodCreateOption) (*Pod, error) {
	return nil, ErrOSNotSupported
}

func (r *Runtime) RemovePod(ctx context.Context, p *Pod, removeCtrs, force bool) error {
	return ErrOSNotSupported
}
