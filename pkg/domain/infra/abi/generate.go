package abi

import (
	"bytes"
	"context"
	"fmt"

	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/systemd/generate"
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

func (ic *ContainerEngine) GenerateKube(ctx context.Context, nameOrIDs []string, options entities.GenerateKubeOptions) (*entities.GenerateKubeReport, error) {
	var (
		pods         []*libpod.Pod
		ctrs         []*libpod.Container
		kubePods     []*k8sAPI.Pod
		kubeServices []k8sAPI.Service
		content      []byte
	)
	for _, nameOrID := range nameOrIDs {
		// Get the container in question
		ctr, err := ic.Libpod.LookupContainer(nameOrID)
		if err != nil {
			pod, err := ic.Libpod.LookupPod(nameOrID)
			if err != nil {
				return nil, err
			}
			pods = append(pods, pod)
		} else {
			if len(ctr.Dependencies()) > 0 {
				return nil, errors.Wrapf(define.ErrNotImplemented, "containers with dependencies")
			}
			// we cannot deal with ctrs already in a pod
			if len(ctr.PodID()) > 0 {
				return nil, errors.Errorf("container %s is associated with pod %s: use generate on the pod itself", ctr.ID(), ctr.PodID())
			}
			ctrs = append(ctrs, ctr)
		}
	}

	// check our inputs
	if len(pods) > 0 && len(ctrs) > 0 {
		return nil, errors.New("cannot generate pods and containers at the same time")
	}

	if len(pods) >= 1 {
		pos, svcs, err := getKubePods(pods, options.Service)
		if err != nil {
			return nil, err
		}

		kubePods = append(kubePods, pos...)
		if options.Service {
			kubeServices = append(kubeServices, svcs...)
		}
	} else {
		po, err := libpod.GenerateForKube(ctrs)
		if err != nil {
			return nil, err
		}

		kubePods = append(kubePods, po)
		if options.Service {
			kubeServices = append(kubeServices, libpod.GenerateKubeServiceFromV1Pod(po, []k8sAPI.ServicePort{}))
		}
	}

	content, err := generateKubeOutput(kubePods, kubeServices, options.Service)
	if err != nil {
		return nil, err
	}

	return &entities.GenerateKubeReport{Reader: bytes.NewReader(content)}, nil
}

func getKubePods(pods []*libpod.Pod, getService bool) ([]*k8sAPI.Pod, []k8sAPI.Service, error) {
	kubePods := make([]*k8sAPI.Pod, 0)
	kubeServices := make([]k8sAPI.Service, 0)

	for _, p := range pods {
		po, svc, err := p.GenerateForKube()
		if err != nil {
			return nil, nil, err
		}

		kubePods = append(kubePods, po)
		if getService {
			kubeServices = append(kubeServices, libpod.GenerateKubeServiceFromV1Pod(po, svc))
		}
	}

	return kubePods, kubeServices, nil
}

func generateKubeOutput(kubePods []*k8sAPI.Pod, kubeServices []k8sAPI.Service, hasService bool) ([]byte, error) {
	output := make([]byte, 0)
	marshalledPods := make([]byte, 0)
	marshalledServices := make([]byte, 0)

	for i, p := range kubePods {
		if i != 0 {
			marshalledPods = append(marshalledPods, []byte("---\n")...)
		}

		b, err := yaml.Marshal(p)
		if err != nil {
			return nil, err
		}

		marshalledPods = append(marshalledPods, b...)
	}

	if hasService {
		for i, s := range kubeServices {
			if i != 0 {
				marshalledServices = append(marshalledServices, []byte("---\n")...)
			}

			b, err := yaml.Marshal(s)
			if err != nil {
				return nil, err
			}

			marshalledServices = append(marshalledServices, b...)
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
	// kube generate order is based on helm install order (service, pod...)
	if hasService {
		output = append(output, marshalledServices...)
		output = append(output, []byte("---\n")...)
	}
	output = append(output, marshalledPods...)

	return output, nil
}
