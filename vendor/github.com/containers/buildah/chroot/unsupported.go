//go:build !linux
// +build !linux

package chroot

import (
	"errors"
	"io"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// RunUsingChroot is not supported.
func RunUsingChroot(spec *specs.Spec, bundlePath string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	return errors.New("--isolation chroot is not supported on this platform")
}
