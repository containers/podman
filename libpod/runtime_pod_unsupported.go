// +build !linux

package libpod

import (
	"context"

	"github.com/containers/podman/v2/libpod/define"
)

// NewPod makes a new, empty pod
func (r *Runtime) NewPod(ctx context.Context, options ...PodCreateOption) (*Pod, error) {
	return nil, define.ErrOSNotSupported
}

func (r *Runtime) removePod(ctx context.Context, p *Pod, removeCtrs, force bool) error {
	return define.ErrOSNotSupported
}
