package rootlessnetns

import (
	"errors"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/containers/storage/pkg/lockfile"
)

var ErrNotSupported = errors.New("rootless netns only supported on linux")

type Netns struct{}

func New(dir string, backend NetworkBackend, conf *config.Config) (*Netns, error) {
	return nil, ErrNotSupported
}

func (n *Netns) Setup(nets int, toRun func() error) error {
	return ErrNotSupported
}

func (n *Netns) Teardown(nets int, toRun func() error) error {
	return ErrNotSupported
}

func (n *Netns) Run(lock *lockfile.LockFile, toRun func() error) error {
	return ErrNotSupported
}

func (n *Netns) Info() *types.RootlessNetnsInfo {
	return &types.RootlessNetnsInfo{}
}
