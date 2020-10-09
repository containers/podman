// +build linux,cgo

package createconfig

import (
	"io/ioutil"

	goSeccomp "github.com/containers/common/pkg/seccomp"
	"github.com/containers/podman/v2/pkg/seccomp"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func getSeccompConfig(config *SecurityConfig, configSpec *spec.Spec) (*spec.LinuxSeccomp, error) {
	var seccompConfig *spec.LinuxSeccomp
	var err error

	if config.SeccompPolicy == seccomp.PolicyImage && config.SeccompProfileFromImage != "" {
		logrus.Debug("Loading seccomp profile from the security config")
		seccompConfig, err = goSeccomp.LoadProfile(config.SeccompProfileFromImage, configSpec)
		if err != nil {
			return nil, errors.Wrap(err, "loading seccomp profile failed")
		}
		return seccompConfig, nil
	}

	if config.SeccompProfilePath != "" {
		logrus.Debugf("Loading seccomp profile from %q", config.SeccompProfilePath)
		seccompProfile, err := ioutil.ReadFile(config.SeccompProfilePath)
		if err != nil {
			return nil, errors.Wrap(err, "opening seccomp profile failed")
		}
		seccompConfig, err = goSeccomp.LoadProfile(string(seccompProfile), configSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "loading seccomp profile (%s) failed", config.SeccompProfilePath)
		}
	} else {
		logrus.Debug("Loading default seccomp profile")
		seccompConfig, err = goSeccomp.GetDefaultProfile(configSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "loading default seccomp profile failed")
		}
	}

	return seccompConfig, nil
}
