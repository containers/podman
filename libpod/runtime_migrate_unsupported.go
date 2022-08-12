//go:build !linux
// +build !linux

package libpod

import (
	"errors"
)

func (r *Runtime) stopPauseProcess() error {
	return errors.New("not implemented (*Runtime) stopPauseProcess")
}

func (r *Runtime) migrate() error {
	return errors.New("not implemented (*Runtime) migrate")
}
