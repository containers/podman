// +build !linux

package libpod

import (
	"context"

	"github.com/containers/libpod/libpod/define"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func (c *Container) mountSHM(shmOptions string) error {
	return define.ErrNotImplemented
}

func (c *Container) unmountSHM(mount string) error {
	return define.ErrNotImplemented
}

func (c *Container) prepare() (err error) {
	return define.ErrNotImplemented
}

func (c *Container) cleanupNetwork() error {
	return define.ErrNotImplemented
}

func (c *Container) generateSpec(ctx context.Context) (*spec.Spec, error) {
	return nil, define.ErrNotImplemented
}

func (c *Container) checkpoint(ctx context.Context, options ContainerCheckpointOptions) error {
	return define.ErrNotImplemented
}

func (c *Container) restore(ctx context.Context, options ContainerCheckpointOptions) error {
	return define.ErrNotImplemented
}

func (c *Container) copyOwnerAndPerms(source, dest string) error {
	return nil
}

func (c *Container) refreshCNI() error {
	return define.ErrNotImplemented
}

func (c *Container) getOCICgroupPath() (string, error) {
	return "", define.ErrNotImplemented
}
