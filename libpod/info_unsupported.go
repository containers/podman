//go:build !linux
// +build !linux

package libpod

import (
	"errors"

	"github.com/containers/podman/v4/libpod/define"
)

func (r *Runtime) info() (*define.Info, error) {
	return nil, errors.New("not implemented (*Runtime) info")
}
