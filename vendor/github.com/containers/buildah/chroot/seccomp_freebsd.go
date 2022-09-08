//go:build freebsd && seccomp
// +build freebsd,seccomp

package chroot

import (
	"github.com/opencontainers/runtime-spec/specs-go"
)

func setSeccomp(spec *specs.Spec) error {
	// Ignore this on FreeBSD
	return nil
}
