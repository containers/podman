package createconfig

import (
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// MakeContainerConfig generates all configuration necessary to start a
// container with libpod from a completed CreateConfig struct.
func (config *CreateConfig) MakeContainerConfig(runtime *libpod.Runtime, pod *libpod.Pod) (*spec.Spec, []libpod.CtrCreateOption, error) {
	if config.Pod != "" && pod == nil {
		return nil, nil, errors.Wrapf(define.ErrInvalidArg, "pod was specified but no pod passed")
	} else if config.Pod == "" && pod != nil {
		return nil, nil, errors.Wrapf(define.ErrInvalidArg, "pod was given but no pod is specified")
	}

	// Parse volumes flag into OCI spec mounts and libpod Named Volumes.
	// If there is an identical mount in the OCI spec, we will replace it
	// with a mount generated here.
	mounts, namedVolumes, err := config.parseVolumes(runtime)
	if err != nil {
		return nil, nil, err
	}

	runtimeSpec, err := config.createConfigToOCISpec(runtime, mounts)
	if err != nil {
		return nil, nil, err
	}

	options, err := config.getContainerCreateOptions(runtime, pod, mounts, namedVolumes)
	if err != nil {
		return nil, nil, err
	}

	logrus.Debugf("created OCI spec and options for new container")

	return runtimeSpec, options, nil
}
