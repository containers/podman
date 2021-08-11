//+build !linux !systemd

package libpod

import (
	"context"

	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/logs"
	"github.com/pkg/errors"
)

func (c *Container) readFromJournal(_ context.Context, _ *logs.LogOptions, _ chan *logs.LogLine) error {
	return errors.Wrapf(define.ErrOSNotSupported, "Journald logging only enabled with systemd on linux")
}

func (c *Container) initializeJournal(ctx context.Context) error {
	return errors.Wrapf(define.ErrOSNotSupported, "Journald logging only enabled with systemd on linux")
}
