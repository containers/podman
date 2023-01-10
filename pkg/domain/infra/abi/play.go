package abi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/common/libimage"
	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/secrets"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/cmd/podman/parse"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	v1apps "github.com/containers/podman/v4/pkg/k8s.io/api/apps/v1"
	v1 "github.com/containers/podman/v4/pkg/k8s.io/api/core/v1"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgen/generate"
	"github.com/containers/podman/v4/pkg/specgen/generate/kube"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/containers/podman/v4/pkg/systemd/notifyproxy"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	"github.com/coreos/go-systemd/v22/daemon"
	"github.com/ghodss/yaml"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	yamlv3 "gopkg.in/yaml.v3"
)

// sdNotifyAnnotation allows for configuring service-global and
// container-specific sd-notify modes.
const sdNotifyAnnotation = "io.containers.sdnotify"

// default network created/used by kube
const kubeDefaultNetwork = "podman-default-kube-network"

// createServiceContainer creates a container that can later on
// be associated with the pods of a K8s yaml.  It will be started along with
// the first pod.
func (ic *ContainerEngine) createServiceContainer(ctx context.Context, name string, options entities.PlayKubeOptions) (*libpod.Container, error) {
	// Make sure to replace the service container as well if requested by
	// the user.
	if options.Replace {
		if _, err := ic.ContainerRm(ctx, []string{name}, entities.RmOptions{Force: true, Ignore: true}); err != nil {
			return nil, fmt.Errorf("replacing service container: %w", err)
		}
	}

	// Similar to infra containers, a service container is using the pause image.
	image, err := generate.PullOrBuildInfraImage(ic.Libpod, "")
	if err != nil {
		return nil, fmt.Errorf("image for service container: %w", err)
	}

	ctrOpts := entities.ContainerCreateOptions{
		// Inherited from infra containers
		ImageVolume:      "bind",
		IsInfra:          false,
		MemorySwappiness: -1,
		ReadOnly:         true,
		ReadWriteTmpFS:   false,
		// No need to spin up slirp etc.
		Net: &entities.NetOptions{Network: specgen.Namespace{NSMode: specgen.NoNetwork}},
	}

	// Create and fill out the runtime spec.
	s := specgen.NewSpecGenerator(image, false)
	if err := specgenutil.FillOutSpecGen(s, &ctrOpts, []string{}); err != nil {
		return nil, fmt.Errorf("completing spec for service container: %w", err)
	}
	s.Name = name

	runtimeSpec, spec, opts, err := generate.MakeContainer(ctx, ic.Libpod, s, false, nil)
	if err != nil {
		return nil, fmt.Errorf("creating runtime spec for service container: %w", err)
	}
	opts = append(opts, libpod.WithIsService())

	// Set the sd-notify mode to "ignore".  Podman is responsible for
	// sending the notify messages when all containers are ready.
	// The mode for individual containers or entire pods can be configured
	// via the `sdNotifyAnnotation` annotation in the K8s YAML.
	opts = append(opts, libpod.WithSdNotifyMode(define.SdNotifyModeIgnore))

	// Create a new libpod container based on the spec.
	ctr, err := ic.Libpod.NewContainer(ctx, runtimeSpec, spec, false, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating service container: %w", err)
	}

	return ctr, nil
}

// Creates the name for a k8s entity based on the provided content of a
// K8s yaml file and a given suffix.
func k8sName(content []byte, suffix string) string {
	// The name of the service container is the first 12
	// characters of the yaml file's hash followed by the
	// '-service' suffix to guarantee a predictable and
	// discoverable name.
	hash := digest.FromBytes(content).Encoded()
	return hash[0:12] + "-" + suffix
}

func (ic *ContainerEngine) PlayKube(ctx context.Context, body io.Reader, options entities.PlayKubeOptions) (_ *entities.PlayKubeReport, finalErr error) {
	if options.ServiceContainer && options.Start == types.OptionalBoolFalse { // Sanity check to be future proof
		return nil, fmt.Errorf("running a service container requires starting the pod(s)")
	}

	report := &entities.PlayKubeReport{}
	validKinds := 0

	// when no network options are specified, create a common network for all the pods
	if len(options.Networks) == 0 {
		_, err := ic.NetworkCreate(
			ctx,
			nettypes.Network{
				Name:       kubeDefaultNetwork,
				DNSEnabled: true,
			},
			&nettypes.NetworkCreateOptions{
				IgnoreIfExists: true,
			},
		)
		if err != nil {
			return nil, err
		}
	}

	// read yaml document
	content, err := io.ReadAll(body)
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
		return nil, fmt.Errorf("unable to sort kube kinds: %w", err)
	}

	ipIndex := 0

	var configMaps []v1.ConfigMap

	ranContainers := false
	// FIXME: both, the service container and the proxies, should ideally
	// be _state_ of an object. The Kube code below is quite Spaghetti-code
	// which we should refactor at some point to make it easier to extend
	// (via shared state instead of passing data around) and make it more
	// maintainable long term.
	var serviceContainer *libpod.Container
	var notifyProxies []*notifyproxy.NotifyProxy
	defer func() {
		// Close the notify proxy on return.  At that point we know
		// that a) all containers have send their READY message and
		// that b) the service container has exited (and hence all
		// containers).
		for _, proxy := range notifyProxies {
			if err := proxy.Close(); err != nil {
				logrus.Errorf("Closing notify proxy %q: %v", proxy.SocketPath(), err)
			}
		}
	}()

	// create pod on each document if it is a pod or deployment
	// any other kube kind will be skipped
	for _, document := range documentList {
		kind, err := getKubeKind(document)
		if err != nil {
			return nil, fmt.Errorf("unable to read kube YAML: %w", err)
		}

		// TODO: create constants for the various "kinds" of yaml files.
		if options.ServiceContainer && serviceContainer == nil && (kind == "Pod" || kind == "Deployment") {
			ctr, err := ic.createServiceContainer(ctx, k8sName(content, "service"), options)
			if err != nil {
				return nil, err
			}
			serviceContainer = ctr
			// Make sure to remove the container in case something goes wrong below.
			defer func() {
				if finalErr == nil {
					return
				}
				if err := ic.Libpod.RemoveContainer(ctx, ctr, true, false, nil); err != nil {
					logrus.Errorf("Cleaning up service container after failure: %v", err)
				}
			}()
		}

		switch kind {
		case "Pod":
			var podYAML v1.Pod
			var podTemplateSpec v1.PodTemplateSpec

			if err := yaml.Unmarshal(document, &podYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube Pod: %w", err)
			}

			podTemplateSpec.ObjectMeta = podYAML.ObjectMeta
			podTemplateSpec.Spec = podYAML.Spec
			for name, val := range podYAML.Annotations {
				if len(val) > define.MaxKubeAnnotation {
					return nil, fmt.Errorf("invalid annotation %q=%q value length exceeds Kubernetetes max %d", name, val, define.MaxKubeAnnotation)
				}
			}
			for name, val := range options.Annotations {
				if podYAML.Annotations == nil {
					podYAML.Annotations = make(map[string]string)
				}
				podYAML.Annotations[name] = val
			}

			r, proxies, err := ic.playKubePod(ctx, podTemplateSpec.ObjectMeta.Name, &podTemplateSpec, options, &ipIndex, podYAML.Annotations, configMaps, serviceContainer)
			if err != nil {
				return nil, err
			}
			notifyProxies = append(notifyProxies, proxies...)

			report.Pods = append(report.Pods, r.Pods...)
			validKinds++
			ranContainers = true
		case "Deployment":
			var deploymentYAML v1apps.Deployment

			if err := yaml.Unmarshal(document, &deploymentYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube Deployment: %w", err)
			}

			r, proxies, err := ic.playKubeDeployment(ctx, &deploymentYAML, options, &ipIndex, configMaps, serviceContainer)
			if err != nil {
				return nil, err
			}
			notifyProxies = append(notifyProxies, proxies...)

			report.Pods = append(report.Pods, r.Pods...)
			validKinds++
			ranContainers = true
		case "PersistentVolumeClaim":
			var pvcYAML v1.PersistentVolumeClaim

			if err := yaml.Unmarshal(document, &pvcYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube PersistentVolumeClaim: %w", err)
			}

			for name, val := range options.Annotations {
				if pvcYAML.Annotations == nil {
					pvcYAML.Annotations = make(map[string]string)
				}
				pvcYAML.Annotations[name] = val
			}

			if options.IsRemote {
				if _, ok := pvcYAML.Annotations[util.VolumeImportSourceAnnotation]; ok {
					return nil, fmt.Errorf("importing volumes is not supported for remote requests")
				}
			}

			r, err := ic.playKubePVC(ctx, &pvcYAML)
			if err != nil {
				return nil, err
			}

			report.Volumes = append(report.Volumes, r.Volumes...)
			validKinds++
		case "ConfigMap":
			var configMap v1.ConfigMap

			if err := yaml.Unmarshal(document, &configMap); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube ConfigMap: %w", err)
			}
			configMaps = append(configMaps, configMap)
		case "Secret":
			var secret v1.Secret

			if err := yaml.Unmarshal(document, &secret); err != nil {
				return nil, fmt.Errorf("unable to read YAML as kube secret: %w", err)
			}

			r, err := ic.playKubeSecret(&secret)
			if err != nil {
				return nil, err
			}
			report.Secrets = append(report.Secrets, entities.PlaySecret{CreateReport: r})
			validKinds++
		default:
			logrus.Infof("Kube kind %s not supported", kind)
			continue
		}
	}

	if validKinds == 0 {
		if len(configMaps) > 0 {
			return nil, fmt.Errorf("ConfigMaps in podman are not a standalone object and must be used in a container")
		}
		return nil, fmt.Errorf("YAML document does not contain any supported kube kind")
	}

	if options.ServiceContainer && ranContainers {
		message := fmt.Sprintf("MAINPID=%d\n%s", os.Getpid(), daemon.SdNotifyReady)
		if err := notifyproxy.SendMessage("", message); err != nil {
			return nil, err
		}

		if _, err := serviceContainer.Wait(ctx); err != nil {
			return nil, fmt.Errorf("waiting for service container: %w", err)
		}
	}

	return report, nil
}

func (ic *ContainerEngine) playKubeDeployment(ctx context.Context, deploymentYAML *v1apps.Deployment, options entities.PlayKubeOptions, ipIndex *int, configMaps []v1.ConfigMap, serviceContainer *libpod.Container) (*entities.PlayKubeReport, []*notifyproxy.NotifyProxy, error) {
	var (
		deploymentName string
		podSpec        v1.PodTemplateSpec
		numReplicas    int32
		i              int32
		report         entities.PlayKubeReport
	)

	deploymentName = deploymentYAML.ObjectMeta.Name
	if deploymentName == "" {
		return nil, nil, errors.New("deployment does not have a name")
	}
	numReplicas = 1
	if deploymentYAML.Spec.Replicas != nil {
		numReplicas = *deploymentYAML.Spec.Replicas
	}
	podSpec = deploymentYAML.Spec.Template

	// create "replicas" number of pods
	var notifyProxies []*notifyproxy.NotifyProxy
	for i = 0; i < numReplicas; i++ {
		podName := fmt.Sprintf("%s-pod-%d", deploymentName, i)
		podReport, proxies, err := ic.playKubePod(ctx, podName, &podSpec, options, ipIndex, deploymentYAML.Annotations, configMaps, serviceContainer)
		if err != nil {
			return nil, notifyProxies, fmt.Errorf("encountered while bringing up pod %s: %w", podName, err)
		}
		report.Pods = append(report.Pods, podReport.Pods...)
		notifyProxies = append(notifyProxies, proxies...)
	}
	return &report, notifyProxies, nil
}

func (ic *ContainerEngine) playKubePod(ctx context.Context, podName string, podYAML *v1.PodTemplateSpec, options entities.PlayKubeOptions, ipIndex *int, annotations map[string]string, configMaps []v1.ConfigMap, serviceContainer *libpod.Container) (*entities.PlayKubeReport, []*notifyproxy.NotifyProxy, error) {
	var (
		writer      io.Writer
		playKubePod entities.PlayKubePod
		report      entities.PlayKubeReport
	)

	mainSdNotifyMode, err := getSdNotifyMode(annotations, "")
	if err != nil {
		return nil, nil, err
	}

	// Create the secret manager before hand
	secretsManager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, nil, err
	}

	// Assert the pod has a name
	if podName == "" {
		return nil, nil, fmt.Errorf("pod does not have a name")
	}

	podOpt := entities.PodCreateOptions{
		Infra:      true,
		Net:        &entities.NetOptions{NoHosts: options.NoHosts},
		ExitPolicy: string(config.PodExitPolicyStop),
	}
	podOpt, err = kube.ToPodOpt(ctx, podName, podOpt, podYAML)
	if err != nil {
		return nil, nil, err
	}

	// add kube default network if no network is explicitly added
	if podOpt.Net.Network.NSMode != "host" && len(options.Networks) == 0 {
		options.Networks = []string{kubeDefaultNetwork}
	}

	if len(options.Networks) > 0 {
		var pastaNetworkNameExists bool

		_, err := ic.Libpod.Network().NetworkInspect("pasta")
		if err == nil {
			pastaNetworkNameExists = true
		}

		ns, networks, netOpts, err := specgen.ParseNetworkFlag(options.Networks, pastaNetworkNameExists)
		if err != nil {
			return nil, nil, err
		}

		podOpt.Net.Network = ns
		podOpt.Net.Networks = networks
		podOpt.Net.NetworkOptions = netOpts
	}

	if options.Userns == "" {
		options.Userns = "host"
		if podYAML.Spec.HostUsers != nil && !*podYAML.Spec.HostUsers {
			options.Userns = "auto"
		}
	} else if podYAML.Spec.HostUsers != nil {
		logrus.Info("overriding the user namespace mode in the pod spec")
	}

	// Validate the userns modes supported.
	podOpt.Userns, err = specgen.ParseUserNamespace(options.Userns)
	if err != nil {
		return nil, nil, err
	}

	// FIXME This is very hard to support properly with a good ux
	if len(options.StaticIPs) > *ipIndex {
		if !podOpt.Net.Network.IsBridge() {
			return nil, nil, fmt.Errorf("static ip addresses can only be set when the network mode is bridge: %w", define.ErrInvalidArg)
		}
		if len(podOpt.Net.Networks) != 1 {
			return nil, nil, fmt.Errorf("cannot set static ip addresses for more than network, use netname:ip=<ip> syntax to specify ips for more than network: %w", define.ErrInvalidArg)
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
			return nil, nil, fmt.Errorf("static mac address can only be set when the network mode is bridge: %w", define.ErrInvalidArg)
		}
		if len(podOpt.Net.Networks) != 1 {
			return nil, nil, fmt.Errorf("cannot set static mac address for more than network, use netname:mac=<mac> syntax to specify mac for more than network: %w", define.ErrInvalidArg)
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

	if len(options.PublishPorts) > 0 {
		publishPorts, err := specgenutil.CreatePortBindings(options.PublishPorts)
		if err != nil {
			return nil, nil, err
		}
		mergePublishPorts(&podOpt, publishPorts)
	}

	p := specgen.NewPodSpecGenerator()
	if err != nil {
		return nil, nil, err
	}

	p, err = entities.ToPodSpecGen(*p, &podOpt)
	if err != nil {
		return nil, nil, err
	}
	podSpec := entities.PodSpec{PodSpecGen: *p}

	configMapIndex := make(map[string]struct{})
	for _, configMap := range configMaps {
		configMapIndex[configMap.Name] = struct{}{}
	}
	for _, p := range options.ConfigMaps {
		f, err := os.Open(p)
		if err != nil {
			return nil, nil, err
		}
		defer f.Close()

		cm, err := readConfigMapFromFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("%q: %w", p, err)
		}

		if _, present := configMapIndex[cm.Name]; present {
			return nil, nil, fmt.Errorf("ambiguous configuration: the same config map %s is present in YAML and in --configmaps %s file", cm.Name, p)
		}

		configMaps = append(configMaps, cm)
	}

	volumes, err := kube.InitializeVolumes(podYAML.Spec.Volumes, configMaps, secretsManager)
	if err != nil {
		return nil, nil, err
	}

	// Go through the volumes and create a podman volume for all volumes that have been
	// defined by a configmap or secret
	for _, v := range volumes {
		if (v.Type == kube.KubeVolumeTypeConfigMap || v.Type == kube.KubeVolumeTypeSecret) && !v.Optional {
			vol, err := ic.Libpod.NewVolume(ctx, libpod.WithVolumeName(v.Source))
			if err != nil {
				if errors.Is(err, define.ErrVolumeExists) {
					// Volume for this configmap already exists do not
					// error out instead reuse the current volume.
					vol, err = ic.Libpod.GetVolume(v.Source)
					if err != nil {
						return nil, nil, fmt.Errorf("cannot re-use local volume for volume from configmap %q: %w", v.Source, err)
					}
				} else {
					return nil, nil, fmt.Errorf("cannot create a local volume for volume from configmap %q: %w", v.Source, err)
				}
			}
			mountPoint, err := vol.MountPoint()
			if err != nil || mountPoint == "" {
				return nil, nil, fmt.Errorf("unable to get mountpoint of volume %q: %w", vol.Name(), err)
			}
			// Create files and add data to the volume mountpoint based on the Items in the volume
			for k, v := range v.Items {
				dataPath := filepath.Join(mountPoint, k)
				f, err := os.Create(dataPath)
				if err != nil {
					return nil, nil, fmt.Errorf("cannot create file %q at volume mountpoint %q: %w", k, mountPoint, err)
				}
				defer f.Close()
				_, err = f.Write(v)
				if err != nil {
					return nil, nil, err
				}
			}
		}
	}

	seccompPaths, err := kube.InitializeSeccompPaths(podYAML.ObjectMeta.Annotations, options.SeccompProfileRoot)
	if err != nil {
		return nil, nil, err
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
		infraOptions.ReadOnly = true
		infraOptions.ReadWriteTmpFS = false
		infraOptions.UserNS = options.Userns
		podSpec.PodSpecGen.InfraImage = infraImage
		podSpec.PodSpecGen.NoInfra = false
		podSpec.PodSpecGen.InfraContainerSpec = specgen.NewSpecGenerator(infraImage, false)
		podSpec.PodSpecGen.InfraContainerSpec.NetworkOptions = p.NetworkOptions
		podSpec.PodSpecGen.InfraContainerSpec.SdNotifyMode = define.SdNotifyModeIgnore

		err = specgenutil.FillOutSpecGen(podSpec.PodSpecGen.InfraContainerSpec, &infraOptions, []string{})
		if err != nil {
			return nil, nil, err
		}
	}

	if serviceContainer != nil {
		podSpec.PodSpecGen.ServiceContainerID = serviceContainer.ID()
	}

	// Create the Pod
	pod, err := generate.MakePod(&podSpec, ic.Libpod)
	if err != nil {
		return nil, nil, err
	}

	podInfraID, err := pod.InfraContainerID()
	if err != nil {
		return nil, nil, err
	}

	if !options.Quiet {
		writer = os.Stderr
	}

	containers := make([]*libpod.Container, 0, len(podYAML.Spec.Containers))
	initContainers := make([]*libpod.Container, 0, len(podYAML.Spec.InitContainers))

	var cwd string
	if options.ContextDir != "" {
		cwd = options.ContextDir
	} else {
		cwd, err = os.Getwd()
		if err != nil {
			return nil, nil, err
		}
	}

	cfg, err := ic.Libpod.GetConfigNoCopy()
	if err != nil {
		return nil, nil, err
	}

	var readOnly types.OptionalBool
	if cfg.Containers.ReadOnly {
		readOnly = types.NewOptionalBool(cfg.Containers.ReadOnly)
	}

	ctrNames := make(map[string]string)
	for _, initCtr := range podYAML.Spec.InitContainers {
		// Error out if same name is used for more than one container
		if _, ok := ctrNames[initCtr.Name]; ok {
			return nil, nil, fmt.Errorf("the pod %q is invalid; duplicate container name %q detected", podName, initCtr.Name)
		}
		ctrNames[initCtr.Name] = ""
		// Init containers cannot have either of lifecycle, livenessProbe, readinessProbe, or startupProbe set
		if initCtr.Lifecycle != nil || initCtr.LivenessProbe != nil || initCtr.ReadinessProbe != nil || initCtr.StartupProbe != nil {
			return nil, nil, fmt.Errorf("cannot create an init container that has either of lifecycle, livenessProbe, readinessProbe, or startupProbe set")
		}
		pulledImage, labels, err := ic.getImageAndLabelInfo(ctx, cwd, annotations, writer, initCtr, options)
		if err != nil {
			return nil, nil, err
		}

		for k, v := range podSpec.PodSpecGen.Labels { // add podYAML labels
			labels[k] = v
		}
		initCtrType := annotations[define.InitContainerType]
		if initCtrType == "" {
			initCtrType = define.OneShotInitContainer
		}

		specgenOpts := kube.CtrSpecGenOptions{
			Annotations:        annotations,
			ConfigMaps:         configMaps,
			Container:          initCtr,
			Image:              pulledImage,
			InitContainerType:  initCtrType,
			Labels:             labels,
			LogDriver:          options.LogDriver,
			LogOptions:         options.LogOptions,
			NetNSIsHost:        p.NetNS.IsHost(),
			PodID:              pod.ID(),
			PodInfraID:         podInfraID,
			PodName:            podName,
			PodSecurityContext: podYAML.Spec.SecurityContext,
			ReadOnly:           readOnly,
			RestartPolicy:      define.RestartPolicyNo,
			SeccompPaths:       seccompPaths,
			SecretsManager:     secretsManager,
			UserNSIsHost:       p.Userns.IsHost(),
			Volumes:            volumes,
		}
		specGen, err := kube.ToSpecGen(ctx, &specgenOpts)
		if err != nil {
			return nil, nil, err
		}
		specGen.SdNotifyMode = define.SdNotifyModeIgnore
		rtSpec, spec, opts, err := generate.MakeContainer(ctx, ic.Libpod, specGen, false, nil)
		if err != nil {
			return nil, nil, err
		}
		opts = append(opts, libpod.WithSdNotifyMode(define.SdNotifyModeIgnore))
		ctr, err := generate.ExecuteCreate(ctx, ic.Libpod, rtSpec, spec, false, opts...)
		if err != nil {
			return nil, nil, err
		}

		initContainers = append(initContainers, ctr)
	}

	var sdNotifyProxies []*notifyproxy.NotifyProxy // containers' sd-notify proxies

	for _, container := range podYAML.Spec.Containers {
		// Error out if the same name is used for more than one container
		if _, ok := ctrNames[container.Name]; ok {
			return nil, nil, fmt.Errorf("the pod %q is invalid; duplicate container name %q detected", podName, container.Name)
		}
		ctrNames[container.Name] = ""
		pulledImage, labels, err := ic.getImageAndLabelInfo(ctx, cwd, annotations, writer, container, options)
		if err != nil {
			return nil, nil, err
		}

		for k, v := range podSpec.PodSpecGen.Labels { // add podYAML labels
			labels[k] = v
		}

		specgenOpts := kube.CtrSpecGenOptions{
			Annotations:        annotations,
			ConfigMaps:         configMaps,
			Container:          container,
			Image:              pulledImage,
			Labels:             labels,
			LogDriver:          options.LogDriver,
			LogOptions:         options.LogOptions,
			NetNSIsHost:        p.NetNS.IsHost(),
			PodID:              pod.ID(),
			PodInfraID:         podInfraID,
			PodName:            podName,
			PodSecurityContext: podYAML.Spec.SecurityContext,
			ReadOnly:           readOnly,
			RestartPolicy:      ctrRestartPolicy,
			SeccompPaths:       seccompPaths,
			SecretsManager:     secretsManager,
			UserNSIsHost:       p.Userns.IsHost(),
			Volumes:            volumes,
		}

		specGen, err := kube.ToSpecGen(ctx, &specgenOpts)
		if err != nil {
			return nil, nil, err
		}
		specGen.RawImageName = container.Image
		rtSpec, spec, opts, err := generate.MakeContainer(ctx, ic.Libpod, specGen, false, nil)
		if err != nil {
			return nil, nil, err
		}

		sdNotifyMode := mainSdNotifyMode
		ctrNotifyMode, err := getSdNotifyMode(annotations, container.Name)
		if err != nil {
			return nil, nil, err
		}
		if ctrNotifyMode != "" {
			sdNotifyMode = ctrNotifyMode
		}
		if sdNotifyMode == "" { // Default to "ignore"
			sdNotifyMode = define.SdNotifyModeIgnore
		}

		opts = append(opts, libpod.WithSdNotifyMode(sdNotifyMode))

		var proxy *notifyproxy.NotifyProxy
		// Create a notify proxy for the container.
		if sdNotifyMode != "" && sdNotifyMode != define.SdNotifyModeIgnore {
			proxy, err = notifyproxy.New("")
			if err != nil {
				return nil, nil, err
			}
			sdNotifyProxies = append(sdNotifyProxies, proxy)
			opts = append(opts, libpod.WithSdNotifySocket(proxy.SocketPath()))
		}

		ctr, err := generate.ExecuteCreate(ctx, ic.Libpod, rtSpec, spec, false, opts...)
		if err != nil {
			return nil, nil, err
		}
		if proxy != nil {
			proxy.AddContainer(ctr)
		}
		containers = append(containers, ctr)
	}

	if options.Start != types.OptionalBoolFalse {
		// Start the containers
		podStartErrors, err := pod.Start(ctx)
		if err != nil && !errors.Is(err, define.ErrPodPartialFail) {
			return nil, nil, err
		}
		for id, err := range podStartErrors {
			playKubePod.ContainerErrors = append(playKubePod.ContainerErrors, fmt.Errorf("starting container %s: %w", id, err).Error())
			fmt.Println(playKubePod.ContainerErrors)
		}

		// Wait for each proxy to receive a READY message. Use a wait
		// group to prevent the potential for ABBA kinds of deadlocks.
		var wg sync.WaitGroup
		errors := make([]error, len(sdNotifyProxies))
		for i := range sdNotifyProxies {
			wg.Add(1)
			defer func() {
				if err := sdNotifyProxies[i].Close(); err != nil {
					logrus.Errorf("Closing sdnotify proxy %q: %v", sdNotifyProxies[i].SocketPath(), err)
				}
			}()
			go func(i int) {
				err := sdNotifyProxies[i].Wait()
				if err != nil {
					err = fmt.Errorf("waiting for sd-notify proxy: %w", err)
				}
				errors[i] = err
				wg.Done()
			}(i)
		}
		wg.Wait()
		for _, err := range errors {
			if err != nil {
				// Close all proxies on error.
				for _, proxy := range sdNotifyProxies {
					_ = proxy.Close()
				}
				return nil, nil, err
			}
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

	return &report, sdNotifyProxies, nil
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
	if (len(buildFile) > 0) && ((!existsLocally && options.Build != types.OptionalBoolFalse) || (options.Build == types.OptionalBoolTrue)) {
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
	setLabel := func(label string) {
		var result string
		ctrSpecific := fmt.Sprintf("%s/%s", label, container.Name)
		for k, v := range annotations {
			switch k {
			case label:
				result = v
			case ctrSpecific:
				labels[label] = v
				return
			}
		}
		if result != "" {
			labels[label] = result
		}
	}

	setLabel(define.AutoUpdateLabel)
	setLabel(define.AutoUpdateAuthfileLabel)

	return pulledImage, labels, nil
}

// playKubePVC creates a podman volume from a kube persistent volume claim.
func (ic *ContainerEngine) playKubePVC(ctx context.Context, pvcYAML *v1.PersistentVolumeClaim) (*entities.PlayKubeReport, error) {
	var report entities.PlayKubeReport
	opts := make(map[string]string)

	// Get pvc name.
	// This is the only required pvc attribute to create a podman volume.
	name := pvcYAML.Name
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("persistent volume claim name can not be empty")
	}

	// Create podman volume options.
	volOptions := []libpod.VolumeCreateOption{
		libpod.WithVolumeName(name),
		libpod.WithVolumeLabels(pvcYAML.Labels),
		libpod.WithVolumeIgnoreIfExist(),
	}

	// Get pvc annotations and create remaining podman volume options if available.
	// These are podman volume options that do not match any of the persistent volume claim
	// attributes, so they can be configured using annotations since they will not affect k8s.
	var importFrom string
	for k, v := range pvcYAML.Annotations {
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
				return nil, fmt.Errorf("cannot convert uid %s to integer: %w", v, err)
			}
			volOptions = append(volOptions, libpod.WithVolumeUID(uid))
			opts["UID"] = v
		case util.VolumeGIDAnnotation:
			gid, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("cannot convert gid %s to integer: %w", v, err)
			}
			volOptions = append(volOptions, libpod.WithVolumeGID(gid))
			opts["GID"] = v
		case util.VolumeMountOptsAnnotation:
			opts["o"] = v
		case util.VolumeImportSourceAnnotation:
			importFrom = v
		}
	}
	volOptions = append(volOptions, libpod.WithVolumeOptions(opts))

	// Validate the file and open it before creating the volume for fast-fail
	var tarFile *os.File
	if len(importFrom) > 0 {
		err := parse.ValidateFileName(importFrom)
		if err != nil {
			return nil, err
		}

		// open tar file
		tarFile, err = os.Open(importFrom)
		if err != nil {
			return nil, err
		}
		defer tarFile.Close()
	}

	// Create volume.
	vol, err := ic.Libpod.NewVolume(ctx, volOptions...)
	if err != nil {
		return nil, err
	}

	if tarFile != nil {
		err = ic.importVolume(ctx, vol, tarFile)
		if err != nil {
			// Remove the volume to avoid partial success
			if rmErr := ic.Libpod.RemoveVolume(ctx, vol, true, nil); rmErr != nil {
				logrus.Debug(rmErr)
			}
			return nil, err
		}
	}

	report.Volumes = append(report.Volumes, entities.PlayKubeVolume{
		Name: vol.Name(),
	})

	return &report, nil
}

func mergePublishPorts(p *entities.PodCreateOptions, publishPortsOption []nettypes.PortMapping) {
	for _, publishPortSpec := range p.Net.PublishPorts {
		if !portAlreadyPublished(publishPortSpec, publishPortsOption) {
			publishPortsOption = append(publishPortsOption, publishPortSpec)
		}
	}
	p.Net.PublishPorts = publishPortsOption
}

func portAlreadyPublished(port nettypes.PortMapping, publishedPorts []nettypes.PortMapping) bool {
	for _, publishedPort := range publishedPorts {
		if port.ContainerPort >= publishedPort.ContainerPort &&
			port.ContainerPort < publishedPort.ContainerPort+publishedPort.Range &&
			isSamePortProtocol(port.Protocol, publishedPort.Protocol) {
			return true
		}
	}
	return false
}

func isSamePortProtocol(a, b string) bool {
	if len(a) == 0 {
		a = string(v1.ProtocolTCP)
	}
	if len(b) == 0 {
		b = string(v1.ProtocolTCP)
	}

	ret := strings.EqualFold(a, b)
	return ret
}

func (ic *ContainerEngine) importVolume(ctx context.Context, vol *libpod.Volume, tarFile *os.File) error {
	volumeConfig, err := vol.Config()
	if err != nil {
		return err
	}

	mountPoint := volumeConfig.MountPoint
	if len(mountPoint) == 0 {
		return errors.New("volume is not mounted anywhere on host")
	}

	driver := volumeConfig.Driver
	volumeOptions := volumeConfig.Options
	volumeMountStatus, err := ic.VolumeMounted(ctx, vol.Name())
	if err != nil {
		return err
	}

	// Check if volume needs a mount and export only if volume is mounted
	if vol.NeedsMount() && !volumeMountStatus.Value {
		return fmt.Errorf("volume needs to be mounted but is not mounted on %s", mountPoint)
	}

	// Check if volume is using `local` driver and has mount options type other than tmpfs
	if len(driver) == 0 || driver == define.VolumeDriverLocal {
		if mountOptionType, ok := volumeOptions["type"]; ok {
			if mountOptionType != "tmpfs" && !volumeMountStatus.Value {
				return fmt.Errorf("volume is using a driver %s and volume is not mounted on %s", driver, mountPoint)
			}
		}
	}

	// dont care if volume is mounted or not we are gonna import everything to mountPoint
	return utils.UntarToFileSystem(mountPoint, tarFile, nil)
}

// readConfigMapFromFile returns a kubernetes configMap obtained from --configmap flag
func readConfigMapFromFile(r io.Reader) (v1.ConfigMap, error) {
	var cm v1.ConfigMap

	content, err := io.ReadAll(r)
	if err != nil {
		return cm, fmt.Errorf("unable to read ConfigMap YAML content: %w", err)
	}

	if err := yaml.Unmarshal(content, &cm); err != nil {
		return cm, fmt.Errorf("unable to read YAML as Kube ConfigMap: %w", err)
	}

	if cm.Kind != "ConfigMap" {
		return cm, fmt.Errorf("invalid YAML kind: %q. [ConfigMap] is the only supported by --configmap", cm.Kind)
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
			return nil, fmt.Errorf("multi doc yaml could not be split: %w", err)
		}

		if o != nil {
			// back to bytes
			document, err := yamlv3.Marshal(o)
			if err != nil {
				return nil, fmt.Errorf("individual doc yaml could not be marshalled: %w", err)
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

	_, err = os.Stat(dockerfilePath)
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

func (ic *ContainerEngine) PlayKubeDown(ctx context.Context, body io.Reader, options entities.PlayKubeDownOptions) (*entities.PlayKubeReport, error) {
	var (
		podNames    []string
		volumeNames []string
	)
	reports := new(entities.PlayKubeReport)

	// read yaml document
	content, err := io.ReadAll(body)
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
		return nil, fmt.Errorf("unable to sort kube kinds: %w", err)
	}

	for _, document := range documentList {
		kind, err := getKubeKind(document)
		if err != nil {
			return nil, fmt.Errorf("unable to read as kube YAML: %w", err)
		}

		switch kind {
		case "Pod":
			var podYAML v1.Pod
			if err := yaml.Unmarshal(document, &podYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube Pod: %w", err)
			}
			podNames = append(podNames, podYAML.ObjectMeta.Name)
		case "Deployment":
			var deploymentYAML v1apps.Deployment

			if err := yaml.Unmarshal(document, &deploymentYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube Deployment: %w", err)
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
		case "PersistentVolumeClaim":
			var pvcYAML v1.PersistentVolumeClaim
			if err := yaml.Unmarshal(document, &pvcYAML); err != nil {
				return nil, fmt.Errorf("unable to read YAML as Kube PersistentVolumeClaim: %w", err)
			}
			volumeNames = append(volumeNames, pvcYAML.Name)
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

	if options.Force {
		reports.VolumeRmReport, err = ic.VolumeRm(ctx, volumeNames, entities.VolumeRmOptions{})
		if err != nil {
			return nil, err
		}
	}

	return reports, nil
}

// playKubeSecret allows users to create and store a kubernetes secret as a podman secret
func (ic *ContainerEngine) playKubeSecret(secret *v1.Secret) (*entities.SecretCreateReport, error) {
	r := &entities.SecretCreateReport{}

	// Create the secret manager before hand
	secretsManager, err := ic.Libpod.SecretsManager()
	if err != nil {
		return nil, err
	}

	data, err := yaml.Marshal(secret)
	if err != nil {
		return nil, err
	}

	secretsPath := ic.Libpod.GetSecretsStorageDir()
	opts := make(map[string]string)
	opts["path"] = filepath.Join(secretsPath, "filedriver")
	// maybe k8sName(data)...
	// using this does not allow the user to use the name given to the secret
	// but keeping secret.Name as the ID can lead to a collision.

	s, err := secretsManager.Lookup(secret.Name)
	if err == nil {
		if val, ok := s.Metadata["immutable"]; ok {
			if val == "true" {
				return nil, fmt.Errorf("cannot remove colliding secret as it is set to immutable")
			}
		}
		_, err = secretsManager.Delete(s.Name)
		if err != nil {
			return nil, err
		}
	}

	// now we have either removed the old secret w/ the same name or
	// the name was not taken. Either way, we can now store.

	meta := make(map[string]string)
	if secret.Immutable != nil && *secret.Immutable {
		meta["immutable"] = "true"
	}

	storeOpts := secrets.StoreOptions{
		DriverOpts: opts,
		Metadata:   meta,
	}

	secretID, err := secretsManager.Store(secret.Name, data, "file", storeOpts)
	if err != nil {
		return nil, err
	}

	r.ID = secretID

	return r, nil
}
