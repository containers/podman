package abi

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/systemd/generate"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	k8sAPI "k8s.io/api/core/v1"
)

func (ic *ContainerEngine) GenerateSystemd(ctx context.Context, nameOrID string, options entities.GenerateSystemdOptions) (*entities.GenerateSystemdReport, error) {
	opts := generate.Options{
		Files: options.Files,
		New:   options.New,
	}

	// First assume it's a container.
	if info, found, err := ic.generateSystemdgenContainerInfo(nameOrID, nil, options); found && err != nil {
		return nil, err
	} else if found && err == nil {
		output, err := generate.CreateContainerSystemdUnit(info, opts)
		if err != nil {
			return nil, err
		}
		return &entities.GenerateSystemdReport{Output: output}, nil
	}

	// --new does not support pods.
	if options.New {
		return nil, errors.Errorf("error generating systemd unit files: cannot generate generic files for a pod")
	}

	// We're either having a pod or garbage.
	pod, err := ic.Libpod.LookupPod(nameOrID)
	if err != nil {
		return nil, err
	}

	// Error out if the pod has no infra container, which we require to be the
	// main service.
	if !pod.HasInfraContainer() {
		return nil, fmt.Errorf("error generating systemd unit files: Pod %q has no infra container", pod.Name())
	}

	// Generate a systemdgen.ContainerInfo for the infra container. This
	// ContainerInfo acts as the main service of the pod.
	infraID, err := pod.InfraContainerID()
	if err != nil {
		return nil, nil
	}
	podInfo, _, err := ic.generateSystemdgenContainerInfo(infraID, pod, options)
	if err != nil {
		return nil, err
	}

	// Compute the container-dependency graph for the Pod.
	containers, err := pod.AllContainers()
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return nil, fmt.Errorf("error generating systemd unit files: Pod %q has no containers", pod.Name())
	}
	graph, err := libpod.BuildContainerGraph(containers)
	if err != nil {
		return nil, err
	}

	// Traverse the dependency graph and create systemdgen.ContainerInfo's for
	// each container.
	containerInfos := []*generate.ContainerInfo{podInfo}
	for ctr, dependencies := range graph.DependencyMap() {
		// Skip the infra container as we already generated it.
		if ctr.ID() == infraID {
			continue
		}
		ctrInfo, _, err := ic.generateSystemdgenContainerInfo(ctr.ID(), nil, options)
		if err != nil {
			return nil, err
		}
		// Now add the container's dependencies and at the container as a
		// required service of the infra container.
		for _, dep := range dependencies {
			if dep.ID() == infraID {
				ctrInfo.BoundToServices = append(ctrInfo.BoundToServices, podInfo.ServiceName)
			} else {
				_, serviceName := generateServiceName(dep, nil, options)
				ctrInfo.BoundToServices = append(ctrInfo.BoundToServices, serviceName)
			}
		}
		podInfo.RequiredServices = append(podInfo.RequiredServices, ctrInfo.ServiceName)
		containerInfos = append(containerInfos, ctrInfo)
	}

	// Now generate the systemd service for all containers.
	builder := strings.Builder{}
	for i, info := range containerInfos {
		if i > 0 {
			builder.WriteByte('\n')
		}
		out, err := generate.CreateContainerSystemdUnit(info, opts)
		if err != nil {
			return nil, err
		}
		builder.WriteString(out)
	}

	return &entities.GenerateSystemdReport{Output: builder.String()}, nil
}

// generateSystemdgenContainerInfo is a helper to generate a
// systemdgen.ContainerInfo for `GenerateSystemd`.
func (ic *ContainerEngine) generateSystemdgenContainerInfo(nameOrID string, pod *libpod.Pod, options entities.GenerateSystemdOptions) (*generate.ContainerInfo, bool, error) {
	ctr, err := ic.Libpod.LookupContainer(nameOrID)
	if err != nil {
		return nil, false, err
	}

	timeout := ctr.StopTimeout()
	if options.StopTimeout != nil {
		timeout = *options.StopTimeout
	}

	config := ctr.Config()
	conmonPidFile := config.ConmonPidFile
	if conmonPidFile == "" {
		return nil, true, errors.Errorf("conmon PID file path is empty, try to recreate the container with --conmon-pidfile flag")
	}

	createCommand := []string{}
	if config.CreateCommand != nil {
		createCommand = config.CreateCommand
	} else if options.New {
		return nil, true, errors.Errorf("cannot use --new on container %q: no create command found", nameOrID)
	}

	name, serviceName := generateServiceName(ctr, pod, options)
	info := &generate.ContainerInfo{
		ServiceName:       serviceName,
		ContainerName:     name,
		RestartPolicy:     options.RestartPolicy,
		PIDFile:           conmonPidFile,
		StopTimeout:       timeout,
		GenerateTimestamp: true,
		CreateCommand:     createCommand,
	}

	return info, true, nil
}

// generateServiceName generates the container name and the service name for systemd service.
func generateServiceName(ctr *libpod.Container, pod *libpod.Pod, options entities.GenerateSystemdOptions) (string, string) {
	var kind, name, ctrName string
	if pod == nil {
		kind = options.ContainerPrefix //defaults to container
		name = ctr.ID()
		if options.Name {
			name = ctr.Name()
		}
		ctrName = name
	} else {
		kind = options.PodPrefix //defaults to pod
		name = pod.ID()
		ctrName = ctr.ID()
		if options.Name {
			name = pod.Name()
			ctrName = ctr.Name()
		}
	}
	return ctrName, fmt.Sprintf("%s%s%s", kind, options.Separator, name)
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
