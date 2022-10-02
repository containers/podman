//go:build !linux && !freebsd
// +build !linux,!freebsd

package libpod

import (
	"context"
	"errors"

	"github.com/containers/podman/v4/pkg/specgen"
)

// NewPod makes a new, empty pod
func (r *Runtime) NewPod(ctx context.Context, p specgen.PodSpecGenerator, options ...PodCreateOption) (_ *Pod, deferredErr error) {
	return nil, errors.New("not implemented (*Runtime) NewPod")
}

// AddInfra adds the created infra container to the pod state
func (r *Runtime) AddInfra(ctx context.Context, pod *Pod, infraCtr *Container) (*Pod, error) {
	return nil, errors.New("not implemented (*Runtime) AddInfra")
}

// SavePod is a helper function to save the pod state from outside of libpod
func (r *Runtime) SavePod(pod *Pod) error {
	return errors.New("not implemented (*Runtime) SavePod")
}

func (r *Runtime) removePod(ctx context.Context, p *Pod, removeCtrs, force bool, timeout *uint) error {
	return errors.New("not implemented (*Runtime) removePod")
}
