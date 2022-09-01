//go:build !linux && !freebsd
// +build !linux,!freebsd

package generate

import (
	"errors"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/opencontainers/runtime-tools/generate"
)

// setLabelOpts sets the label options of the SecurityConfig according to the
// input.
func setLabelOpts(s *specgen.SpecGenerator, runtime *libpod.Runtime, pidConfig specgen.Namespace, ipcConfig specgen.Namespace) error {
	return errors.New("unsupported setLabelOpts")
}

func securityConfigureGenerator(s *specgen.SpecGenerator, g *generate.Generator, newImage *libimage.Image, rtc *config.Config) error {
	return errors.New("unsupported securityConfigureGenerator")
}
