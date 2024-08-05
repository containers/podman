//go:build linux || aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || netbsd || openbsd || solaris

package define

import (
	"errors"
	"os"

	"github.com/sirupsen/logrus"
)

// Delete removes the machinefile symlink (if it exists) and
// the actual path
func (m *VMFile) Delete() error {
	if m.Symlink != nil {
		if err := os.Remove(*m.Symlink); err != nil && !errors.Is(err, os.ErrNotExist) {
			logrus.Errorf("unable to remove symlink %q", *m.Symlink)
		}
	}
	if err := os.Remove(m.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
