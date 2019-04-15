package createconfig

import (
	"github.com/containers/libpod/libpod"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// MakeContainerConfig generates all configuration necessary to start a
// container with libpod from a completed CreateConfig struct.
func (config *CreateConfig) MakeContainerConfig(runtime *libpod.Runtime, pod *libpod.Pod) (*spec.Spec, []libpod.CtrCreateOption, error) {
	if config.Pod != "" && pod == nil {
		return nil, nil, errors.Wrapf(libpod.ErrInvalidArg, "pod was specified but no pod passed")
	} else if config.Pod == "" && pod != nil {
		return nil, nil, errors.Wrapf(libpod.ErrInvalidArg, "pod was given but no pod is specified")
	}

	runtimeSpec, namedVolumes, err := config.createConfigToOCISpec(runtime)
	if err != nil {
		return nil, nil, err
	}

	options, err := config.getContainerCreateOptions(runtime, pod, namedVolumes)
	if err != nil {
		return nil, nil, err
	}

	return runtimeSpec, options, nil
}
