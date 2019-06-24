//+build !linux !systemd

package libpod

import (
	"github.com/containers/libpod/libpod/define"
	"github.com/pkg/errors"
)

func (c *Container) readFromJournal(options *LogOptions, logChannel chan *LogLine) error {
	return errors.Wrapf(define.ErrOSNotSupported, "Journald logging only enabled with systemd on linux")
}
