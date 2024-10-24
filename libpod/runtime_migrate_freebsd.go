//go:build !remote

package libpod

import (
	"errors"
)

func (r *Runtime) stopPauseProcess() error {
	return nil
}

func (r *Runtime) Migrate(newRuntime string) error {
	return errors.New("not implemented (*Runtime) migrate")
}
