// +build linux,selinux

package chroot

import (
	"github.com/opencontainers/runtime-spec/specs-go"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// setSelinuxLabel sets the process label for child processes that we'll start.
func setSelinuxLabel(spec *specs.Spec) error {
	logrus.Debugf("setting selinux label")
	if spec.Process.SelinuxLabel != "" && selinux.GetEnabled() {
		if err := label.SetProcessLabel(spec.Process.SelinuxLabel); err != nil {
			return errors.Wrapf(err, "error setting process label to %q", spec.Process.SelinuxLabel)
		}
	}
	return nil
}
