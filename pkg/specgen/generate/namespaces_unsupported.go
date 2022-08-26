//go:build !linux
// +build !linux

package generate

import (
	"errors"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/opencontainers/runtime-tools/generate"
)

func specConfigureNamespaces(s *specgen.SpecGenerator, g *generate.Generator, rt *libpod.Runtime, pod *libpod.Pod) error {
	return errors.New("unsupported specConfigureNamespaces")
}
