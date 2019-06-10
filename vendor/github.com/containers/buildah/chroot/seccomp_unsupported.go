// +build !linux !seccomp

package chroot

import (
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func setSeccomp(spec *specs.Spec) error {
	if spec.Linux.Seccomp != nil {
		return errors.New("configured a seccomp filter without seccomp support?")
	}
	return nil
}
