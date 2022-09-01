//go:build !linux && !freebsd
// +build !linux,!freebsd

package generate

import (
	"context"
	"errors"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/specgen"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// SpecGenToOCI returns the base configuration for the container.
func SpecGenToOCI(ctx context.Context, s *specgen.SpecGenerator, rt *libpod.Runtime, rtc *config.Config, newImage *libimage.Image, mounts []spec.Mount, pod *libpod.Pod, finalCmd []string, compatibleOptions *libpod.InfraInherit) (*spec.Spec, error) {
	return nil, errors.New("unsupported SpecGenToOCI")
}

func WeightDevices(wtDevices map[string]spec.LinuxWeightDevice) ([]spec.LinuxWeightDevice, error) {
	return []spec.LinuxWeightDevice{}, errors.New("unsupported WeightDevices")
}
