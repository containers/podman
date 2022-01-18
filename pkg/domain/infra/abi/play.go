package abi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/common/libimage"
	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/autoupdate"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgen/generate"
	"github.com/containers/podman/v4/pkg/specgen/generate/kube"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/containers/podman/v4/pkg/util"
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

	// sort kube kinds
	documentList, err = sortKubeKinds(documentList)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to sort kube kinds in %q", path)
	}

	ipIndex := 0

	var configMaps []v1.ConfigMap

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

			r, err := ic.playKubePod(ctx, podTemplateSpec.ObjectMeta.Name, &podTemplateSpec, options, &ipIndex, podYAML.Annotations, configMaps)
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

			r, err := ic.playKubeDeployment(ctx, &deploymentYAML, options, &ipIndex, configMaps)
			if err != nil {
				return nil, err
			}

			report.Pods = append(report.Pods, r.Pods...)
			validKinds++
		case "PersistentVolumeClaim":
			var pvcYAML v1.PersistentVolumeClaim

			if err := yaml.Unmarshal(document, &pvcYAML); err != nil {
				return nil, errors.Wrapf(err, "unable to read YAML %q as Kube PersistentVolumeClaim", path)
			}

			r, err := ic.playKubePVC(ctx, &pvcYAML, options)
			if err != nil {
				return nil, err
			}

			report.Volumes = append(report.Volumes, r.Volumes...)
			validKinds++
		case "ConfigMap":
			var configMap v1.ConfigMap

			if err := yaml.Unmarshal(document, &configMap); err != nil {
				return nil, errors.Wrapf(err, "unable to read YAML %q as Kube ConfigMap", path)
			}
			configMaps = append(configMaps, configMap)
		default:
			logrus.Infof("Kube kind %s not supported", kind)
			continue
		}
	}

	if validKinds == 0 {
		return nil, fmt.Errorf("YAML document does not contain any supported kube kind")
	}

	return report, nil
}

func (ic *ContainerEngine) playKubeDeployment(ctx context.Context, deploymentYAML *v1apps.Deployment, options entities.PlayKubeOptions, ipIndex *int, configMaps []v1.ConfigMap) (*entities.PlayKubeReport, error) {
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
		podReport, err := ic.playKubePod(ctx, podName, &podSpec, options, ipIndex, deploymentYAML.Annotations, configMaps)
		if err != nil {
			return nil, errors.Wrapf(err, "error encountered while bringing up pod %s", podName)
		}
		report.Pods = append(report.Pods, podReport.Pods...)
	}
	return &report, nil
}

func (ic *ContainerEngine) playKubePod(ctx context.Context, podName string, podYAML *v1.PodTemplateSpec, options entities.PlayKubeOptions, ipIndex *int, annotations map[string]string, configMaps []v1.ConfigMap) (*entities.PlayKubeReport, error) {
	var (
		writer      io.Writer
		playKubePod entities.PlayKubePod
		report      entities.PlayKubeReport
	)

	// Create the secret manager before hand
	secretsManager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, err
	}

	// Assert the pod has a name
	if podName == "" {
		return nil, errors.Errorf("pod does not have a name")
	}

	podOpt := entities.PodCreateOptions{Infra: true, Net: &entities.NetOptions{NoHosts: options.NoHosts}}
	podOpt, err = kube.ToPodOpt(ctx, podName, podOpt, podYAML)
	if err != nil {
		return nil, err
	}

	ns, networks, netOpts, err := specgen.ParseNetworkFlag(options.Networks)
	if err != nil {
		return nil, err
	}

	if (ns.IsBridge() && len(networks) == 0) || ns.IsHost() {
		return nil, errors.Errorf("invalid value passed to --network: bridge or host networking must be configured in YAML")
	}

	podOpt.Net.Network = ns
	podOpt.Net.Networks = networks
	podOpt.Net.NetworkOptions = netOpts

	// FIXME This is very hard to support properly with a good ux
	if len(options.StaticIPs) > *ipIndex {
		if !podOpt.Net.Network.IsBridge() {
			errors.Wrap(define.ErrInvalidArg, "static ip addresses can only be set when the network mode is bridge")
		}
		if len(podOpt.Net.Networks) != 1 {
			return nil, errors.Wrap(define.ErrInvalidArg, "cannot set static ip addresses for more than network, use netname:ip=<ip> syntax to specify ips for more than network")
		}
		for name, netOpts := range podOpt.Net.Networks {
			netOpts.StaticIPs = append(netOpts.StaticIPs, options.StaticIPs[*ipIndex])
			podOpt.Net.Networks[name] = netOpts
		}
	} else if len(options.StaticIPs) > 0 {
		// only warn if the user has set at least one ip
		logrus.Warn("No more static ips left using a random one")
	}
	if len(options.StaticMACs) > *ipIndex {
		if !podOpt.Net.Network.IsBridge() {
			errors.Wrap(define.ErrInvalidArg, "static mac address can only be set when the network mode is bridge")
		}
		if len(podOpt.Net.Networks) != 1 {
			return nil, errors.Wrap(define.ErrInvalidArg, "cannot set static mac address for more than network, use netname:mac=<mac> syntax to specify mac for more than network")
		}
		for name, netOpts := range podOpt.Net.Networks {
			netOpts.StaticMAC = nettypes.HardwareAddr(options.StaticMACs[*ipIndex])
			podOpt.Net.Networks[name] = netOpts
		}
	} else if len(options.StaticIPs) > 0 {
		// only warn if the user has set at least one mac
		logrus.Warn("No more static macs left using a random one")
	}
	*ipIndex++

	p := specgen.NewPodSpecGenerator()
	if err != nil {
		return nil, err
	}

	p, err = entities.ToPodSpecGen(*p, &podOpt)
	if err != nil {
		return nil, err
	}
	podSpec := entities.PodSpec{PodSpecGen: *p}

	configMapIndex := make(map[string]struct{})
	for _, configMap := range configMaps {
		configMapIndex[configMap.Name] = struct{}{}
	}
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

		if _, present := configMapIndex[cm.Name]; present {
			return nil, errors.Errorf("ambiguous configuration: the same config map %s is present in YAML and in --configmaps %s file", cm.Name, p)
		}

		configMaps = append(configMaps, cm)
	}

	volumes, err := kube.InitializeVolumes(podYAML.Spec.Volumes, configMaps)
	if err != nil {
		return nil, err
	}

	// Go through the volumes and create a podman volume for all volumes that have been
	// defined by a configmap
	for _, v := range volumes {
		if v.Type == kube.KubeVolumeTypeConfigMap && !v.Optional {
			vol, err := ic.Libpod.NewVolume(ctx, libpod.WithVolumeName(v.Source))
			if err != nil {
				return nil, errors.Wrapf(err, "cannot create a local volume for volume from configmap %q", v.Source)
			}
			mountPoint, err := vol.MountPoint()
			if err != nil || mountPoint == "" {
				return nil, errors.Wrapf(err, "unable to get mountpoint of volume %q", vol.Name())
			}
			// Create files and add data to the volume mountpoint based on the Items in the volume
			for k, v := range v.Items {
				dataPath := filepath.Join(mountPoint, k)
				f, err := os.Create(dataPath)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot create file %q at volume mountpoint %q", k, mountPoint)
				}
				defer f.Close()
				_, err = f.WriteString(v)
				if err != nil {
					return nil, err
				}
			}
		}
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

	if podOpt.Infra {
		infraImage := util.DefaultContainerConfig().Engine.InfraImage
		infraOptions := entities.NewInfraContainerCreateOptions()
		infraOptions.Hostname = podSpec.PodSpecGen.PodBasicConfig.Hostname
		podSpec.PodSpecGen.InfraImage = infraImage
		podSpec.PodSpecGen.NoInfra = false
		podSpec.PodSpecGen.InfraContainerSpec = specgen.NewSpecGenerator(infraImage, false)
		podSpec.PodSpecGen.InfraContainerSpec.NetworkOptions = p.NetworkOptions

		err = specgenutil.FillOutSpecGen(podSpec.PodSpecGen.InfraContainerSpec, &infraOptions, []string{})
		if err != nil {
			return nil, err
		}
	}

	// Create the Pod
	pod, err := generate.MakePod(&podSpec, ic.Libpod)
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

	containers := make([]*libpod.Container, 0, len(podYAML.Spec.Containers))
	initContainers := make([]*libpod.Container, 0, len(podYAML.Spec.InitContainers))
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	for _, initCtr := range podYAML.Spec.InitContainers {
		// Init containers cannot have either of lifecycle, livenessProbe, readinessProbe, or startupProbe set
		if initCtr.Lifecycle != nil || initCtr.LivenessProbe != nil || initCtr.ReadinessProbe != nil || initCtr.StartupProbe != nil {
			return nil, errors.Errorf("cannot create an init container that has either of lifecycle, livenessProbe, readinessProbe, or startupProbe set")
		}
		pulledImage, labels, err := ic.getImageAndLabelInfo(ctx, cwd, annotations, writer, initCtr, options)
		if err != nil {
			return nil, err
		}
		specgenOpts := kube.CtrSpecGenOptions{
			Annotations:       annotations,
			Container:         initCtr,
			Image:             pulledImage,
			Volumes:           volumes,
			PodID:             pod.ID(),
			PodName:           podName,
			PodInfraID:        podInfraID,
			ConfigMaps:        configMaps,
			SeccompPaths:      seccompPaths,
			RestartPolicy:     ctrRestartPolicy,
			NetNSIsHost:       p.NetNS.IsHost(),
			SecretsManager:    secretsManager,
			LogDriver:         options.LogDriver,
			LogOptions:        options.LogOptions,
			Labels:            labels,
			InitContainerType: define.AlwaysInitContainer,
		}
		specGen, err := kube.ToSpecGen(ctx, &specgenOpts)
		if err != nil {
			return nil, err
		}
		rtSpec, spec, opts, err := generate.MakeContainer(ctx, ic.Libpod, specGen)
		if err != nil {
			return nil, err
		}
		ctr, err := generate.ExecuteCreate(ctx, ic.Libpod, rtSpec, spec, false, opts...)
		if err != nil {
			return nil, err
		}

		initContainers = append(initContainers, ctr)
	}
	for _, container := range podYAML.Spec.Containers {
		if !strings.Contains("infra", container.Name) {
			pulledImage, labels, err := ic.getImageAndLabelInfo(ctx, cwd, annotations, writer, container, options)
			if err != nil {
				return nil, err
			}

			specgenOpts := kube.CtrSpecGenOptions{
				Container:      container,
				Image:          pulledImage,
				Volumes:        volumes,
				PodID:          pod.ID(),
				PodName:        podName,
				PodInfraID:     podInfraID,
				ConfigMaps:     configMaps,
				SeccompPaths:   seccompPaths,
				RestartPolicy:  ctrRestartPolicy,
				NetNSIsHost:    p.NetNS.IsHost(),
				SecretsManager: secretsManager,
				LogDriver:      options.LogDriver,
				LogOptions:     options.LogOptions,
				Labels:         labels,
			}
			specGen, err := kube.ToSpecGen(ctx, &specgenOpts)
			if err != nil {
				return nil, err
			}
			rtSpec, spec, opts, err := generate.MakeContainer(ctx, ic.Libpod, specGen)
			if err != nil {
				return nil, err
			}
			ctr, err := generate.ExecuteCreate(ctx, ic.Libpod, rtSpec, spec, false, opts...)
			if err != nil {
				return nil, err
			}
			containers = append(containers, ctr)
		}
	}

	if options.Start != types.OptionalBoolFalse {
		// Start the containers
		podStartErrors, err := pod.Start(ctx)
		if err != nil && errors.Cause(err) != define.ErrPodPartialFail {
			return nil, err
		}
		for id, err := range podStartErrors {
			playKubePod.ContainerErrors = append(playKubePod.ContainerErrors, errors.Wrapf(err, "error starting container %s", id).Error())
			fmt.Println(playKubePod.ContainerErrors)
		}
	}

	playKubePod.ID = pod.ID()
	for _, ctr := range containers {
		playKubePod.Containers = append(playKubePod.Containers, ctr.ID())
	}
	for _, initCtr := range initContainers {
		playKubePod.InitContainers = append(playKubePod.InitContainers, initCtr.ID())
	}

	report.Pods = append(report.Pods, playKubePod)

	return &report, nil
}

// getImageAndLabelInfo returns the image information and how the image should be pulled plus as well as labels to be used for the container in the pod.
// Moved this to a separate function so that it can be used for both init and regular containers when playing a kube yaml.
func (ic *ContainerEngine) getImageAndLabelInfo(ctx context.Context, cwd string, annotations map[string]string, writer io.Writer, container v1.Container, options entities.PlayKubeOptions) (*libimage.Image, map[string]string, error) {
	// Contains all labels obtained from kube
	labels := make(map[string]string)
	var pulledImage *libimage.Image
	buildFile, err := getBuildFile(container.Image, cwd)
	if err != nil {
		return nil, nil, err
	}
	existsLocally, err := ic.Libpod.LibimageRuntime().Exists(container.Image)
	if err != nil {
		return nil, nil, err
	}
	if (len(buildFile) > 0 && !existsLocally) || (len(buildFile) > 0 && options.Build) {
		buildOpts := new(buildahDefine.BuildOptions)
		commonOpts := new(buildahDefine.CommonBuildOptions)
		buildOpts.ConfigureNetwork = buildahDefine.NetworkDefault
		buildOpts.Isolation = buildahDefine.IsolationChroot
		buildOpts.CommonBuildOpts = commonOpts
		buildOpts.Output = container.Image
		buildOpts.ContextDirectory = filepath.Dir(buildFile)
		if _, _, err := ic.Libpod.Build(ctx, *buildOpts, []string{buildFile}...); err != nil {
			return nil, nil, err
		}
		i, _, err := ic.Libpod.LibimageRuntime().LookupImage(container.Image, new(libimage.LookupImageOptions))
		if err != nil {
			return nil, nil, err
		}
		pulledImage = i
	} else {
		// NOTE: set the pull policy to "newer".  This will cover cases
		// where the "latest" tag requires a pull and will also
		// transparently handle "localhost/" prefixed files which *may*
		// refer to a locally built image OR an image running a
		// registry on localhost.
		pullPolicy := config.PullPolicyNewer
		if len(container.ImagePullPolicy) > 0 {
			// Make sure to lower the strings since K8s pull policy
			// may be capitalized (see bugzilla.redhat.com/show_bug.cgi?id=1985905).
			rawPolicy := string(container.ImagePullPolicy)
			pullPolicy, err = config.ParsePullPolicy(strings.ToLower(rawPolicy))
			if err != nil {
				return nil, nil, err
			}
		}
		// This ensures the image is the image store
		pullOptions := &libimage.PullOptions{}
		pullOptions.AuthFilePath = options.Authfile
		pullOptions.CertDirPath = options.CertDir
		pullOptions.SignaturePolicyPath = options.SignaturePolicy
		pullOptions.Writer = writer
		pullOptions.Username = options.Username
		pullOptions.Password = options.Password
		pullOptions.InsecureSkipTLSVerify = options.SkipTLSVerify

		pulledImages, err := ic.Libpod.LibimageRuntime().Pull(ctx, container.Image, pullPolicy, pullOptions)
		if err != nil {
			return nil, nil, err
		}
		pulledImage = pulledImages[0]
	}

	// Handle kube annotations
	for k, v := range annotations {
		switch k {
		// Auto update annotation without container name will apply to
		// all containers within the pod
		case autoupdate.Label, autoupdate.AuthfileLabel:
			labels[k] = v
		// Auto update annotation with container name will apply only
		// to the specified container
		case fmt.Sprintf("%s/%s", autoupdate.Label, container.Name),
			fmt.Sprintf("%s/%s", autoupdate.AuthfileLabel, container.Name):
			prefixAndCtr := strings.Split(k, "/")
			labels[prefixAndCtr[0]] = v
		}
	}

	return pulledImage, labels, nil
}

// playKubePVC creates a podman volume from a kube persistent volume claim.
func (ic *ContainerEngine) playKubePVC(ctx context.Context, pvcYAML *v1.PersistentVolumeClaim, options entities.PlayKubeOptions) (*entities.PlayKubeReport, error) {
	var report entities.PlayKubeReport
	opts := make(map[string]string)

	// Get pvc name.
	// This is the only required pvc attribute to create a podman volume.
	name := pvcYAML.GetName()
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("persistent volume claim name can not be empty")
	}

	// Create podman volume options.
	volOptions := []libpod.VolumeCreateOption{
		libpod.WithVolumeName(name),
		libpod.WithVolumeLabels(pvcYAML.GetLabels()),
	}

	// Get pvc annotations and create remaining podman volume options if available.
	// These are podman volume options that do not match any of the persistent volume claim
	// attributes, so they can be configured using annotations since they will not affect k8s.
	for k, v := range pvcYAML.GetAnnotations() {
		switch k {
		case util.VolumeDriverAnnotation:
			volOptions = append(volOptions, libpod.WithVolumeDriver(v))
		case util.VolumeDeviceAnnotation:
			opts["device"] = v
		case util.VolumeTypeAnnotation:
			opts["type"] = v
		case util.VolumeUIDAnnotation:
			uid, err := strconv.Atoi(v)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot convert uid %s to integer", v)
			}
			volOptions = append(volOptions, libpod.WithVolumeUID(uid))
			opts["UID"] = v
		case util.VolumeGIDAnnotation:
			gid, err := strconv.Atoi(v)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot convert gid %s to integer", v)
			}
			volOptions = append(volOptions, libpod.WithVolumeGID(gid))
			opts["GID"] = v
		case util.VolumeMountOptsAnnotation:
			opts["o"] = v
		}
	}
	volOptions = append(volOptions, libpod.WithVolumeOptions(opts))

	// Create volume.
	vol, err := ic.Libpod.NewVolume(ctx, volOptions...)
	if err != nil {
		return nil, err
	}

	report.Volumes = append(report.Volumes, entities.PlayKubeVolume{
		Name: vol.Name(),
	})

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

// splitMultiDocYAML reads multiple documents in a YAML file and
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

// sortKubeKinds adds the correct creation order for the kube kinds.
// Any pod dependency will be created first like volumes, secrets, etc.
func sortKubeKinds(documentList [][]byte) ([][]byte, error) {
	var sortedDocumentList [][]byte

	for _, document := range documentList {
		kind, err := getKubeKind(document)
		if err != nil {
			return nil, err
		}

		switch kind {
		case "Pod", "Deployment":
			sortedDocumentList = append(sortedDocumentList, document)
		default:
			sortedDocumentList = append([][]byte{document}, sortedDocumentList...)
		}
	}

	return sortedDocumentList, nil
}
func imageNamePrefix(imageName string) string {
	prefix := imageName
	s := strings.Split(prefix, ":")
	if len(s) > 0 {
		prefix = s[0]
	}
	s = strings.Split(prefix, "/")
	if len(s) > 0 {
		prefix = s[len(s)-1]
	}
	s = strings.Split(prefix, "@")
	if len(s) > 0 {
		prefix = s[0]
	}
	return prefix
}

func getBuildFile(imageName string, cwd string) (string, error) {
	buildDirName := imageNamePrefix(imageName)
	containerfilePath := filepath.Join(cwd, buildDirName, "Containerfile")
	dockerfilePath := filepath.Join(cwd, buildDirName, "Dockerfile")

	_, err := os.Stat(containerfilePath)
	if err == nil {
		logrus.Debugf("Building %s with %s", imageName, containerfilePath)
		return containerfilePath, nil
	}
	// If the error is not because the file does not exist, take
	// a mulligan and try Dockerfile.  If that also fails, return that
	// error
	if err != nil && !os.IsNotExist(err) {
		logrus.Error(err.Error())
	}

	_, err = os.Stat(filepath.Join(dockerfilePath))
	if err == nil {
		logrus.Debugf("Building %s with %s", imageName, dockerfilePath)
		return dockerfilePath, nil
	}
	// Strike two
	if os.IsNotExist(err) {
		return "", nil
	}
	return "", err
}

func (ic *ContainerEngine) PlayKubeDown(ctx context.Context, path string, _ entities.PlayKubeDownOptions) (*entities.PlayKubeReport, error) {
	var (
		podNames []string
	)
	reports := new(entities.PlayKubeReport)

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

	// sort kube kinds
	documentList, err = sortKubeKinds(documentList)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to sort kube kinds in %q", path)
	}

	for _, document := range documentList {
		kind, err := getKubeKind(document)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to read %q as kube YAML", path)
		}

		switch kind {
		case "Pod":
			var podYAML v1.Pod
			if err := yaml.Unmarshal(document, &podYAML); err != nil {
				return nil, errors.Wrapf(err, "unable to read YAML %q as Kube Pod", path)
			}
			podNames = append(podNames, podYAML.ObjectMeta.Name)
		case "Deployment":
			var deploymentYAML v1apps.Deployment

			if err := yaml.Unmarshal(document, &deploymentYAML); err != nil {
				return nil, errors.Wrapf(err, "unable to read YAML %q as Kube Deployment", path)
			}
			var numReplicas int32 = 1
			deploymentName := deploymentYAML.ObjectMeta.Name
			if deploymentYAML.Spec.Replicas != nil {
				numReplicas = *deploymentYAML.Spec.Replicas
			}
			for i := 0; i < int(numReplicas); i++ {
				podName := fmt.Sprintf("%s-pod-%d", deploymentName, i)
				podNames = append(podNames, podName)
			}
		default:
			continue
		}
	}

	// Add the reports
	reports.StopReport, err = ic.PodStop(ctx, podNames, entities.PodStopOptions{})
	if err != nil {
		return nil, err
	}

	reports.RmReport, err = ic.PodRm(ctx, podNames, entities.PodRmOptions{})
	if err != nil {
		return nil, err
	}
	return reports, nil
}
