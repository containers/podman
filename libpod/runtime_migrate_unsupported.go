// +build !linux

package libpod

import (
	"context"
)

func (r *Runtime) migrate(ctx context.Context) error {
	return nil
}

func (r *Runtime) stopPauseProcess() error {
	return nil
}
