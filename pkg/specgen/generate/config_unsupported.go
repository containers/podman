//go:build !linux
// +build !linux

package generate

import (
	"errors"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v4/pkg/specgen"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
)

// DevicesFromPath computes a list of devices
func DevicesFromPath(g *generate.Generator, devicePath string) error {
	return errors.New("unsupported DevicesFromPath")
}

func BlockAccessToKernelFilesystems(privileged, pidModeIsHost bool, mask, unmask []string, g *generate.Generator) {
}

func supportAmbientCapabilities() bool {
	return false
}

func getSeccompConfig(s *specgen.SpecGenerator, configSpec *spec.Spec, img *libimage.Image) (*spec.LinuxSeccomp, error) {
	return nil, errors.New("not implemented getSeccompConfig")
}
