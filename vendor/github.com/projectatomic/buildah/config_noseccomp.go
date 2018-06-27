// +build !seccomp

package buildah

import "github.com/opencontainers/runtime-spec/specs-go"

func setupSeccomp(spec *specs.Spec, seccompProfilePath string) error {
	// If no seccomp is being used, the Seccomp profile in the Linux spec
	// is not set
	return nil
}
