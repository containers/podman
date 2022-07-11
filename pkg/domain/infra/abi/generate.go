package abi

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	k8sAPI "github.com/containers/podman/v4/pkg/k8s.io/api/core/v1"
	"github.com/containers/podman/v4/pkg/systemd/generate"
	"github.com/ghodss/yaml"
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
		err = fmt.Errorf("%v: %w", err.Error(), ctrErr)
		return nil, fmt.Errorf("%s does not refer to a container or pod: %w", nameOrID, err)
	}

	// Generate the units for the pod and all its containers.
	units, err := generate.PodUnits(pod, options)
	if err != nil {
		return nil, err
	}
	return &entities.GenerateSystemdReport{Units: units}, nil
}

func (ic *ContainerEngine) GenerateKube(ctx context.Context, nameOrIDs []string, options entities.GenerateKubeOptions) (*entities.GenerateKubeReport, error) {
	var (
		pods       []*libpod.Pod
		ctrs       []*libpod.Container
		vols       []*libpod.Volume
		podContent [][]byte
		content    [][]byte
	)

	defaultKubeNS := true
	// Lookup for podman objects.
	for _, nameOrID := range nameOrIDs {
		// Let's assume it's a container, so get the container.
		ctr, err := ic.Libpod.LookupContainer(nameOrID)
		if err != nil {
			if !strings.Contains(err.Error(), "no such container") {
				return nil, err
			}
		} else {
			//  now that infra holds NS data, we need to support dependencies.
			// we cannot deal with ctrs already in a pod.
			if len(ctr.PodID()) > 0 {
				return nil, fmt.Errorf("container %s is associated with pod %s: use generate on the pod itself", ctr.ID(), ctr.PodID())
			}
			ctrs = append(ctrs, ctr)
			continue
		}

		// Maybe it's a pod.
		pod, err := ic.Libpod.LookupPod(nameOrID)
		if err != nil {
			if !strings.Contains(err.Error(), "no such pod") {
				return nil, err
			}
		} else {
			// Get the pod config to see if the user has modified the default
			// namespace sharing values as this might affect the pods when run
			// in a k8s cluster
			podConfig, err := pod.Config()
			if err != nil {
				return nil, err
			}
			if !(podConfig.UsePodIPC && podConfig.UsePodNet && podConfig.UsePodUTS) {
				defaultKubeNS = false
			}

			pods = append(pods, pod)
			continue
		}

		// Or volume.
		vol, err := ic.Libpod.LookupVolume(nameOrID)
		if err != nil {
			if !strings.Contains(err.Error(), "no such volume") {
				return nil, err
			}
		} else {
			vols = append(vols, vol)
			continue
		}

		// If it reaches here is because the name or id did not exist.
		return nil, fmt.Errorf("name or ID %q not found", nameOrID)
	}

	if !defaultKubeNS {
		warning := `
# NOTE: The namespace sharing for a pod has been modified by the user and is not the same as the
# default settings for kubernetes. This can lead to unexpected behavior when running the generated
# kube yaml in a kubernetes cluster.
`
		content = append(content, []byte(warning))
	}

	// Generate kube persistent volume claims from volumes.
	if len(vols) >= 1 {
		pvs, err := getKubePVCs(vols)
		if err != nil {
			return nil, err
		}

		content = append(content, pvs...)
	}

	// Generate kube pods and services from pods.
	if len(pods) >= 1 {
		pos, svcs, err := getKubePods(ctx, pods, options.Service)
		if err != nil {
			return nil, err
		}

		podContent = append(podContent, pos...)
		if options.Service {
			content = append(content, svcs...)
		}
	}

	// Generate the kube pods from containers.
	if len(ctrs) >= 1 {
		po, err := libpod.GenerateForKube(ctx, ctrs)
		if err != nil {
			return nil, err
		}
		if len(po.Spec.Volumes) != 0 {
			warning := `
# NOTE: If you generated this yaml from an unprivileged and rootless podman container on an SELinux
# enabled system, check the podman generate kube man page for steps to follow to ensure that your pod/container
# has the right permissions to access the volumes added.
`
			content = append(content, []byte(warning))
		}
		b, err := generateKubeYAML(libpod.ConvertV1PodToYAMLPod(po))
		if err != nil {
			return nil, err
		}

		podContent = append(podContent, b)
		if options.Service {
			svc, err := libpod.GenerateKubeServiceFromV1Pod(po, []k8sAPI.ServicePort{})
			if err != nil {
				return nil, err
			}
			b, err := generateKubeYAML(svc)
			if err != nil {
				return nil, err
			}
			content = append(content, b)
		}
	}

	// Content order is based on helm install order (secret, persistentVolumeClaim, service, pod).
	content = append(content, podContent...)

	// Generate kube YAML file from all kube kinds.
	k, err := generateKubeOutput(content)
	if err != nil {
		return nil, err
	}

	return &entities.GenerateKubeReport{Reader: bytes.NewReader(k)}, nil
}

// getKubePods returns kube pod and service YAML files from podman pods.
func getKubePods(ctx context.Context, pods []*libpod.Pod, getService bool) ([][]byte, [][]byte, error) {
	pos := [][]byte{}
	svcs := [][]byte{}

	for _, p := range pods {
		po, sp, err := p.GenerateForKube(ctx)
		if err != nil {
			return nil, nil, err
		}

		b, err := generateKubeYAML(po)
		if err != nil {
			return nil, nil, err
		}
		pos = append(pos, b)

		if getService {
			svc, err := libpod.GenerateKubeServiceFromV1Pod(po, sp)
			if err != nil {
				return nil, nil, err
			}
			b, err := generateKubeYAML(svc)
			if err != nil {
				return nil, nil, err
			}
			svcs = append(svcs, b)
		}
	}

	return pos, svcs, nil
}

// getKubePVCs returns kube persistent volume claim YAML files from podman volumes.
func getKubePVCs(volumes []*libpod.Volume) ([][]byte, error) {
	pvs := [][]byte{}

	for _, v := range volumes {
		b, err := generateKubeYAML(v.GenerateForKube())
		if err != nil {
			return nil, err
		}
		pvs = append(pvs, b)
	}

	return pvs, nil
}

// generateKubeYAML marshalls a kube kind into a YAML file.
func generateKubeYAML(kubeKind interface{}) ([]byte, error) {
	b, err := yaml.Marshal(kubeKind)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// generateKubeOutput generates kube YAML file containing multiple kube kinds.
func generateKubeOutput(content [][]byte) ([]byte, error) {
	output := make([]byte, 0)

	header := `# Save the output of this file and use kubectl create -f to import
# it into Kubernetes.
#
# Created with podman-%s
`
	podmanVersion, err := define.GetVersion()
	if err != nil {
		return nil, err
	}

	// Add header to kube YAML file.
	output = append(output, []byte(fmt.Sprintf(header, podmanVersion.Version))...)

	// kube generate order is based on helm install order (secret, persistentVolume, service, pod...).
	// Add kube kinds.
	for i, b := range content {
		if i != 0 {
			output = append(output, []byte("---\n")...)
		}

		output = append(output, b...)
	}

	return output, nil
}
