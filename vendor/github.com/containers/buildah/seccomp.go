//go:build seccomp && linux
// +build seccomp,linux

package buildah

import (
	"fmt"
	"io/ioutil"

	"github.com/containers/common/pkg/seccomp"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func setupSeccomp(spec *specs.Spec, seccompProfilePath string) error {
	switch seccompProfilePath {
	case "unconfined":
		spec.Linux.Seccomp = nil
	case "":
		seccompConfig, err := seccomp.GetDefaultProfile(spec)
		if err != nil {
			return fmt.Errorf("loading default seccomp profile failed: %w", err)
		}
		spec.Linux.Seccomp = seccompConfig
	default:
		seccompProfile, err := ioutil.ReadFile(seccompProfilePath)
		if err != nil {
			return fmt.Errorf("opening seccomp profile (%s) failed: %w", seccompProfilePath, err)
		}
		seccompConfig, err := seccomp.LoadProfile(string(seccompProfile), spec)
		if err != nil {
			return fmt.Errorf("loading seccomp profile (%s) failed: %w", seccompProfilePath, err)
		}
		spec.Linux.Seccomp = seccompConfig
	}
	return nil
}
