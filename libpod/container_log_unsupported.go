//+build !linux !systemd

package libpod

import (
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/logs"
	"github.com/pkg/errors"
)

func (c *Container) readFromJournal(options *logs.LogOptions, logChannel chan *logs.LogLine) error {
	return errors.Wrapf(define.ErrOSNotSupported, "Journald logging only enabled with systemd on linux")
}
