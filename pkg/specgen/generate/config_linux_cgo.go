//go:build linux && cgo
// +build linux,cgo

package generate

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/containers/common/libimage"
	goSeccomp "github.com/containers/common/pkg/seccomp"
	"github.com/containers/podman/v4/pkg/seccomp"
	"github.com/containers/podman/v4/pkg/specgen"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

func getSeccompConfig(s *specgen.SpecGenerator, configSpec *spec.Spec, img *libimage.Image) (*spec.LinuxSeccomp, error) {
	var seccompConfig *spec.LinuxSeccomp
	var err error
	scp, err := seccomp.LookupPolicy(s.SeccompPolicy)
	if err != nil {
		return nil, err
	}

	if scp == seccomp.PolicyImage {
		if img == nil {
			return nil, errors.New("cannot read seccomp profile without a valid image")
		}
		labels, err := img.Labels(context.Background())
		if err != nil {
			return nil, err
		}
		imagePolicy := labels[seccomp.ContainerImageLabel]
		if len(imagePolicy) < 1 {
			return nil, errors.New("no seccomp policy defined by image")
		}
		logrus.Debug("Loading seccomp profile from the security config")
		seccompConfig, err = goSeccomp.LoadProfile(imagePolicy, configSpec)
		if err != nil {
			return nil, fmt.Errorf("loading seccomp profile failed: %w", err)
		}
		return seccompConfig, nil
	}

	if s.SeccompProfilePath != "" {
		logrus.Debugf("Loading seccomp profile from %q", s.SeccompProfilePath)
		seccompProfile, err := ioutil.ReadFile(s.SeccompProfilePath)
		if err != nil {
			return nil, fmt.Errorf("opening seccomp profile failed: %w", err)
		}
		seccompConfig, err = goSeccomp.LoadProfile(string(seccompProfile), configSpec)
		if err != nil {
			return nil, fmt.Errorf("loading seccomp profile (%s) failed: %w", s.SeccompProfilePath, err)
		}
	} else {
		logrus.Debug("Loading default seccomp profile")
		seccompConfig, err = goSeccomp.GetDefaultProfile(configSpec)
		if err != nil {
			return nil, fmt.Errorf("loading seccomp profile (%s) failed: %w", s.SeccompProfilePath, err)
		}
	}

	return seccompConfig, nil
}
