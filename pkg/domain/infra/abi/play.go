package abi

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	ann "github.com/containers/libpod/pkg/annotations"
	"github.com/containers/libpod/pkg/domain/entities"
	envLib "github.com/containers/libpod/pkg/env"
	ns "github.com/containers/libpod/pkg/namespaces"
	createconfig "github.com/containers/libpod/pkg/spec"
	"github.com/containers/libpod/pkg/specgen/generate"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/distribution/reference"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

const (
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	kubeDirectoryPermission = 0755
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	kubeFilePermission = 0644
)

func (ic *ContainerEngine) PlayKube(ctx context.Context, path string, options entities.PlayKubeOptions) (*entities.PlayKubeReport, error) {
	var (
		containers    []*libpod.Container
		pod           *libpod.Pod
		podOptions    []libpod.PodCreateOption
		podYAML       v1.Pod
		registryCreds *types.DockerAuthConfig
		writer        io.Writer
		report        entities.PlayKubeReport
	)

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(content, &podYAML); err != nil {
		return nil, errors.Wrapf(err, "unable to read %q as YAML", path)
	}

	// NOTE: pkg/bindings/play is also parsing the file.
	// A pkg/kube would be nice to refactor and abstract
	// parts of the K8s-related code.
	if podYAML.Kind != "Pod" {
		return nil, errors.Errorf("invalid YAML kind: %q. Pod is the only supported Kubernetes YAML kind", podYAML.Kind)
	}

	// check for name collision between pod and container
	podName := podYAML.ObjectMeta.Name
	if podName == "" {
		return nil, errors.Errorf("pod does not have a name")
	}
	for _, n := range podYAML.Spec.Containers {
		if n.Name == podName {
			report.Logs = append(report.Logs,
				fmt.Sprintf("a container exists with the same name (%q) as the pod in your YAML file; changing pod name to %s_pod\n", podName, podName))
			podName = fmt.Sprintf("%s_pod", podName)
		}
	}

	podOptions = append(podOptions, libpod.WithInfraContainer())
	podOptions = append(podOptions, libpod.WithPodName(podName))
	// TODO for now we just used the default kernel namespaces; we need to add/subtract this from yaml

	hostname := podYAML.Spec.Hostname
	if hostname == "" {
		hostname = podName
	}
	podOptions = append(podOptions, libpod.WithPodHostname(hostname))

	if podYAML.Spec.HostNetwork {
		podOptions = append(podOptions, libpod.WithPodHostNetwork())
	}

	nsOptions, err := generate.GetNamespaceOptions(strings.Split(createconfig.DefaultKernelNamespaces, ","))
	if err != nil {
		return nil, err
	}
	podOptions = append(podOptions, nsOptions...)
	podPorts := getPodPorts(podYAML.Spec.Containers)
	podOptions = append(podOptions, libpod.WithInfraContainerPorts(podPorts))

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
			podOptions = append(podOptions, libpod.WithPodNetworks(networks))
		}
	}

	// Create the Pod
	pod, err = ic.Libpod.NewPod(ctx, podOptions...)
	if err != nil {
		return nil, err
	}

	podInfraID, err := pod.InfraContainerID()
	if err != nil {
		return nil, err
	}
	hasUserns := false
	if podInfraID != "" {
		podCtr, err := ic.Libpod.GetContainer(podInfraID)
		if err != nil {
			return nil, err
		}
		mappings, err := podCtr.IDMappings()
		if err != nil {
			return nil, err
		}
		hasUserns = len(mappings.UIDMap) > 0
	}

	namespaces := map[string]string{
		// Disabled during code review per mheon
		//"pid":  fmt.Sprintf("container:%s", podInfraID),
		"net": fmt.Sprintf("container:%s", podInfraID),
		"ipc": fmt.Sprintf("container:%s", podInfraID),
		"uts": fmt.Sprintf("container:%s", podInfraID),
	}
	if hasUserns {
		namespaces["user"] = fmt.Sprintf("container:%s", podInfraID)
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

	// map from name to mount point
	volumes := make(map[string]string)
	for _, volume := range podYAML.Spec.Volumes {
		hostPath := volume.VolumeSource.HostPath
		if hostPath == nil {
			return nil, errors.Errorf("HostPath is currently the only supported VolumeSource")
		}
		if hostPath.Type != nil {
			switch *hostPath.Type {
			case v1.HostPathDirectoryOrCreate:
				if _, err := os.Stat(hostPath.Path); os.IsNotExist(err) {
					if err := os.Mkdir(hostPath.Path, kubeDirectoryPermission); err != nil {
						return nil, errors.Errorf("Error creating HostPath %s at %s", volume.Name, hostPath.Path)
					}
				}
				// Label a newly created volume
				if err := libpod.LabelVolumePath(hostPath.Path); err != nil {
					return nil, errors.Wrapf(err, "Error giving %s a label", hostPath.Path)
				}
			case v1.HostPathFileOrCreate:
				if _, err := os.Stat(hostPath.Path); os.IsNotExist(err) {
					f, err := os.OpenFile(hostPath.Path, os.O_RDONLY|os.O_CREATE, kubeFilePermission)
					if err != nil {
						return nil, errors.Errorf("Error creating HostPath %s at %s", volume.Name, hostPath.Path)
					}
					if err := f.Close(); err != nil {
						logrus.Warnf("Error in closing newly created HostPath file: %v", err)
					}
				}
				// unconditionally label a newly created volume
				if err := libpod.LabelVolumePath(hostPath.Path); err != nil {
					return nil, errors.Wrapf(err, "Error giving %s a label", hostPath.Path)
				}
			case v1.HostPathDirectory:
			case v1.HostPathFile:
			case v1.HostPathUnset:
				// do nothing here because we will verify the path exists in validateVolumeHostDir
				break
			default:
				return nil, errors.Errorf("Directories are the only supported HostPath type")
			}
		}

		if err := parse.ValidateVolumeHostDir(hostPath.Path); err != nil {
			return nil, errors.Wrapf(err, "Error in parsing HostPath in YAML")
		}
		volumes[volume.Name] = hostPath.Path
	}

	seccompPaths, err := initializeSeccompPaths(podYAML.ObjectMeta.Annotations, options.SeccompProfileRoot)
	if err != nil {
		return nil, err
	}

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
			return nil, err
		}
		// In kube, if the image is tagged with latest, it should always pull
		if tagged, isTagged := named.(reference.NamedTagged); isTagged {
			if tagged.Tag() == image.LatestTag {
				pullPolicy = util.PullImageAlways
			}
		}
		newImage, err := ic.Libpod.ImageRuntime().New(ctx, container.Image, options.SignaturePolicy, options.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, nil, pullPolicy)
		if err != nil {
			return nil, err
		}
		conf, err := kubeContainerToCreateConfig(ctx, container, ic.Libpod, newImage, namespaces, volumes, pod.ID(), podInfraID, seccompPaths)
		if err != nil {
			return nil, err
		}
		ctr, err := createconfig.CreateContainerFromCreateConfig(ic.Libpod, conf, ctx, pod)
		if err != nil {
			return nil, err
		}
		containers = append(containers, ctr)
	}

	// start the containers
	for _, ctr := range containers {
		if err := ctr.Start(ctx, true); err != nil {
			// Making this a hard failure here to avoid a mess
			// the other containers are in created status
			return nil, err
		}
	}

	report.Pod = pod.ID()
	for _, ctr := range containers {
		report.Containers = append(report.Containers, ctr.ID())
	}

	return &report, nil
}

// getPodPorts converts a slice of kube container descriptions to an
// array of ocicni portmapping descriptions usable in libpod
func getPodPorts(containers []v1.Container) []ocicni.PortMapping {
	var infraPorts []ocicni.PortMapping
	for _, container := range containers {
		for _, p := range container.Ports {
			if p.HostPort != 0 && p.ContainerPort == 0 {
				p.ContainerPort = p.HostPort
			}
			if p.Protocol == "" {
				p.Protocol = "tcp"
			}
			portBinding := ocicni.PortMapping{
				HostPort:      p.HostPort,
				ContainerPort: p.ContainerPort,
				Protocol:      strings.ToLower(string(p.Protocol)),
			}
			if p.HostIP != "" {
				logrus.Debug("HostIP on port bindings is not supported")
			}
			// only hostPort is utilized in podman context, all container ports
			// are accessible inside the shared network namespace
			if p.HostPort != 0 {
				infraPorts = append(infraPorts, portBinding)
			}

		}
	}
	return infraPorts
}

func setupSecurityContext(securityConfig *createconfig.SecurityConfig, userConfig *createconfig.UserConfig, containerYAML v1.Container) {
	if containerYAML.SecurityContext == nil {
		return
	}
	if containerYAML.SecurityContext.ReadOnlyRootFilesystem != nil {
		securityConfig.ReadOnlyRootfs = *containerYAML.SecurityContext.ReadOnlyRootFilesystem
	}
	if containerYAML.SecurityContext.Privileged != nil {
		securityConfig.Privileged = *containerYAML.SecurityContext.Privileged
	}

	if containerYAML.SecurityContext.AllowPrivilegeEscalation != nil {
		securityConfig.NoNewPrivs = !*containerYAML.SecurityContext.AllowPrivilegeEscalation
	}

	if seopt := containerYAML.SecurityContext.SELinuxOptions; seopt != nil {
		if seopt.User != "" {
			securityConfig.SecurityOpts = append(securityConfig.SecurityOpts, fmt.Sprintf("label=user:%s", seopt.User))
			securityConfig.LabelOpts = append(securityConfig.LabelOpts, fmt.Sprintf("user:%s", seopt.User))
		}
		if seopt.Role != "" {
			securityConfig.SecurityOpts = append(securityConfig.SecurityOpts, fmt.Sprintf("label=role:%s", seopt.Role))
			securityConfig.LabelOpts = append(securityConfig.LabelOpts, fmt.Sprintf("role:%s", seopt.Role))
		}
		if seopt.Type != "" {
			securityConfig.SecurityOpts = append(securityConfig.SecurityOpts, fmt.Sprintf("label=type:%s", seopt.Type))
			securityConfig.LabelOpts = append(securityConfig.LabelOpts, fmt.Sprintf("type:%s", seopt.Type))
		}
		if seopt.Level != "" {
			securityConfig.SecurityOpts = append(securityConfig.SecurityOpts, fmt.Sprintf("label=level:%s", seopt.Level))
			securityConfig.LabelOpts = append(securityConfig.LabelOpts, fmt.Sprintf("level:%s", seopt.Level))
		}
	}
	if caps := containerYAML.SecurityContext.Capabilities; caps != nil {
		for _, capability := range caps.Add {
			securityConfig.CapAdd = append(securityConfig.CapAdd, string(capability))
		}
		for _, capability := range caps.Drop {
			securityConfig.CapDrop = append(securityConfig.CapDrop, string(capability))
		}
	}
	if containerYAML.SecurityContext.RunAsUser != nil {
		userConfig.User = fmt.Sprintf("%d", *containerYAML.SecurityContext.RunAsUser)
	}
	if containerYAML.SecurityContext.RunAsGroup != nil {
		if userConfig.User == "" {
			userConfig.User = "0"
		}
		userConfig.User = fmt.Sprintf("%s:%d", userConfig.User, *containerYAML.SecurityContext.RunAsGroup)
	}
}

// kubeContainerToCreateConfig takes a v1.Container and returns a createconfig describing a container
func kubeContainerToCreateConfig(ctx context.Context, containerYAML v1.Container, runtime *libpod.Runtime, newImage *image.Image, namespaces map[string]string, volumes map[string]string, podID, infraID string, seccompPaths *kubeSeccompPaths) (*createconfig.CreateConfig, error) {
	var (
		containerConfig createconfig.CreateConfig
		pidConfig       createconfig.PidConfig
		networkConfig   createconfig.NetworkConfig
		cgroupConfig    createconfig.CgroupConfig
		utsConfig       createconfig.UtsConfig
		ipcConfig       createconfig.IpcConfig
		userConfig      createconfig.UserConfig
		securityConfig  createconfig.SecurityConfig
	)

	// The default for MemorySwappiness is -1, not 0
	containerConfig.Resources.MemorySwappiness = -1

	containerConfig.Image = containerYAML.Image
	containerConfig.ImageID = newImage.ID()
	containerConfig.Name = containerYAML.Name
	containerConfig.Tty = containerYAML.TTY

	containerConfig.Pod = podID

	imageData, _ := newImage.Inspect(ctx)

	userConfig.User = "0"
	if imageData != nil {
		userConfig.User = imageData.Config.User
	}

	setupSecurityContext(&securityConfig, &userConfig, containerYAML)

	securityConfig.SeccompProfilePath = seccompPaths.findForContainer(containerConfig.Name)

	containerConfig.Command = []string{}
	if imageData != nil && imageData.Config != nil {
		containerConfig.Command = append(containerConfig.Command, imageData.Config.Entrypoint...)
	}
	if len(containerYAML.Command) != 0 {
		containerConfig.Command = append(containerConfig.Command, containerYAML.Command...)
	} else if imageData != nil && imageData.Config != nil {
		containerConfig.Command = append(containerConfig.Command, imageData.Config.Cmd...)
	}
	if imageData != nil && len(containerConfig.Command) == 0 {
		return nil, errors.Errorf("No command specified in container YAML or as CMD or ENTRYPOINT in this image for %s", containerConfig.Name)
	}

	containerConfig.UserCommand = containerConfig.Command

	containerConfig.StopSignal = 15

	containerConfig.WorkDir = "/"
	if imageData != nil {
		// FIXME,
		// we are currently ignoring imageData.Config.ExposedPorts
		containerConfig.BuiltinImgVolumes = imageData.Config.Volumes
		if imageData.Config.WorkingDir != "" {
			containerConfig.WorkDir = imageData.Config.WorkingDir
		}
		containerConfig.Labels = imageData.Config.Labels
		if imageData.Config.StopSignal != "" {
			stopSignal, err := util.ParseSignal(imageData.Config.StopSignal)
			if err != nil {
				return nil, err
			}
			containerConfig.StopSignal = stopSignal
		}
	}

	if containerYAML.WorkingDir != "" {
		containerConfig.WorkDir = containerYAML.WorkingDir
	}
	// If the user does not pass in ID mappings, just set to basics
	if userConfig.IDMappings == nil {
		userConfig.IDMappings = &storage.IDMappingOptions{}
	}

	networkConfig.NetMode = ns.NetworkMode(namespaces["net"])
	ipcConfig.IpcMode = ns.IpcMode(namespaces["ipc"])
	utsConfig.UtsMode = ns.UTSMode(namespaces["uts"])
	// disabled in code review per mheon
	//containerConfig.PidMode = ns.PidMode(namespaces["pid"])
	userConfig.UsernsMode = ns.UsernsMode(namespaces["user"])
	if len(containerConfig.WorkDir) == 0 {
		containerConfig.WorkDir = "/"
	}

	containerConfig.Pid = pidConfig
	containerConfig.Network = networkConfig
	containerConfig.Uts = utsConfig
	containerConfig.Ipc = ipcConfig
	containerConfig.Cgroup = cgroupConfig
	containerConfig.User = userConfig
	containerConfig.Security = securityConfig

	annotations := make(map[string]string)
	if infraID != "" {
		annotations[ann.SandboxID] = infraID
		annotations[ann.ContainerType] = ann.ContainerTypeContainer
	}
	containerConfig.Annotations = annotations

	// Environment Variables
	envs := map[string]string{}
	if imageData != nil {
		imageEnv, err := envLib.ParseSlice(imageData.Config.Env)
		if err != nil {
			return nil, errors.Wrap(err, "error parsing image environment variables")
		}
		envs = imageEnv
	}
	for _, e := range containerYAML.Env {
		envs[e.Name] = e.Value
	}
	containerConfig.Env = envs

	for _, volume := range containerYAML.VolumeMounts {
		hostPath, exists := volumes[volume.Name]
		if !exists {
			return nil, errors.Errorf("Volume mount %s specified for container but not configured in volumes", volume.Name)
		}
		if err := parse.ValidateVolumeCtrDir(volume.MountPath); err != nil {
			return nil, errors.Wrapf(err, "error in parsing MountPath")
		}
		containerConfig.Volumes = append(containerConfig.Volumes, fmt.Sprintf("%s:%s", hostPath, volume.MountPath))
	}
	return &containerConfig, nil
}

// kubeSeccompPaths holds information about a pod YAML's seccomp configuration
// it holds both container and pod seccomp paths
type kubeSeccompPaths struct {
	containerPaths map[string]string
	podPath        string
}

// findForContainer checks whether a container has a seccomp path configured for it
// if not, it returns the podPath, which should always have a value
func (k *kubeSeccompPaths) findForContainer(ctrName string) string {
	if path, ok := k.containerPaths[ctrName]; ok {
		return path
	}
	return k.podPath
}

// initializeSeccompPaths takes annotations from the pod object metadata and finds annotations pertaining to seccomp
// it parses both pod and container level
// if the annotation is of the form "localhost/%s", the seccomp profile will be set to profileRoot/%s
func initializeSeccompPaths(annotations map[string]string, profileRoot string) (*kubeSeccompPaths, error) {
	seccompPaths := &kubeSeccompPaths{containerPaths: make(map[string]string)}
	var err error
	if annotations != nil {
		for annKeyValue, seccomp := range annotations {
			// check if it is prefaced with container.seccomp.security.alpha.kubernetes.io/
			prefixAndCtr := strings.Split(annKeyValue, "/")
			if prefixAndCtr[0]+"/" != v1.SeccompContainerAnnotationKeyPrefix {
				continue
			} else if len(prefixAndCtr) != 2 {
				// this could be caused by a user inputting either of
				// container.seccomp.security.alpha.kubernetes.io{,/}
				// both of which are invalid
				return nil, errors.Errorf("Invalid seccomp path: %s", prefixAndCtr[0])
			}

			path, err := verifySeccompPath(seccomp, profileRoot)
			if err != nil {
				return nil, err
			}
			seccompPaths.containerPaths[prefixAndCtr[1]] = path
		}

		podSeccomp, ok := annotations[v1.SeccompPodAnnotationKey]
		if ok {
			seccompPaths.podPath, err = verifySeccompPath(podSeccomp, profileRoot)
		} else {
			seccompPaths.podPath, err = libpod.DefaultSeccompPath()
		}
		if err != nil {
			return nil, err
		}
	}
	return seccompPaths, nil
}

// verifySeccompPath takes a path and checks whether it is a default, unconfined, or a path
// the available options are parsed as defined in https://kubernetes.io/docs/concepts/policy/pod-security-policy/#seccomp
func verifySeccompPath(path string, profileRoot string) (string, error) {
	switch path {
	case v1.DeprecatedSeccompProfileDockerDefault:
		fallthrough
	case v1.SeccompProfileRuntimeDefault:
		return libpod.DefaultSeccompPath()
	case "unconfined":
		return path, nil
	default:
		parts := strings.Split(path, "/")
		if parts[0] == "localhost" {
			return filepath.Join(profileRoot, parts[1]), nil
		}
		return "", errors.Errorf("invalid seccomp path: %s", path)
	}
}
