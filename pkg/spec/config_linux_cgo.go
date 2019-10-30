// +build linux,cgo

package createconfig

import (
	"io/ioutil"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	seccomp "github.com/seccomp/containers-golang"
)

func getSeccompConfig(config *CreateConfig, configSpec *spec.Spec) (*spec.LinuxSeccomp, error) {
	var seccompConfig *spec.LinuxSeccomp
	var err error

	if config.SeccompProfilePath != "" {
		seccompProfile, err := ioutil.ReadFile(config.SeccompProfilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "opening seccomp profile (%s) failed", config.SeccompProfilePath)
		}
		seccompConfig, err = seccomp.LoadProfile(string(seccompProfile), configSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "loading seccomp profile (%s) failed", config.SeccompProfilePath)
		}
	} else {
		seccompConfig, err = seccomp.GetDefaultProfile(configSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "loading seccomp profile (%s) failed", config.SeccompProfilePath)
		}
	}

	return seccompConfig, nil
}
