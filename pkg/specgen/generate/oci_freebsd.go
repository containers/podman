//go:build freebsd

package generate

import (
	"context"
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/specgen"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
)

// SpecGenToOCI returns the base configuration for the container.
func SpecGenToOCI(ctx context.Context, s *specgen.SpecGenerator, rt *libpod.Runtime, rtc *config.Config, newImage *libimage.Image, mounts []spec.Mount, pod *libpod.Pod, finalCmd []string, compatibleOptions *libpod.InfraInherit) (*spec.Spec, error) {
	g, err := generate.New("freebsd")
	if err != nil {
		return nil, err
	}

	g.SetProcessCwd(s.WorkDir)

	g.SetProcessArgs(finalCmd)

	g.SetProcessTerminal(s.Terminal)

	for key, val := range s.Annotations {
		g.AddAnnotation(key, val)
	}

	g.ClearProcessEnv()
	for name, val := range s.Env {
		g.AddProcessEnv(name, val)
	}

	addRlimits(s, &g)

	// NAMESPACES
	if err := specConfigureNamespaces(s, &g, rt, pod); err != nil {
		return nil, err
	}
	configSpec := g.Config

	if err := securityConfigureGenerator(s, &g, newImage, rtc); err != nil {
		return nil, err
	}

	// BIND MOUNTS
	configSpec.Mounts = SupersedeUserMounts(mounts, configSpec.Mounts)
	// Process mounts to ensure correct options
	if err := InitFSMounts(configSpec.Mounts); err != nil {
		return nil, err
	}

	// Add annotations
	if configSpec.Annotations == nil {
		configSpec.Annotations = make(map[string]string)
	}

	if s.Remove {
		configSpec.Annotations[define.InspectAnnotationAutoremove] = define.InspectResponseTrue
	}

	if len(s.VolumesFrom) > 0 {
		configSpec.Annotations[define.InspectAnnotationVolumesFrom] = strings.Join(s.VolumesFrom, ",")
	}

	if s.Privileged {
		configSpec.Annotations[define.InspectAnnotationPrivileged] = define.InspectResponseTrue
	}

	if s.Init {
		configSpec.Annotations[define.InspectAnnotationInit] = define.InspectResponseTrue
	}

	if s.OOMScoreAdj != nil {
		g.SetProcessOOMScoreAdj(*s.OOMScoreAdj)
	}

	return configSpec, nil
}

func WeightDevices(wtDevices map[string]spec.LinuxWeightDevice) ([]spec.LinuxWeightDevice, error) {
	devs := []spec.LinuxWeightDevice{}
	return devs, nil
}
