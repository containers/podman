//go:build !remote && !linux

package libpod

import (
	"errors"
)

func (r *Runtime) stopPauseProcess() error {
	return errors.New("not implemented (*Runtime) stopPauseProcess")
}

func (r *Runtime) Migrate(newRuntime string) error {
	return errors.New("not implemented (*Runtime) migrate")
}
