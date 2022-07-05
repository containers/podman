//go:build !linux || !systemd
// +build !linux !systemd

package libpod

import (
	"context"
	"fmt"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/logs"
)

func (c *Container) readFromJournal(_ context.Context, _ *logs.LogOptions, _ chan *logs.LogLine, colorID int64) error {
	return fmt.Errorf("journald logging only enabled with systemd on linux: %w", define.ErrOSNotSupported)
}

func (c *Container) initializeJournal(ctx context.Context) error {
	return fmt.Errorf("journald logging only enabled with systemd on linux: %w", define.ErrOSNotSupported)
}
