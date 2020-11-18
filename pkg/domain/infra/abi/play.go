package abi

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/specgen/generate"
	"github.com/containers/podman/v2/pkg/specgen/generate/kube"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/docker/distribution/reference"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

func (ic *ContainerEngine) PlayKube(ctx context.Context, path string, options entities.PlayKubeOptions) (*entities.PlayKubeReport, error) {
	var (
		kubeObject v1.ObjectReference
	)

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(content, &kubeObject); err != nil {
		return nil, errors.Wrapf(err, "unable to read %q as YAML", path)
	}

	// NOTE: pkg/bindings/play is also parsing the file.
	// A pkg/kube would be nice to refactor and abstract
	// parts of the K8s-related code.
	switch kubeObject.Kind {
	case "Pod":
		var podYAML v1.Pod
		var podTemplateSpec v1.PodTemplateSpec
		if err := yaml.Unmarshal(content, &podYAML); err != nil {
			return nil, errors.Wrapf(err, "unable to read YAML %q as Kube Pod", path)
		}
		podTemplateSpec.ObjectMeta = podYAML.ObjectMeta
		podTemplateSpec.Spec = podYAML.Spec
		return ic.playKubePod(ctx, podTemplateSpec.ObjectMeta.Name, &podTemplateSpec, options)
	case "Deployment":
		var deploymentYAML v1apps.Deployment
		if err := yaml.Unmarshal(content, &deploymentYAML); err != nil {
			return nil, errors.Wrapf(err, "unable to read YAML %q as Kube Deployment", path)
		}
		return ic.playKubeDeployment(ctx, &deploymentYAML, options)
	default:
		return nil, errors.Errorf("invalid YAML kind: %q. [Pod|Deployment] are the only supported Kubernetes Kinds", kubeObject.Kind)
	}

}

func (ic *ContainerEngine) playKubeDeployment(ctx context.Context, deploymentYAML *v1apps.Deployment, options entities.PlayKubeOptions) (*entities.PlayKubeReport, error) {
	var (
		deploymentName string
		podSpec        v1.PodTemplateSpec
		numReplicas    int32
		i              int32
		report         entities.PlayKubeReport
	)

	deploymentName = deploymentYAML.ObjectMeta.Name
	if deploymentName == "" {
		return nil, errors.Errorf("Deployment does not have a name")
	}
	numReplicas = 1
	if deploymentYAML.Spec.Replicas != nil {
		numReplicas = *deploymentYAML.Spec.Replicas
	}
	podSpec = deploymentYAML.Spec.Template

	// create "replicas" number of pods
	for i = 0; i < numReplicas; i++ {
		podName := fmt.Sprintf("%s-pod-%d", deploymentName, i)
		podReport, err := ic.playKubePod(ctx, podName, &podSpec, options)
		if err != nil {
			return nil, errors.Wrapf(err, "error encountered while bringing up pod %s", podName)
		}
		report.Pods = append(report.Pods, podReport.Pods...)
	}
	return &report, nil
}

func (ic *ContainerEngine) playKubePod(ctx context.Context, podName string, podYAML *v1.PodTemplateSpec, options entities.PlayKubeOptions) (*entities.PlayKubeReport, error) {
	var (
		registryCreds *types.DockerAuthConfig
		writer        io.Writer
		playKubePod   entities.PlayKubePod
		report        entities.PlayKubeReport
	)

	// check for name collision between pod and container
	if podName == "" {
		return nil, errors.Errorf("pod does not have a name")
	}
	for _, n := range podYAML.Spec.Containers {
		if n.Name == podName {
			playKubePod.Logs = append(playKubePod.Logs,
				fmt.Sprintf("a container exists with the same name (%q) as the pod in your YAML file; changing pod name to %s_pod\n", podName, podName))
			podName = fmt.Sprintf("%s_pod", podName)
		}
	}

	p, err := kube.ToPodGen(ctx, podName, podYAML)
	if err != nil {
		return nil, err
	}
	if options.Network != "" {
		switch strings.ToLower(options.Network) {
		case "bridge", "host":
			return nil, errors.Errorf("invalid value passed to --network: bridge or host networking must be configured in YAML")
		case "":
			return nil, errors.Errorf("invalid value passed to --network: must provide a comma-separated list of CNI networks")
		default:
			// We'll assume this is a comma-separated list of CNI
			// networks.
			networks := strings.Split(options.Network, ",")
			logrus.Debugf("Pod joining CNI networks: %v", networks)
			p.CNINetworks = append(p.CNINetworks, networks...)
		}
	}

	// Create the Pod
	pod, err := generate.MakePod(p, ic.Libpod)
	if err != nil {
		return nil, err
	}

	podInfraID, err := pod.InfraContainerID()
	if err != nil {
		return nil, err
	}

	if !options.Quiet {
		writer = os.Stderr
	}

	if len(options.Username) > 0 && len(options.Password) > 0 {
		registryCreds = &types.DockerAuthConfig{
			Username: options.Username,
			Password: options.Password,
		}
	}

	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds:         registryCreds,
		DockerCertPath:              options.CertDir,
		DockerInsecureSkipTLSVerify: options.SkipTLSVerify,
	}

	volumes, err := kube.InitializeVolumes(podYAML.Spec.Volumes)
	if err != nil {
		return nil, err
	}

	seccompPaths, err := kube.InitializeSeccompPaths(podYAML.ObjectMeta.Annotations, options.SeccompProfileRoot)
	if err != nil {
		return nil, err
	}

	var ctrRestartPolicy string
	switch podYAML.Spec.RestartPolicy {
	case v1.RestartPolicyAlways:
		ctrRestartPolicy = libpod.RestartPolicyAlways
	case v1.RestartPolicyOnFailure:
		ctrRestartPolicy = libpod.RestartPolicyOnFailure
	case v1.RestartPolicyNever:
		ctrRestartPolicy = libpod.RestartPolicyNo
	default: // Default to Always
		ctrRestartPolicy = libpod.RestartPolicyAlways
	}

	configMaps := []v1.ConfigMap{}
	for _, p := range options.ConfigMaps {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		cm, err := readConfigMapFromFile(f)
		if err != nil {
			return nil, errors.Wrapf(err, "%q", p)
		}

		configMaps = append(configMaps, cm)
	}

	containers := make([]*libpod.Container, 0, len(podYAML.Spec.Containers))
	for _, container := range podYAML.Spec.Containers {
		pullPolicy := util.PullImageMissing
		if len(container.ImagePullPolicy) > 0 {
			pullPolicy, err = util.ValidatePullType(string(container.ImagePullPolicy))
			if err != nil {
				return nil, err
			}
		}
		named, err := reference.ParseNormalizedNamed(container.Image)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse image %q", container.Image)
		}
		// In kube, if the image is tagged with latest, it should always pull
		if tagged, isTagged := named.(reference.NamedTagged); isTagged {
			if tagged.Tag() == image.LatestTag {
				pullPolicy = util.PullImageAlways
			}
		}

		// This ensures the image is the image store
		newImage, err := ic.Libpod.ImageRuntime().New(ctx, container.Image, options.SignaturePolicy, options.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, nil, pullPolicy)
		if err != nil {
			return nil, err
		}

		specGen, err := kube.ToSpecGen(ctx, container, container.Image, newImage, volumes, pod.ID(), podName, podInfraID, configMaps, seccompPaths, ctrRestartPolicy)
		if err != nil {
			return nil, err
		}

		ctr, err := generate.MakeContainer(ctx, ic.Libpod, specGen)
		if err != nil {
			return nil, err
		}
		containers = append(containers, ctr)
	}

	if options.Start != types.OptionalBoolFalse {
		//start the containers
		podStartErrors, err := pod.Start(ctx)
		if err != nil {
			return nil, err
		}

		// Previous versions of playkube started containers individually and then
		// looked for errors.  Because we now use the uber-Pod start call, we should
		// iterate the map of possible errors and return one if there is a problem. This
		// keeps the behavior the same

		for _, e := range podStartErrors {
			if e != nil {
				return nil, e
			}
		}
	}

	playKubePod.ID = pod.ID()
	for _, ctr := range containers {
		playKubePod.Containers = append(playKubePod.Containers, ctr.ID())
	}

	report.Pods = append(report.Pods, playKubePod)

	return &report, nil
}

// readConfigMapFromFile returns a kubernetes configMap obtained from --configmap flag
func readConfigMapFromFile(r io.Reader) (v1.ConfigMap, error) {
	var cm v1.ConfigMap

	content, err := ioutil.ReadAll(r)
	if err != nil {
		return cm, errors.Wrapf(err, "unable to read ConfigMap YAML content")
	}

	if err := yaml.Unmarshal(content, &cm); err != nil {
		return cm, errors.Wrapf(err, "unable to read YAML as Kube ConfigMap")
	}

	if cm.Kind != "ConfigMap" {
		return cm, errors.Errorf("invalid YAML kind: %q. [ConfigMap] is the only supported by --configmap", cm.Kind)
	}

	return cm, nil
}
