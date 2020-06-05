package generate

import (
	"fmt"
	"strings"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/pkg/errors"
)

// PodUnits generates systemd units for the specified pod and its containers.
// Based on the options, the return value might be the content of all units or
// the files they been written to.
func PodUnits(pod *libpod.Pod, options entities.GenerateSystemdOptions) (string, error) {
	if options.New {
		return "", errors.New("--new is not supported for pods")
	}

	// Error out if the pod has no infra container, which we require to be the
	// main service.
	if !pod.HasInfraContainer() {
		return "", fmt.Errorf("error generating systemd unit files: Pod %q has no infra container", pod.Name())
	}

	podInfo, err := generatePodInfo(pod, options)
	if err != nil {
		return "", err
	}

	infraID, err := pod.InfraContainerID()
	if err != nil {
		return "", err
	}

	// Compute the container-dependency graph for the Pod.
	containers, err := pod.AllContainers()
	if err != nil {
		return "", err
	}
	if len(containers) == 0 {
		return "", fmt.Errorf("error generating systemd unit files: Pod %q has no containers", pod.Name())
	}
	graph, err := libpod.BuildContainerGraph(containers)
	if err != nil {
		return "", err
	}

	// Traverse the dependency graph and create systemdgen.containerInfo's for
	// each container.
	containerInfos := []*containerInfo{podInfo}
	for ctr, dependencies := range graph.DependencyMap() {
		// Skip the infra container as we already generated it.
		if ctr.ID() == infraID {
			continue
		}
		ctrInfo, err := generateContainerInfo(ctr, options)
		if err != nil {
			return "", err
		}
		// Now add the container's dependencies and at the container as a
		// required service of the infra container.
		for _, dep := range dependencies {
			if dep.ID() == infraID {
				ctrInfo.BoundToServices = append(ctrInfo.BoundToServices, podInfo.ServiceName)
			} else {
				_, serviceName := containerServiceName(dep, options)
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
		out, err := createContainerSystemdUnit(info, options)
		if err != nil {
			return "", err
		}
		builder.WriteString(out)
	}

	return builder.String(), nil
}

func generatePodInfo(pod *libpod.Pod, options entities.GenerateSystemdOptions) (*containerInfo, error) {
	// Generate a systemdgen.containerInfo for the infra container. This
	// containerInfo acts as the main service of the pod.
	infraCtr, err := pod.InfraContainer()
	if err != nil {
		return nil, errors.Wrap(err, "could not find infra container")
	}

	timeout := infraCtr.StopTimeout()
	if options.StopTimeout != nil {
		timeout = *options.StopTimeout
	}

	config := infraCtr.Config()
	conmonPidFile := config.ConmonPidFile
	if conmonPidFile == "" {
		return nil, errors.Errorf("conmon PID file path is empty, try to recreate the container with --conmon-pidfile flag")
	}

	createCommand := []string{}

	nameOrID := pod.ID()
	ctrNameOrID := infraCtr.ID()
	if options.Name {
		nameOrID = pod.Name()
		ctrNameOrID = infraCtr.Name()
	}
	serviceName := fmt.Sprintf("%s%s%s", options.PodPrefix, options.Separator, nameOrID)

	info := containerInfo{
		ServiceName:       serviceName,
		ContainerNameOrID: ctrNameOrID,
		RestartPolicy:     options.RestartPolicy,
		PIDFile:           conmonPidFile,
		StopTimeout:       timeout,
		GenerateTimestamp: true,
		CreateCommand:     createCommand,
	}
	return &info, nil
}
