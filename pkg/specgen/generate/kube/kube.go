package kube

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/containers/common/pkg/parse"
	"github.com/containers/podman/v3/libpod/image"
	ann "github.com/containers/podman/v3/pkg/annotations"
	"github.com/containers/podman/v3/pkg/specgen"
	"github.com/containers/podman/v3/pkg/util"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func ToPodGen(ctx context.Context, podName string, podYAML *v1.PodTemplateSpec) (*specgen.PodSpecGenerator, error) {
	p := specgen.NewPodSpecGenerator()
	p.Name = podName
	p.Labels = podYAML.ObjectMeta.Labels
	// TODO we only configure Process namespace. We also need to account for Host{IPC,Network,PID}
	// which is not currently possible with pod create
	if podYAML.Spec.ShareProcessNamespace != nil && *podYAML.Spec.ShareProcessNamespace {
		p.SharedNamespaces = append(p.SharedNamespaces, "pid")
	}
	p.Hostname = podYAML.Spec.Hostname
	if p.Hostname == "" {
		p.Hostname = podName
	}
	if podYAML.Spec.HostNetwork {
		p.NetNS.NSMode = specgen.Host
	}
	if podYAML.Spec.HostAliases != nil {
		hosts := make([]string, 0, len(podYAML.Spec.HostAliases))
		for _, hostAlias := range podYAML.Spec.HostAliases {
			for _, host := range hostAlias.Hostnames {
				hosts = append(hosts, host+":"+hostAlias.IP)
			}
		}
		p.HostAdd = hosts
	}
	podPorts := getPodPorts(podYAML.Spec.Containers)
	p.PortMappings = podPorts

	if dnsConfig := podYAML.Spec.DNSConfig; dnsConfig != nil {
		// name servers
		if dnsServers := dnsConfig.Nameservers; len(dnsServers) > 0 {
			servers := make([]net.IP, 0)
			for _, server := range dnsServers {
				servers = append(servers, net.ParseIP(server))
			}
			p.DNSServer = servers
		}
		// search domans
		if domains := dnsConfig.Searches; len(domains) > 0 {
			p.DNSSearch = domains
		}
		// dns options
		if options := dnsConfig.Options; len(options) > 0 {
			dnsOptions := make([]string, 0)
			for _, opts := range options {
				d := opts.Name
				if opts.Value != nil {
					d += ":" + *opts.Value
				}
				dnsOptions = append(dnsOptions, d)
			}
		}
	}
	return p, nil
}

type CtrSpecGenOptions struct {
	// Container as read from the pod yaml
	Container v1.Container
	// Image available to use (pulled or found local)
	Image *image.Image
	// Volumes for all containers
	Volumes map[string]*KubeVolume
	// PodID of the parent pod
	PodID string
	// PodName of the parent pod
	PodName string
	// PodInfraID as the infrastructure container id
	PodInfraID string
	// ConfigMaps the configuration maps for environment variables
	ConfigMaps []v1.ConfigMap
	// SeccompPaths for finding the seccomp profile path
	SeccompPaths *KubeSeccompPaths
	// RestartPolicy defines the restart policy of the container
	RestartPolicy string
	// NetNSIsHost tells the container to use the host netns
	NetNSIsHost bool
}

func ToSpecGen(ctx context.Context, opts *CtrSpecGenOptions) (*specgen.SpecGenerator, error) {
	s := specgen.NewSpecGenerator(opts.Container.Image, false)

	// pod name should be non-empty for Deployment objects to be able to create
	// multiple pods having containers with unique names
	if len(opts.PodName) < 1 {
		return nil, errors.Errorf("got empty pod name on container creation when playing kube")
	}

	s.Name = fmt.Sprintf("%s-%s", opts.PodName, opts.Container.Name)

	s.Terminal = opts.Container.TTY

	s.Pod = opts.PodID

	setupSecurityContext(s, opts.Container)

	// Since we prefix the container name with pod name to work-around the uniqueness requirement,
	// the seccomp profile should reference the actual container name from the YAML
	// but apply to the containers with the prefixed name
	s.SeccompProfilePath = opts.SeccompPaths.FindForContainer(opts.Container.Name)

	s.ResourceLimits = &spec.LinuxResources{}
	milliCPU, err := quantityToInt64(opts.Container.Resources.Limits.Cpu())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to set CPU quota")
	}
	if milliCPU > 0 {
		period, quota := util.CoresToPeriodAndQuota(float64(milliCPU) / 1000)
		s.ResourceLimits.CPU = &spec.LinuxCPU{
			Quota:  &quota,
			Period: &period,
		}
	}

	limit, err := quantityToInt64(opts.Container.Resources.Limits.Memory())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to set memory limit")
	}

	memoryRes, err := quantityToInt64(opts.Container.Resources.Requests.Memory())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to set memory reservation")
	}

	if limit > 0 || memoryRes > 0 {
		s.ResourceLimits.Memory = &spec.LinuxMemory{}
	}

	if limit > 0 {
		s.ResourceLimits.Memory.Limit = &limit
	}

	if memoryRes > 0 {
		s.ResourceLimits.Memory.Reservation = &memoryRes
	}

	// TODO: We don't understand why specgen does not take of this, but
	// integration tests clearly pointed out that it was required.
	imageData, err := opts.Image.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	s.WorkDir = "/"
	// Entrypoint/Command handling is based off of
	// https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#notes
	if imageData != nil && imageData.Config != nil {
		if imageData.Config.WorkingDir != "" {
			s.WorkDir = imageData.Config.WorkingDir
		}
		// Pull entrypoint and cmd from image
		s.Entrypoint = imageData.Config.Entrypoint
		s.Command = imageData.Config.Cmd
		s.Labels = imageData.Config.Labels
		if len(imageData.Config.StopSignal) > 0 {
			stopSignal, err := util.ParseSignal(imageData.Config.StopSignal)
			if err != nil {
				return nil, err
			}
			s.StopSignal = &stopSignal
		}
	}
	// If only the yaml.Command is specified, set it as the entrypoint and drop the image Cmd
	if len(opts.Container.Command) != 0 {
		s.Entrypoint = opts.Container.Command
		s.Command = []string{}
	}
	// Only override the cmd field if yaml.Args is specified
	// Keep the image entrypoint, or the yaml.command if specified
	if len(opts.Container.Args) != 0 {
		s.Command = opts.Container.Args
	}

	// FIXME,
	// we are currently ignoring imageData.Config.ExposedPorts
	if opts.Container.WorkingDir != "" {
		s.WorkDir = opts.Container.WorkingDir
	}

	annotations := make(map[string]string)
	if opts.PodInfraID != "" {
		annotations[ann.SandboxID] = opts.PodInfraID
		annotations[ann.ContainerType] = ann.ContainerTypeContainer
	}
	s.Annotations = annotations

	// Environment Variables
	envs := map[string]string{}
	for _, env := range imageData.Config.Env {
		keyval := strings.Split(env, "=")
		envs[keyval[0]] = keyval[1]
	}

	for _, env := range opts.Container.Env {
		value := envVarValue(env, opts.ConfigMaps)

		envs[env.Name] = value
	}
	for _, envFrom := range opts.Container.EnvFrom {
		cmEnvs := envVarsFromConfigMap(envFrom, opts.ConfigMaps)

		for k, v := range cmEnvs {
			envs[k] = v
		}
	}
	s.Env = envs

	for _, volume := range opts.Container.VolumeMounts {
		volumeSource, exists := opts.Volumes[volume.Name]
		if !exists {
			return nil, errors.Errorf("Volume mount %s specified for container but not configured in volumes", volume.Name)
		}
		switch volumeSource.Type {
		case KubeVolumeTypeBindMount:
			if err := parse.ValidateVolumeCtrDir(volume.MountPath); err != nil {
				return nil, errors.Wrapf(err, "error in parsing MountPath")
			}
			mount := spec.Mount{
				Destination: volume.MountPath,
				Source:      volumeSource.Source,
				Type:        "bind",
			}
			if volume.ReadOnly {
				mount.Options = []string{"ro"}
			}
			s.Mounts = append(s.Mounts, mount)
		case KubeVolumeTypeNamed:
			namedVolume := specgen.NamedVolume{
				Dest: volume.MountPath,
				Name: volumeSource.Source,
			}
			if volume.ReadOnly {
				namedVolume.Options = []string{"ro"}
			}
			s.Volumes = append(s.Volumes, &namedVolume)
		default:
			return nil, errors.Errorf("Unsupported volume source type")
		}
	}

	s.RestartPolicy = opts.RestartPolicy

	if opts.NetNSIsHost {
		s.NetNS.NSMode = specgen.Host
	}

	return s, nil
}

func setupSecurityContext(s *specgen.SpecGenerator, containerYAML v1.Container) {
	if containerYAML.SecurityContext == nil {
		return
	}
	if containerYAML.SecurityContext.ReadOnlyRootFilesystem != nil {
		s.ReadOnlyFilesystem = *containerYAML.SecurityContext.ReadOnlyRootFilesystem
	}
	if containerYAML.SecurityContext.Privileged != nil {
		s.Privileged = *containerYAML.SecurityContext.Privileged
	}

	if containerYAML.SecurityContext.AllowPrivilegeEscalation != nil {
		s.NoNewPrivileges = !*containerYAML.SecurityContext.AllowPrivilegeEscalation
	}

	if seopt := containerYAML.SecurityContext.SELinuxOptions; seopt != nil {
		if seopt.User != "" {
			s.SelinuxOpts = append(s.SelinuxOpts, fmt.Sprintf("user:%s", seopt.User))
		}
		if seopt.Role != "" {
			s.SelinuxOpts = append(s.SelinuxOpts, fmt.Sprintf("role:%s", seopt.Role))
		}
		if seopt.Type != "" {
			s.SelinuxOpts = append(s.SelinuxOpts, fmt.Sprintf("type:%s", seopt.Type))
		}
		if seopt.Level != "" {
			s.SelinuxOpts = append(s.SelinuxOpts, fmt.Sprintf("level:%s", seopt.Level))
		}
	}
	if caps := containerYAML.SecurityContext.Capabilities; caps != nil {
		for _, capability := range caps.Add {
			s.CapAdd = append(s.CapAdd, string(capability))
		}
		for _, capability := range caps.Drop {
			s.CapDrop = append(s.CapDrop, string(capability))
		}
	}
	if containerYAML.SecurityContext.RunAsUser != nil {
		s.User = fmt.Sprintf("%d", *containerYAML.SecurityContext.RunAsUser)
	}
	if containerYAML.SecurityContext.RunAsGroup != nil {
		if s.User == "" {
			s.User = "0"
		}
		s.User = fmt.Sprintf("%s:%d", s.User, *containerYAML.SecurityContext.RunAsGroup)
	}
}

func quantityToInt64(quantity *resource.Quantity) (int64, error) {
	if i, ok := quantity.AsInt64(); ok {
		return i, nil
	}

	if i, ok := quantity.AsDec().Unscaled(); ok {
		return i, nil
	}

	return 0, errors.Errorf("Quantity cannot be represented as int64: %v", quantity)
}

// envVarsFromConfigMap returns all key-value pairs as env vars from a configMap that matches the envFrom setting of a container
func envVarsFromConfigMap(envFrom v1.EnvFromSource, configMaps []v1.ConfigMap) map[string]string {
	envs := map[string]string{}

	if envFrom.ConfigMapRef != nil {
		cmName := envFrom.ConfigMapRef.Name

		for _, c := range configMaps {
			if cmName == c.Name {
				envs = c.Data
				break
			}
		}
	}

	return envs
}

// envVarValue returns the environment variable value configured within the container's env setting.
// It gets the value from a configMap if specified, otherwise returns env.Value
func envVarValue(env v1.EnvVar, configMaps []v1.ConfigMap) string {
	for _, c := range configMaps {
		if env.ValueFrom != nil {
			if env.ValueFrom.ConfigMapKeyRef != nil {
				if env.ValueFrom.ConfigMapKeyRef.Name == c.Name {
					if value, ok := c.Data[env.ValueFrom.ConfigMapKeyRef.Key]; ok {
						return value
					}
				}
			}
		}
	}

	return env.Value
}

// getPodPorts converts a slice of kube container descriptions to an
// array of portmapping
func getPodPorts(containers []v1.Container) []specgen.PortMapping {
	var infraPorts []specgen.PortMapping
	for _, container := range containers {
		for _, p := range container.Ports {
			if p.HostPort != 0 && p.ContainerPort == 0 {
				p.ContainerPort = p.HostPort
			}
			if p.Protocol == "" {
				p.Protocol = "tcp"
			}
			portBinding := specgen.PortMapping{
				HostPort:      uint16(p.HostPort),
				ContainerPort: uint16(p.ContainerPort),
				Protocol:      strings.ToLower(string(p.Protocol)),
				HostIP:        p.HostIP,
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
