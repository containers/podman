// +build linux,cgo

package createconfig

import (
	"io/ioutil"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	seccomp "github.com/seccomp/containers-golang"
	"github.com/sirupsen/logrus"
)

func getSeccompConfig(config *SecurityConfig, configSpec *spec.Spec) (*spec.LinuxSeccomp, error) {
	var seccompConfig *spec.LinuxSeccomp
	var err error

	if config.SeccompPolicy == SeccompPolicyImage && config.SeccompProfileFromImage != "" {
		logrus.Debug("Loading seccomp profile from the security config")
		seccompConfig, err = seccomp.LoadProfile(config.SeccompProfileFromImage, configSpec)
		if err != nil {
			return nil, errors.Wrap(err, "loading seccomp profile failed")
		}
		return seccompConfig, nil
	}

	if config.SeccompProfilePath != "" {
		logrus.Debugf("Loading seccomp profile from %q", config.SeccompProfilePath)
		seccompProfile, err := ioutil.ReadFile(config.SeccompProfilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "opening seccomp profile (%s) failed", config.SeccompProfilePath)
		}
		seccompConfig, err = seccomp.LoadProfile(string(seccompProfile), configSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "loading seccomp profile (%s) failed", config.SeccompProfilePath)
		}
	} else {
		logrus.Debug("Loading default seccomp profile")
		seccompConfig, err = seccomp.GetDefaultProfile(configSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "loading seccomp profile (%s) failed", config.SeccompProfilePath)
		}
	}

	return seccompConfig, nil
}
