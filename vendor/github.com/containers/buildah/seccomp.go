// +build seccomp,linux

package buildah

import (
	"io/ioutil"

	"github.com/containers/common/pkg/seccomp"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

func setupSeccomp(spec *specs.Spec, seccompProfilePath string) error {
	switch seccompProfilePath {
	case "unconfined":
		spec.Linux.Seccomp = nil
	case "":
		seccompConfig, err := seccomp.GetDefaultProfile(spec)
		if err != nil {
			return errors.Wrapf(err, "loading default seccomp profile failed")
		}
		spec.Linux.Seccomp = seccompConfig
	default:
		seccompProfile, err := ioutil.ReadFile(seccompProfilePath)
		if err != nil {
			return errors.Wrapf(err, "opening seccomp profile (%s) failed", seccompProfilePath)
		}
		seccompConfig, err := seccomp.LoadProfile(string(seccompProfile), spec)
		if err != nil {
			return errors.Wrapf(err, "loading seccomp profile (%s) failed", seccompProfilePath)
		}
		spec.Linux.Seccomp = seccompConfig
	}
	return nil
}
