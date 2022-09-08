//go:build (!linux && !freebsd) || !seccomp
// +build !linux,!freebsd !seccomp

package chroot

import (
	"errors"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func setSeccomp(spec *specs.Spec) error {
	if spec.Linux.Seccomp != nil {
		return errors.New("configured a seccomp filter without seccomp support?")
	}
	return nil
}

func setupSeccomp(spec *specs.Spec, seccompProfilePath string) error {
	if spec.Linux != nil {
		// runtime-tools may have supplied us with a default filter
		spec.Linux.Seccomp = nil
	}
	return nil
}
