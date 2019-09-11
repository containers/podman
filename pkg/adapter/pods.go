// +build !remoteclient

package adapter

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/shared"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/adapter/shortcuts"
	ns "github.com/containers/libpod/pkg/namespaces"
	createconfig "github.com/containers/libpod/pkg/spec"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

const (
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	createDirectoryPermission = 0755
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	createFilePermission = 0644
)

// PodContainerStats is struct containing an adapter Pod and a libpod
// ContainerStats and is used primarily for outputing pod stats.
type PodContainerStats struct {
	Pod            *Pod
	ContainerStats map[string]*libpod.ContainerStats
}

// PrunePods removes pods
func (r *LocalRuntime) PrunePods(ctx context.Context, cli *cliconfig.PodPruneValues) ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	maxWorkers := shared.DefaultPoolSize("rm")
	if cli.GlobalIsSet("max-workers") {
		maxWorkers = cli.GlobalFlags.MaxWorks
	}
	logrus.Debugf("Setting maximum rm workers to %d", maxWorkers)

	states := []string{shared.PodStateStopped, shared.PodStateExited}
	if cli.Force {
		states = append(states, shared.PodStateRunning)
	}

	pods, err := r.GetPodsByStatus(states)
	if err != nil {
		return ok, failures, err
	}
	if len(pods) < 1 {
		return ok, failures, nil
	}

	pool := shared.NewPool("pod_prune", maxWorkers, len(pods))
	for _, p := range pods {
		p := p

		pool.Add(shared.Job{
			ID: p.ID(),
			Fn: func() error {
				err := r.Runtime.RemovePod(ctx, p, cli.Force, cli.Force)
				if err != nil {
					logrus.Debugf("Failed to remove pod %s: %s", p.ID(), err.Error())
				}
				return err
			},
		})
	}
	return pool.Run()
}

// RemovePods ...
func (r *LocalRuntime) RemovePods(ctx context.Context, cli *cliconfig.PodRmValues) ([]string, []error) {
	var (
		errs   []error
		podids []string
	)
	pods, err := shortcuts.GetPodsByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}

	for _, p := range pods {
		if err := r.Runtime.RemovePod(ctx, p, cli.Force, cli.Force); err != nil {
			errs = append(errs, err)
		} else {
			podids = append(podids, p.ID())
		}
	}
	return podids, errs
}

// GetLatestPod gets the latest pod and wraps it in an adapter pod
func (r *LocalRuntime) GetLatestPod() (*Pod, error) {
	pod := Pod{}
	p, err := r.Runtime.GetLatestPod()
	pod.Pod = p
	return &pod, err
}

// GetAllPods gets all pods and wraps it in an adapter pod
func (r *LocalRuntime) GetAllPods() ([]*Pod, error) {
	var pods []*Pod
	allPods, err := r.Runtime.GetAllPods()
	if err != nil {
		return nil, err
	}
	for _, p := range allPods {
		pod := Pod{}
		pod.Pod = p
		pods = append(pods, &pod)
	}
	return pods, nil
}

// LookupPod gets a pod by name or id and wraps it in an adapter pod
func (r *LocalRuntime) LookupPod(nameOrID string) (*Pod, error) {
	pod := Pod{}
	p, err := r.Runtime.LookupPod(nameOrID)
	pod.Pod = p
	return &pod, err
}

// StopPods is a wrapper to libpod to stop pods based on a cli context
func (r *LocalRuntime) StopPods(ctx context.Context, cli *cliconfig.PodStopValues) ([]string, []error) {
	timeout := -1
	if cli.Flags().Changed("timeout") {
		timeout = int(cli.Timeout)
	}
	var (
		errs   []error
		podids []string
	)
	pods, err := shortcuts.GetPodsByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}

	for _, p := range pods {
		stopped := true
		conErrs, stopErr := p.StopWithTimeout(ctx, true, timeout)
		if stopErr != nil {
			errs = append(errs, stopErr)
			stopped = false
		}
		if conErrs != nil {
			stopped = false
			for _, err := range conErrs {
				errs = append(errs, err)
			}
		}
		if stopped {
			podids = append(podids, p.ID())
		}
	}
	return podids, errs
}

// KillPods is a wrapper to libpod to start pods based on the cli context
func (r *LocalRuntime) KillPods(ctx context.Context, cli *cliconfig.PodKillValues, signal uint) ([]string, []error) {
	var (
		errs   []error
		podids []string
	)
	pods, err := shortcuts.GetPodsByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}
	for _, p := range pods {
		killed := true
		conErrs, killErr := p.Kill(signal)
		if killErr != nil {
			errs = append(errs, killErr)
			killed = false
		}
		if conErrs != nil {
			killed = false
			for _, err := range conErrs {
				errs = append(errs, err)
			}
		}
		if killed {
			podids = append(podids, p.ID())
		}
	}
	return podids, errs
}

// StartPods is a wrapper to start pods based on the cli context
func (r *LocalRuntime) StartPods(ctx context.Context, cli *cliconfig.PodStartValues) ([]string, []error) {
	var (
		errs   []error
		podids []string
	)
	pods, err := shortcuts.GetPodsByContext(cli.All, cli.Latest, cli.InputArgs, r.Runtime)
	if err != nil {
		errs = append(errs, err)
		return nil, errs
	}
	for _, p := range pods {
		started := true
		conErrs, startErr := p.Start(ctx)
		if startErr != nil {
			errs = append(errs, startErr)
			started = false
		}
		if conErrs != nil {
			started = false
			for _, err := range conErrs {
				errs = append(errs, err)
			}
		}
		if started {
			podids = append(podids, p.ID())
		}
	}
	return podids, errs
}

// CreatePod is a wrapper for libpod and creating a new pod from the cli context
func (r *LocalRuntime) CreatePod(ctx context.Context, cli *cliconfig.PodCreateValues, labels map[string]string) (string, error) {
	var (
		options []libpod.PodCreateOption
		err     error
	)

	if cli.Flag("cgroup-parent").Changed {
		options = append(options, libpod.WithPodCgroupParent(cli.CgroupParent))
	}

	if len(labels) != 0 {
		options = append(options, libpod.WithPodLabels(labels))
	}

	if cli.Flag("name").Changed {
		options = append(options, libpod.WithPodName(cli.Name))
	}

	if cli.Flag("hostname").Changed {
		options = append(options, libpod.WithPodHostname(cli.Hostname))
	}

	if cli.Infra {
		options = append(options, libpod.WithInfraContainer())
		nsOptions, err := shared.GetNamespaceOptions(strings.Split(cli.Share, ","))
		if err != nil {
			return "", err
		}
		options = append(options, nsOptions...)
	}

	if len(cli.Publish) > 0 {
		portBindings, err := shared.CreatePortBindings(cli.Publish)
		if err != nil {
			return "", err
		}
		options = append(options, libpod.WithInfraContainerPorts(portBindings))

	}
	// always have containers use pod cgroups
	// User Opt out is not yet supported
	options = append(options, libpod.WithPodCgroups())

	pod, err := r.NewPod(ctx, options...)
	if err != nil {
		return "", err
	}
	return pod.ID(), nil
}

// GetPodStatus is a wrapper to get the status of a local libpod pod
func (p *Pod) GetPodStatus() (string, error) {
	return shared.GetPodStatus(p.Pod)
}

// BatchContainerOp is a wrapper for the shared function of the same name
func BatchContainerOp(ctr *libpod.Container, opts shared.PsOptions) (shared.BatchContainerStruct, error) {
	return shared.BatchContainerOp(ctr, opts)
}

// PausePods is a wrapper for pausing pods via libpod
func (r *LocalRuntime) PausePods(c *cliconfig.PodPauseValues) ([]string, map[string]error, []error) {
	var (
		pauseIDs    []string
		pauseErrors []error
	)
	containerErrors := make(map[string]error)

	pods, err := shortcuts.GetPodsByContext(c.All, c.Latest, c.InputArgs, r.Runtime)
	if err != nil {
		pauseErrors = append(pauseErrors, err)
		return nil, containerErrors, pauseErrors
	}

	for _, pod := range pods {
		ctrErrs, err := pod.Pause()
		if err != nil {
			pauseErrors = append(pauseErrors, err)
			continue
		}
		if ctrErrs != nil {
			for ctr, err := range ctrErrs {
				containerErrors[ctr] = err
			}
			continue
		}
		pauseIDs = append(pauseIDs, pod.ID())

	}
	return pauseIDs, containerErrors, pauseErrors
}

// UnpausePods is a wrapper for unpausing pods via libpod
func (r *LocalRuntime) UnpausePods(c *cliconfig.PodUnpauseValues) ([]string, map[string]error, []error) {
	var (
		unpauseIDs    []string
		unpauseErrors []error
	)
	containerErrors := make(map[string]error)

	pods, err := shortcuts.GetPodsByContext(c.All, c.Latest, c.InputArgs, r.Runtime)
	if err != nil {
		unpauseErrors = append(unpauseErrors, err)
		return nil, containerErrors, unpauseErrors
	}

	for _, pod := range pods {
		ctrErrs, err := pod.Unpause()
		if err != nil {
			unpauseErrors = append(unpauseErrors, err)
			continue
		}
		if ctrErrs != nil {
			for ctr, err := range ctrErrs {
				containerErrors[ctr] = err
			}
			continue
		}
		unpauseIDs = append(unpauseIDs, pod.ID())

	}
	return unpauseIDs, containerErrors, unpauseErrors
}

// RestartPods is a wrapper to restart pods via libpod
func (r *LocalRuntime) RestartPods(ctx context.Context, c *cliconfig.PodRestartValues) ([]string, map[string]error, []error) {
	var (
		restartIDs    []string
		restartErrors []error
	)
	containerErrors := make(map[string]error)

	pods, err := shortcuts.GetPodsByContext(c.All, c.Latest, c.InputArgs, r.Runtime)
	if err != nil {
		restartErrors = append(restartErrors, err)
		return nil, containerErrors, restartErrors
	}

	for _, pod := range pods {
		ctrErrs, err := pod.Restart(ctx)
		if err != nil {
			restartErrors = append(restartErrors, err)
			continue
		}
		if ctrErrs != nil {
			for ctr, err := range ctrErrs {
				containerErrors[ctr] = err
			}
			continue
		}
		restartIDs = append(restartIDs, pod.ID())

	}
	return restartIDs, containerErrors, restartErrors

}

// PodTop is a wrapper function to call GetPodPidInformation in libpod and return its results
// for output
func (r *LocalRuntime) PodTop(c *cliconfig.PodTopValues, descriptors []string) ([]string, error) {
	var (
		pod *Pod
		err error
	)

	if c.Latest {
		pod, err = r.GetLatestPod()
	} else {
		pod, err = r.LookupPod(c.InputArgs[0])
	}
	if err != nil {
		return nil, errors.Wrapf(err, "unable to lookup requested container")
	}
	podStatus, err := pod.GetPodStatus()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get status for pod %s", pod.ID())
	}
	if podStatus != "Running" {
		return nil, errors.Errorf("pod top can only be used on pods with at least one running container")
	}
	return pod.GetPodPidInformation(descriptors)
}

// GetStatPods returns pods for use in pod stats
func (r *LocalRuntime) GetStatPods(c *cliconfig.PodStatsValues) ([]*Pod, error) {
	var (
		adapterPods []*Pod
		pods        []*libpod.Pod
		err         error
	)

	if len(c.InputArgs) > 0 || c.Latest || c.All {
		pods, err = shortcuts.GetPodsByContext(c.All, c.Latest, c.InputArgs, r.Runtime)
	} else {
		pods, err = r.Runtime.GetRunningPods()
	}
	if err != nil {
		return nil, err
	}
	// convert libpod pods to adapter pods
	for _, p := range pods {
		adapterPod := Pod{
			p,
		}
		adapterPods = append(adapterPods, &adapterPod)
	}
	return adapterPods, nil
}

// PlayKubeYAML creates pods and containers from a kube YAML file
func (r *LocalRuntime) PlayKubeYAML(ctx context.Context, c *cliconfig.KubePlayValues, yamlFile string) (*Pod, error) {
	var (
		containers    []*libpod.Container
		pod           *libpod.Pod
		podOptions    []libpod.PodCreateOption
		podYAML       v1.Pod
		registryCreds *types.DockerAuthConfig
		writer        io.Writer
	)

	content, err := ioutil.ReadFile(yamlFile)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(content, &podYAML); err != nil {
		return nil, errors.Wrapf(err, "unable to read %s as YAML", yamlFile)
	}

	if podYAML.Kind != "Pod" {
		return nil, errors.Errorf("Invalid YAML kind: %s. Pod is the only supported Kubernetes YAML kind", podYAML.Kind)
	}

	// check for name collision between pod and container
	podName := podYAML.ObjectMeta.Name
	if podName == "" {
		return nil, errors.Errorf("pod does not have a name")
	}
	for _, n := range podYAML.Spec.Containers {
		if n.Name == podName {
			fmt.Printf("a container exists with the same name (%s) as the pod in your YAML file; changing pod name to %s_pod\n", podName, podName)
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

	nsOptions, err := shared.GetNamespaceOptions(strings.Split(shared.DefaultKernelNamespaces, ","))
	if err != nil {
		return nil, err
	}
	podOptions = append(podOptions, nsOptions...)
	podPorts := getPodPorts(podYAML.Spec.Containers)
	podOptions = append(podOptions, libpod.WithInfraContainerPorts(podPorts))

	// Create the Pod
	pod, err = r.NewPod(ctx, podOptions...)
	if err != nil {
		return nil, err
	}

	podInfraID, err := pod.InfraContainerID()
	if err != nil {
		return nil, err
	}
	hasUserns := false
	if podInfraID != "" {
		podCtr, err := r.GetContainer(podInfraID)
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
	if !c.Quiet {
		writer = os.Stderr
	}

	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerRegistryCreds: registryCreds,
		DockerCertPath:      c.CertDir,
	}
	if c.Flag("tls-verify").Changed {
		dockerRegistryOptions.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!c.TlsVerify)
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
					if err := os.Mkdir(hostPath.Path, createDirectoryPermission); err != nil {
						return nil, errors.Errorf("Error creating HostPath %s at %s", volume.Name, hostPath.Path)
					}
				}
				// unconditionally label a newly created volume as private
				if err := libpod.LabelVolumePath(hostPath.Path, false); err != nil {
					return nil, errors.Wrapf(err, "Error giving %s a label", hostPath.Path)
				}
			case v1.HostPathFileOrCreate:
				if _, err := os.Stat(hostPath.Path); os.IsNotExist(err) {
					f, err := os.OpenFile(hostPath.Path, os.O_RDONLY|os.O_CREATE, createFilePermission)
					if err != nil {
						return nil, errors.Errorf("Error creating HostPath %s at %s", volume.Name, hostPath.Path)
					}
					if err := f.Close(); err != nil {
						logrus.Warnf("Error in closing newly created HostPath file: %v", err)
					}
				}
				// unconditionally label a newly created volume as private
				if err := libpod.LabelVolumePath(hostPath.Path, false); err != nil {
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

	for _, container := range podYAML.Spec.Containers {
		newImage, err := r.ImageRuntime().New(ctx, container.Image, c.SignaturePolicy, c.Authfile, writer, &dockerRegistryOptions, image.SigningOptions{}, nil, util.PullImageMissing)
		if err != nil {
			return nil, err
		}
		createConfig, err := kubeContainerToCreateConfig(ctx, container, r.Runtime, newImage, namespaces, volumes, pod.ID())
		if err != nil {
			return nil, err
		}
		ctr, err := shared.CreateContainerFromCreateConfig(r.Runtime, createConfig, ctx, pod)
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

	// We've now successfully converted this YAML into a pod
	// print our pod and containers, signifying we succeeded
	fmt.Printf("Pod:\n%s\n", pod.ID())
	if len(containers) == 1 {
		fmt.Printf("Container:\n")
	}
	if len(containers) > 1 {
		fmt.Printf("Containers:\n")
	}
	for _, ctr := range containers {
		fmt.Println(ctr.ID())
	}

	if err := playcleanup(ctx, r, pod, nil); err != nil {
		logrus.Errorf("unable to remove pod %s after failing to play kube", pod.ID())
	}
	return nil, nil
}

func playcleanup(ctx context.Context, runtime *LocalRuntime, pod *libpod.Pod, err error) error {
	if err != nil && pod != nil {
		return runtime.RemovePod(ctx, pod, true, true)
	}
	return nil
}

// getPodPorts converts a slice of kube container descriptions to an
// array of ocicni portmapping descriptions usable in libpod
func getPodPorts(containers []v1.Container) []ocicni.PortMapping {
	var infraPorts []ocicni.PortMapping
	for _, container := range containers {
		for _, p := range container.Ports {
			portBinding := ocicni.PortMapping{
				HostPort:      p.HostPort,
				ContainerPort: p.ContainerPort,
				Protocol:      strings.ToLower(string(p.Protocol)),
			}
			if p.HostIP != "" {
				logrus.Debug("HostIP on port bindings is not supported")
			}
			infraPorts = append(infraPorts, portBinding)
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
func kubeContainerToCreateConfig(ctx context.Context, containerYAML v1.Container, runtime *libpod.Runtime, newImage *image.Image, namespaces map[string]string, volumes map[string]string, podID string) (*createconfig.CreateConfig, error) {
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
	containerConfig.WorkDir = containerYAML.WorkingDir

	containerConfig.Pod = podID

	imageData, _ := newImage.Inspect(ctx)

	userConfig.User = "0"
	if imageData != nil {
		userConfig.User = imageData.Config.User
	}

	setupSecurityContext(&securityConfig, &userConfig, containerYAML)

	var err error
	containerConfig.Security.SeccompProfilePath, err = libpod.DefaultSeccompPath()
	if err != nil {
		return nil, err
	}

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

	// Set default environment variables and incorporate data from image, if necessary
	envs := shared.EnvVariablesFromData(imageData)

	// Environment Variables
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
