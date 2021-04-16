package abi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/libpod/image"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/specgen/generate"
	"github.com/containers/podman/v3/pkg/specgen/generate/kube"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/docker/distribution/reference"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	yamlv3 "gopkg.in/yaml.v3"
	v1apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

func (ic *ContainerEngine) PlayKube(ctx context.Context, path string, options entities.PlayKubeOptions) (*entities.PlayKubeReport, error) {
	report := &entities.PlayKubeReport{}
	validKinds := 0

	// read yaml document
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// split yaml document
	documentList, err := splitMultiDocYAML(content)
	if err != nil {
		return nil, err
	}

	// create pod on each document if it is a pod or deployment
	// any other kube kind will be skipped
	for _, document := range documentList {
		kind, err := getKubeKind(document)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to read %q as kube YAML", path)
		}

		switch kind {
		case "Pod":
			var podYAML v1.Pod
			var podTemplateSpec v1.PodTemplateSpec

			if err := yaml.Unmarshal(document, &podYAML); err != nil {
				return nil, errors.Wrapf(err, "unable to read YAML %q as Kube Pod", path)
			}

			podTemplateSpec.ObjectMeta = podYAML.ObjectMeta
			podTemplateSpec.Spec = podYAML.Spec

			r, err := ic.playKubePod(ctx, podTemplateSpec.ObjectMeta.Name, &podTemplateSpec, options)
			if err != nil {
				return nil, err
			}

			report.Pods = append(report.Pods, r.Pods...)
			validKinds++
		case "Deployment":
			var deploymentYAML v1apps.Deployment

			if err := yaml.Unmarshal(document, &deploymentYAML); err != nil {
				return nil, errors.Wrapf(err, "unable to read YAML %q as Kube Deployment", path)
			}

			r, err := ic.playKubeDeployment(ctx, &deploymentYAML, options)
			if err != nil {
				return nil, err
			}

			report.Pods = append(report.Pods, r.Pods...)
			validKinds++
		default:
			logrus.Infof("kube kind %s not supported", kind)
			continue
		}
	}

	if validKinds == 0 {
		return nil, fmt.Errorf("YAML document does not contain any supported kube kind")
	}

	return report, nil
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
		ctrRestartPolicy = define.RestartPolicyAlways
	case v1.RestartPolicyOnFailure:
		ctrRestartPolicy = define.RestartPolicyOnFailure
	case v1.RestartPolicyNever:
		ctrRestartPolicy = define.RestartPolicyNo
	default: // Default to Always
		ctrRestartPolicy = define.RestartPolicyAlways
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
		// but if the domain is localhost, that means the image was built locally
		// so do not attempt a pull.
		if tagged, isTagged := named.(reference.NamedTagged); isTagged {
			if tagged.Tag() == image.LatestTag && reference.Domain(named) != image.DefaultLocalRegistry {
				pullPolicy = util.PullImageAlways
			}
		}

		// This ensures the image is the image store
		newImage, err := ic.Libpod.ImageRuntime().New(ctx, container.Image, options.SignaturePolicy, options.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, nil, pullPolicy, nil)
		if err != nil {
			return nil, err
		}

		specgenOpts := kube.CtrSpecGenOptions{
			Container:     container,
			Image:         newImage,
			Volumes:       volumes,
			PodID:         pod.ID(),
			PodName:       podName,
			PodInfraID:    podInfraID,
			ConfigMaps:    configMaps,
			SeccompPaths:  seccompPaths,
			RestartPolicy: ctrRestartPolicy,
			NetNSIsHost:   p.NetNS.IsHost(),
			LogDriver:     options.LogDriver,
		}
		specGen, err := kube.ToSpecGen(ctx, &specgenOpts)
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
		// Start the containers
		podStartErrors, err := pod.Start(ctx)
		if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
			return nil, err
		}
		for id, err := range podStartErrors {
			playKubePod.ContainerErrors = append(playKubePod.ContainerErrors, errors.Wrapf(err, "error starting container %s", id).Error())
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

// splitMultiDocYAML reads mutiple documents in a YAML file and
// returns them as a list.
func splitMultiDocYAML(yamlContent []byte) ([][]byte, error) {
	var documentList [][]byte

	d := yamlv3.NewDecoder(bytes.NewReader(yamlContent))
	for {
		var o interface{}
		// read individual document
		err := d.Decode(&o)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrapf(err, "multi doc yaml could not be split")
		}

		if o != nil {
			// back to bytes
			document, err := yamlv3.Marshal(o)
			if err != nil {
				return nil, errors.Wrapf(err, "individual doc yaml could not be marshalled")
			}

			documentList = append(documentList, document)
		}
	}

	return documentList, nil
}

// getKubeKind unmarshals a kube YAML document and returns its kind.
func getKubeKind(obj []byte) (string, error) {
	var kubeObject v1.ObjectReference

	if err := yaml.Unmarshal(obj, &kubeObject); err != nil {
		return "", err
	}

	return kubeObject.Kind, nil
}
