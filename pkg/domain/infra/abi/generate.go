package abi

import (
	"bytes"
	"context"
	"fmt"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/systemd/generate"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	k8sAPI "k8s.io/api/core/v1"
)

func (ic *ContainerEngine) GenerateSystemd(ctx context.Context, nameOrID string, options entities.GenerateSystemdOptions) (*entities.GenerateSystemdReport, error) {
	// First assume it's a container.
	ctr, ctrErr := ic.Libpod.LookupContainer(nameOrID)
	if ctrErr == nil {
		// Generate the unit for the container.
		name, content, err := generate.ContainerUnit(ctr, options)
		if err != nil {
			return nil, err
		}
		return &entities.GenerateSystemdReport{Units: map[string]string{name: content}}, nil
	}

	// If it's not a container, we either have a pod or garbage.
	pod, err := ic.Libpod.LookupPod(nameOrID)
	if err != nil {
		err = errors.Wrap(ctrErr, err.Error())
		return nil, errors.Wrapf(err, "%s does not refer to a container or pod", nameOrID)
	}

	// Generate the units for the pod and all its containers.
	units, err := generate.PodUnits(pod, options)
	if err != nil {
		return nil, err
	}
	return &entities.GenerateSystemdReport{Units: units}, nil
}

func (ic *ContainerEngine) GenerateKube(ctx context.Context, nameOrID string, options entities.GenerateKubeOptions) (*entities.GenerateKubeReport, error) {
	var (
		pod          *libpod.Pod
		podYAML      *k8sAPI.Pod
		err          error
		ctr          *libpod.Container
		servicePorts []k8sAPI.ServicePort
		serviceYAML  k8sAPI.Service
	)
	// Get the container in question.
	ctr, err = ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		pod, err = ic.Libpod.LookupPod(nameOrID)
		if err != nil {
			return nil, err
		}
		podYAML, servicePorts, err = pod.GenerateForKube()
	} else {
		if len(ctr.Dependencies()) > 0 {
			return nil, errors.Wrapf(define.ErrNotImplemented, "containers with dependencies")
		}
		podYAML, err = ctr.GenerateForKube()
	}
	if err != nil {
		return nil, err
	}

	if options.Service {
		serviceYAML = libpod.GenerateKubeServiceFromV1Pod(podYAML, servicePorts)
	}

	content, err := generateKubeOutput(podYAML, &serviceYAML)
	if err != nil {
		return nil, err
	}

	return &entities.GenerateKubeReport{Reader: bytes.NewReader(content)}, nil
}

func generateKubeOutput(podYAML *k8sAPI.Pod, serviceYAML *k8sAPI.Service) ([]byte, error) {
	var (
		output            []byte
		marshalledPod     []byte
		marshalledService []byte
		err               error
	)

	marshalledPod, err = yaml.Marshal(podYAML)
	if err != nil {
		return nil, err
	}

	if serviceYAML != nil {
		marshalledService, err = yaml.Marshal(serviceYAML)
		if err != nil {
			return nil, err
		}
	}

	header := `# Generation of Kubernetes YAML is still under development!
#
# Save the output of this file and use kubectl create -f to import
# it into Kubernetes.
#
# Created with podman-%s
`
	podmanVersion, err := define.GetVersion()
	if err != nil {
		return nil, err
	}

	output = append(output, []byte(fmt.Sprintf(header, podmanVersion.Version))...)
	output = append(output, marshalledPod...)
	if serviceYAML != nil {
		output = append(output, []byte("---\n")...)
		output = append(output, marshalledService...)
	}

	return output, nil
}
