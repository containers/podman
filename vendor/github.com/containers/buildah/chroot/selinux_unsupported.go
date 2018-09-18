// +build !linux !selinux

package chroot

import (
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func setSelinuxLabel(spec *specs.Spec) error {
	if spec.Linux.MountLabel != "" {
		return errors.New("configured an SELinux mount label without SELinux support?")
	}
	if spec.Process.SelinuxLabel != "" {
		return errors.New("configured an SELinux process label without SELinux support?")
	}
	return nil
}
