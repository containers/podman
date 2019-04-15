package createconfig

import (
	"github.com/containers/libpod/libpod"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// MakeContainerConfig generates all configuration necessary to start a
// container with libpod from a completed CreateConfig struct.
func (config *CreateConfig) MakeContainerConfig(runtime *libpod.Runtime, pod *libpod.Pod) (*spec.Spec, []libpod.CtrCreateOption, error) {
	runtimeSpec, err := config.createConfigToOCISpec()
	if err != nil {
		return nil, nil, err
	}

	options, err := config.getContainerCreateOptions(runtime, pod)
	if err != nil {
		return nil, nil, err
	}

	return runtimeSpec, options, nil
}
