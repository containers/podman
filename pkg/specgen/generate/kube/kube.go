//go:build !remote

package kube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/containers/common/libimage"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/parse"
	"github.com/containers/common/pkg/secrets"
	"github.com/containers/image/v5/manifest"
	itypes "github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod/define"
	ann "github.com/containers/podman/v4/pkg/annotations"
	"github.com/containers/podman/v4/pkg/domain/entities"
	v1 "github.com/containers/podman/v4/pkg/k8s.io/api/core/v1"
	"github.com/containers/podman/v4/pkg/k8s.io/apimachinery/pkg/api/resource"
	"github.com/containers/podman/v4/pkg/k8s.io/apimachinery/pkg/util/intstr"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgen/generate"
	systemdDefine "github.com/containers/podman/v4/pkg/systemd/define"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/docker/docker/pkg/meminfo"
	"github.com/docker/go-units"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
	"sigs.k8s.io/yaml"
)

func ToPodOpt(ctx context.Context, podName string, p entities.PodCreateOptions, publishAllPorts bool, podYAML *v1.PodTemplateSpec) (entities.PodCreateOptions, error) {
	p.Net = &entities.NetOptions{NoHosts: p.Net.NoHosts}

	p.Name = podName
	p.Labels = podYAML.ObjectMeta.Labels
	// Kube pods must share {ipc, net, uts} by default
	p.Share = append(p.Share, "ipc")
	p.Share = append(p.Share, "net")
	p.Share = append(p.Share, "uts")
	// TODO we only configure Process namespace. We also need to account for Host{IPC,Network,PID}
	// which is not currently possible with pod create
	if podYAML.Spec.ShareProcessNamespace != nil && *podYAML.Spec.ShareProcessNamespace {
		p.Share = append(p.Share, "pid")
	}
	if podYAML.Spec.HostPID {
		p.Pid = "host"
	}
	if podYAML.Spec.HostIPC {
		p.Ipc = "host"
	}
	p.Hostname = podYAML.Spec.Hostname
	if p.Hostname == "" {
		p.Hostname = podName
	}
	if podYAML.Spec.HostNetwork {
		p.Net.Network = specgen.Namespace{NSMode: "host"}
		nodeHostName, err := os.Hostname()
		if err != nil {
			return p, err
		}
		p.Hostname = nodeHostName
		p.Uts = "host"
	}
	if podYAML.Spec.HostAliases != nil {
		if p.Net.NoHosts {
			return p, errors.New("HostAliases in yaml file will not work with --no-hosts")
		}
		hosts := make([]string, 0, len(podYAML.Spec.HostAliases))
		for _, hostAlias := range podYAML.Spec.HostAliases {
			for _, host := range hostAlias.Hostnames {
				hosts = append(hosts, host+":"+hostAlias.IP)
			}
		}
		p.Net.AddHosts = hosts
	}
	podPorts := getPodPorts(podYAML.Spec.Containers, publishAllPorts)
	p.Net.PublishPorts = podPorts

	if dnsConfig := podYAML.Spec.DNSConfig; dnsConfig != nil {
		// name servers
		if dnsServers := dnsConfig.Nameservers; len(dnsServers) > 0 {
			servers := make([]net.IP, 0)
			for _, server := range dnsServers {
				servers = append(servers, net.ParseIP(server))
			}
			p.Net.DNSServers = servers
		}
		// search domains
		if domains := dnsConfig.Searches; len(domains) > 0 {
			p.Net.DNSSearch = domains
		}
		// dns options
		if options := dnsConfig.Options; len(options) > 0 {
			dnsOptions := make([]string, 0, len(options))
			for _, opts := range options {
				d := opts.Name
				if opts.Value != nil {
					d += ":" + *opts.Value
				}
				dnsOptions = append(dnsOptions, d)
			}
			p.Net.DNSOptions = dnsOptions
		}
	}

	if pscConfig := podYAML.Spec.SecurityContext; pscConfig != nil {
		// Extract sysctl list from pod security context
		if options := pscConfig.Sysctls; len(options) > 0 {
			sysctlOptions := make([]string, 0, len(options))
			for _, opts := range options {
				sysctlOptions = append(sysctlOptions, opts.Name+"="+opts.Value)
			}
			p.Sysctl = sysctlOptions
		}
	}
	return p, nil
}

type CtrSpecGenOptions struct {
	// Annotations from the Pod
	Annotations map[string]string
	// Container as read from the pod yaml
	Container v1.Container
	// Image available to use (pulled or found local)
	Image *libimage.Image
	// IPCNSIsHost tells the container to use the host ipcns
	IpcNSIsHost bool
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
	// ReadOnly make all containers root file system readonly
	ReadOnly itypes.OptionalBool
	// RestartPolicy defines the restart policy of the container
	RestartPolicy string
	// NetNSIsHost tells the container to use the host netns
	NetNSIsHost bool
	// UserNSIsHost tells the container to use the host userns
	UserNSIsHost bool
	// PidNSIsHost tells the container to use the host pidns
	PidNSIsHost bool
	// UtsNSIsHost tells the container to use the host utsns
	UtsNSIsHost bool
	// SecretManager to access the secrets
	SecretsManager *secrets.SecretsManager
	// LogDriver which should be used for the container
	LogDriver string
	// LogOptions log options which should be used for the container
	LogOptions []string
	// Labels define key-value pairs of metadata
	Labels map[string]string
	//
	IsInfra bool
	// InitContainerType sets what type the init container is
	// Note: When playing a kube yaml, the inti container type will be set to "always" only
	InitContainerType string
	// PodSecurityContext is the security context specified for the pod
	PodSecurityContext *v1.PodSecurityContext
	// TerminationGracePeriodSeconds is the grace period given to a container to stop before being forcefully killed
	TerminationGracePeriodSeconds *int64
}

func ToSpecGen(ctx context.Context, opts *CtrSpecGenOptions) (*specgen.SpecGenerator, error) {
	s := specgen.NewSpecGenerator(opts.Container.Image, false)

	rtc, err := config.Default()
	if err != nil {
		return nil, err
	}

	if s.Umask == nil {
		umask := rtc.Umask()
		s.Umask = &umask
	}

	if s.CgroupsMode == nil {
		cgroups := rtc.Cgroups()
		s.CgroupsMode = &cgroups
	}
	if s.ImageVolumeMode == nil {
		s.ImageVolumeMode = &rtc.Engine.ImageVolumeMode
	}
	if s.ImageVolumeMode != nil && *s.ImageVolumeMode == define.TypeBind {
		localAnon := "anonymous"
		s.ImageVolumeMode = &localAnon
	}

	// pod name should be non-empty for Deployment objects to be able to create
	// multiple pods having containers with unique names
	if len(opts.PodName) < 1 {
		return nil, errors.New("got empty pod name on container creation when playing kube")
	}

	localName := fmt.Sprintf("%s-%s", opts.PodName, opts.Container.Name)
	s.Name = &localName

	s.Terminal = &opts.Container.TTY

	s.Pod = &opts.PodID

	s.LogConfiguration = &specgen.LogConfig{
		Driver: &opts.LogDriver,
	}

	s.LogConfiguration.Options = make(map[string]string)
	for _, o := range opts.LogOptions {
		opt, val, hasVal := strings.Cut(o, "=")
		if !hasVal {
			return nil, fmt.Errorf("invalid log option %q", o)
		}
		switch strings.ToLower(opt) {
		case "driver":
			s.LogConfiguration.Driver = &val
		case "path":
			s.LogConfiguration.Path = &val
		case "max-size":
			logSize, err := units.FromHumanSize(val)
			if err != nil {
				return nil, err
			}
			s.LogConfiguration.Size = &logSize
		default:
			switch len(val) {
			case 0:
				return nil, fmt.Errorf("invalid log option: %w", define.ErrInvalidArg)
			default:
				// tags for journald only
				if s.LogConfiguration.Driver == nil || *s.LogConfiguration.Driver == define.JournaldLogging {
					s.LogConfiguration.Options[opt] = val
				} else {
					logrus.Warnf("Can only set tags with journald log driver but driver is %q", s.LogConfiguration.Driver)
				}
			}
		}
	}

	s.InitContainerType = &opts.InitContainerType

	setupSecurityContext(s, opts.Container.SecurityContext, opts.PodSecurityContext)
	err = setupLivenessProbe(s, opts.Container, opts.RestartPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to configure livenessProbe: %w", err)
	}
	err = setupStartupProbe(s, opts.Container, opts.RestartPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to configure startupProbe: %w", err)
	}

	// Since we prefix the container name with pod name to work-around the uniqueness requirement,
	// the seccomp profile should reference the actual container name from the YAML
	// but apply to the containers with the prefixed name
	seccompPath := opts.SeccompPaths.FindForContainer(opts.Container.Name)
	s.SeccompProfilePath = &seccompPath

	s.ResourceLimits = &spec.LinuxResources{}
	milliCPU := opts.Container.Resources.Limits.Cpu().MilliValue()
	if milliCPU > 0 {
		period, quota := util.CoresToPeriodAndQuota(float64(milliCPU) / 1000)
		s.ResourceLimits.CPU = &spec.LinuxCPU{
			Quota:  &quota,
			Period: &period,
		}
	}

	limit, err := quantityToInt64(opts.Container.Resources.Limits.Memory())
	if err != nil {
		return nil, fmt.Errorf("failed to set memory limit: %w", err)
	}

	memoryRes, err := quantityToInt64(opts.Container.Resources.Requests.Memory())
	if err != nil {
		return nil, fmt.Errorf("failed to set memory reservation: %w", err)
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

	ulimitVal, ok := opts.Annotations[define.UlimitAnnotation]
	if ok {
		ulimits := strings.Split(ulimitVal, ",")
		for _, ul := range ulimits {
			parsed, err := units.ParseUlimit(ul)
			if err != nil {
				return nil, err
			}
			s.Rlimits = append(s.Rlimits, spec.POSIXRlimit{Type: parsed.Name, Soft: uint64(parsed.Soft), Hard: uint64(parsed.Hard)})
		}
	}

	// TODO: We don't understand why specgen does not take of this, but
	// integration tests clearly pointed out that it was required.
	imageData, err := opts.Image.Inspect(ctx, nil)
	if err != nil {
		return nil, err
	}
	localSlash := "/"
	s.WorkDir = &localSlash
	// Entrypoint/Command handling is based off of
	// https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#notes
	if imageData != nil && imageData.Config != nil {
		if imageData.Config.WorkingDir != "" {
			s.WorkDir = &imageData.Config.WorkingDir
		}
		if s.User == nil {
			s.User = &imageData.Config.User
		}

		exposed, err := generate.GenExposedPorts(imageData.Config.ExposedPorts)
		if err != nil {
			return nil, err
		}

		for k, v := range s.Expose {
			exposed[k] = v
		}
		s.Expose = exposed
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
	if !opts.IsInfra && len(opts.Container.Command) != 0 {
		s.Entrypoint = opts.Container.Command
		s.Command = []string{}
	}
	// Only override the cmd field if yaml.Args is specified
	// Keep the image entrypoint, or the yaml.command if specified
	if !opts.IsInfra && len(opts.Container.Args) != 0 {
		s.Command = opts.Container.Args
	}

	// FIXME,
	// we are currently ignoring imageData.Config.ExposedPorts
	if !opts.IsInfra && opts.Container.WorkingDir != "" {
		s.WorkDir = &opts.Container.WorkingDir
	}

	annotations := make(map[string]string)
	if opts.Annotations != nil {
		annotations = opts.Annotations
	}
	if opts.PodInfraID != "" {
		annotations[ann.SandboxID] = opts.PodInfraID
	}
	s.Annotations = annotations

	if containerCIDFile, ok := opts.Annotations[define.InspectAnnotationCIDFile+"/"+opts.Container.Name]; ok {
		s.Annotations[define.InspectAnnotationCIDFile] = containerCIDFile
	}

	if seccomp, ok := opts.Annotations[define.InspectAnnotationSeccomp+"/"+opts.Container.Name]; ok {
		s.Annotations[define.InspectAnnotationSeccomp] = seccomp
	}

	if apparmor, ok := opts.Annotations[define.InspectAnnotationApparmor+"/"+opts.Container.Name]; ok {
		s.Annotations[define.InspectAnnotationApparmor] = apparmor
	}

	if label, ok := opts.Annotations[define.InspectAnnotationLabel+"/"+opts.Container.Name]; ok {
		if label == "nested" {
			localTrue := true
			s.ContainerSecurityConfig.LabelNested = &localTrue
		}
		if !slices.Contains(s.ContainerSecurityConfig.SelinuxOpts, label) {
			s.ContainerSecurityConfig.SelinuxOpts = append(s.ContainerSecurityConfig.SelinuxOpts, label)
		}
		s.Annotations[define.InspectAnnotationLabel] = strings.Join(s.ContainerSecurityConfig.SelinuxOpts, ",label=")
	}

	if autoremove, ok := opts.Annotations[define.InspectAnnotationAutoremove+"/"+opts.Container.Name]; ok {
		autoremoveAsBool, err := strconv.ParseBool(autoremove)
		if err != nil {
			return nil, err
		}
		s.Remove = &autoremoveAsBool
		s.Annotations[define.InspectAnnotationAutoremove] = autoremove
	}

	if init, ok := opts.Annotations[define.InspectAnnotationInit+"/"+opts.Container.Name]; ok {
		initAsBool, err := strconv.ParseBool(init)
		if err != nil {
			return nil, err
		}

		s.Init = &initAsBool
		s.Annotations[define.InspectAnnotationInit] = init
	}

	if publishAll, ok := opts.Annotations[define.InspectAnnotationPublishAll+"/"+opts.Container.Name]; ok {
		if opts.IsInfra {
			publishAllAsBool, err := strconv.ParseBool(publishAll)
			if err != nil {
				return nil, err
			}
			s.PublishExposedPorts = &publishAllAsBool
		}

		s.Annotations[define.InspectAnnotationPublishAll] = publishAll
	}

	s.Annotations[define.KubeHealthCheckAnnotation] = "true"

	// Environment Variables
	envs := map[string]string{}
	for _, env := range imageData.Config.Env {
		key, val, _ := strings.Cut(env, "=")
		envs[key] = val
	}

	for _, env := range opts.Container.Env {
		value, err := envVarValue(env, opts)
		if err != nil {
			return nil, err
		}

		// Only set the env if the value is not nil
		if value != nil {
			envs[env.Name] = *value
		}
	}
	for _, envFrom := range opts.Container.EnvFrom {
		cmEnvs, err := envVarsFrom(envFrom, opts)
		if err != nil {
			return nil, err
		}

		for k, v := range cmEnvs {
			envs[k] = v
		}
	}
	s.Env = envs

	for _, volume := range opts.Container.VolumeMounts {
		volumeSource, exists := opts.Volumes[volume.Name]
		if !exists {
			return nil, fmt.Errorf("volume mount %s specified for container but not configured in volumes", volume.Name)
		}
		// Skip if the volume is optional. This means that a configmap for a configmap volume was not found but it was
		// optional so we can move on without throwing an error
		if exists && volumeSource.Optional {
			continue
		}

		dest, options, err := parseMountPath(volume.MountPath, volume.ReadOnly, volume.MountPropagation)
		if err != nil {
			return nil, err
		}

		volume.MountPath = dest
		switch volumeSource.Type {
		case KubeVolumeTypeBindMount:
			// If the container has bind mounts, we need to check if
			// a selinux mount option exists for it
			for k, v := range opts.Annotations {
				// Make sure the z/Z option is not already there (from editing the YAML)
				if k == define.BindMountPrefix {
					lastIndex := strings.LastIndex(v, ":")
					if lastIndex != -1 && v[:lastIndex] == volumeSource.Source && !slices.Contains(options, "z") && !slices.Contains(options, "Z") {
						options = append(options, v[lastIndex+1:])
					}
				}
			}
			mount := spec.Mount{
				Destination: volume.MountPath,
				Source:      volumeSource.Source,
				Type:        define.TypeBind,
				Options:     options,
			}
			if len(volume.SubPath) > 0 {
				mount.Options = append(mount.Options, fmt.Sprintf("subpath=%s", volume.SubPath))
			}
			s.Mounts = append(s.Mounts, mount)
		case KubeVolumeTypeNamed:
			namedVolume := specgen.NamedVolume{
				Dest:    volume.MountPath,
				Name:    volumeSource.Source,
				Options: options,
				SubPath: volume.SubPath,
			}
			s.Volumes = append(s.Volumes, &namedVolume)
		case KubeVolumeTypeConfigMap:
			cmVolume := specgen.NamedVolume{
				Dest:    volume.MountPath,
				Name:    volumeSource.Source,
				Options: options,
				SubPath: volume.SubPath,
			}
			s.Volumes = append(s.Volumes, &cmVolume)
		case KubeVolumeTypeCharDevice:
			// We are setting the path as hostPath:mountPath to comply with pkg/specgen/generate.DeviceFromPath.
			// The type is here just to improve readability as it is not taken into account when the actual device is created.
			device := spec.LinuxDevice{
				Path: fmt.Sprintf("%s:%s", volumeSource.Source, volume.MountPath),
				Type: "c",
			}
			s.Devices = append(s.Devices, device)
		case KubeVolumeTypeBlockDevice:
			// We are setting the path as hostPath:mountPath to comply with pkg/specgen/generate.DeviceFromPath.
			// The type is here just to improve readability as it is not taken into account when the actual device is created.
			device := spec.LinuxDevice{
				Path: fmt.Sprintf("%s:%s", volumeSource.Source, volume.MountPath),
				Type: "b",
			}
			s.Devices = append(s.Devices, device)
		case KubeVolumeTypeSecret:
			// in podman play kube we need to add these secrets as volumes rather than as
			// specgen.Secrets. Adding them as volumes allows for all key: value pairs to be mounted
			secretVolume := specgen.NamedVolume{
				Dest:    volume.MountPath,
				Name:    volumeSource.Source,
				Options: options,
				SubPath: volume.SubPath,
			}
			s.Volumes = append(s.Volumes, &secretVolume)
		case KubeVolumeTypeEmptyDir:
			emptyDirVolume := specgen.NamedVolume{
				Dest:        volume.MountPath,
				Name:        volumeSource.Source,
				Options:     options,
				IsAnonymous: true,
				SubPath:     volume.SubPath,
			}
			s.Volumes = append(s.Volumes, &emptyDirVolume)
		default:
			return nil, errors.New("unsupported volume source type")
		}
	}

	s.RestartPolicy = &opts.RestartPolicy

	if opts.NetNSIsHost {
		s.NetNS.NSMode = specgen.Host
	}
	if opts.UserNSIsHost {
		s.UserNS.NSMode = specgen.Host
	}
	if opts.PidNSIsHost {
		s.PidNS.NSMode = specgen.Host
	}
	if opts.IpcNSIsHost {
		s.IpcNS.NSMode = specgen.Host
	}
	if opts.UtsNSIsHost {
		s.UtsNS.NSMode = specgen.Host
	}

	// Add labels that come from kube
	if len(s.Labels) == 0 {
		// If there are no labels, let's use the map that comes
		// from kube
		s.Labels = opts.Labels
	} else {
		// If there are already labels in the map, append the ones
		// obtained from kube
		for k, v := range opts.Labels {
			s.Labels[k] = v
		}
	}

	if ro := opts.ReadOnly; ro != itypes.OptionalBoolUndefined {
		roFS := ro == itypes.OptionalBoolTrue
		s.ReadOnlyFilesystem = &roFS
	}
	// This should default to true for kubernetes yaml
	localTrue := true
	s.ReadWriteTmpfs = &localTrue

	// Make sure the container runs in a systemd unit which is
	// stored as a label at container creation.
	if unit := os.Getenv(systemdDefine.EnvVariable); unit != "" {
		s.Labels[systemdDefine.EnvVariable] = unit
	}

	// Set the stopTimeout if terminationGracePeriodSeconds is set in the kube yaml
	if opts.TerminationGracePeriodSeconds != nil {
		timeout := uint(*opts.TerminationGracePeriodSeconds)
		s.StopTimeout = &timeout
	}

	return s, nil
}

func parseMountPath(mountPath string, readOnly bool, propagationMode *v1.MountPropagationMode) (string, []string, error) {
	options := []string{}
	splitVol := strings.Split(mountPath, ":")
	if len(splitVol) > 2 {
		return "", options, fmt.Errorf("%q incorrect volume format, should be ctr-dir[:option]", mountPath)
	}
	dest := splitVol[0]
	if len(splitVol) > 1 {
		options = strings.Split(splitVol[1], ",")
	}
	if err := parse.ValidateVolumeCtrDir(dest); err != nil {
		return "", options, fmt.Errorf("parsing MountPath: %w", err)
	}
	if readOnly {
		options = append(options, "ro")
	}
	opts, err := parse.ValidateVolumeOpts(options)
	if err != nil {
		return "", opts, fmt.Errorf("parsing MountOptions: %w", err)
	}
	if propagationMode != nil {
		switch *propagationMode {
		case v1.MountPropagationNone:
			opts = append(opts, "private")
		case v1.MountPropagationHostToContainer:
			opts = append(opts, "rslave")
		case v1.MountPropagationBidirectional:
			opts = append(opts, "rshared")
		default:
			return "", opts, fmt.Errorf("unknown propagation mode %q", *propagationMode)
		}
	}
	return dest, opts, nil
}

func probeToHealthConfig(probe *v1.Probe, containerPorts []v1.ContainerPort) (*manifest.Schema2HealthConfig, error) {
	var commandString string
	failureCmd := "exit 1"
	probeHandler := probe.Handler
	host := "localhost" // Kubernetes default is host IP, but with Podman currently we run inside the container

	// configure healthcheck on the basis of Handler Actions.
	switch {
	case probeHandler.Exec != nil:
		// `makeHealthCheck` function can accept a json array as the command.
		cmd, err := json.Marshal(probeHandler.Exec.Command)
		if err != nil {
			return nil, err
		}
		commandString = string(cmd)
	case probeHandler.HTTPGet != nil:
		// set defaults as in https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#http-probes
		uriScheme := v1.URISchemeHTTP
		if probeHandler.HTTPGet.Scheme != "" {
			uriScheme = probeHandler.HTTPGet.Scheme
		}
		if probeHandler.HTTPGet.Host != "" {
			host = probeHandler.HTTPGet.Host
		}
		path := "/"
		if probeHandler.HTTPGet.Path != "" {
			path = probeHandler.HTTPGet.Path
		}
		portNum, err := getPortNumber(probeHandler.HTTPGet.Port, containerPorts)
		if err != nil {
			return nil, err
		}
		commandString = fmt.Sprintf("curl -f %s://%s:%d%s || %s", uriScheme, host, portNum, path, failureCmd)
	case probeHandler.TCPSocket != nil:
		portNum, err := getPortNumber(probeHandler.TCPSocket.Port, containerPorts)
		if err != nil {
			return nil, err
		}
		if probeHandler.TCPSocket.Host != "" {
			host = probeHandler.TCPSocket.Host
		}
		commandString = fmt.Sprintf("nc -z -v %s %d || %s", host, portNum, failureCmd)
	}
	return makeHealthCheck(commandString, probe.PeriodSeconds, probe.FailureThreshold, probe.TimeoutSeconds, probe.InitialDelaySeconds)
}

func getPortNumber(port intstr.IntOrString, containerPorts []v1.ContainerPort) (int, error) {
	var portNum int
	if port.Type == intstr.String && port.IntValue() == 0 {
		idx := slices.IndexFunc(containerPorts, func(cp v1.ContainerPort) bool { return cp.Name == port.String() })
		if idx == -1 {
			return 0, fmt.Errorf("unknown port: %s", port.String())
		}
		portNum = int(containerPorts[idx].ContainerPort)
	} else {
		portNum = port.IntValue()
	}
	return portNum, nil
}

func setupLivenessProbe(s *specgen.SpecGenerator, containerYAML v1.Container, restartPolicy string) error {
	var err error
	if containerYAML.LivenessProbe == nil {
		return nil
	}
	emptyHandler := v1.Handler{}
	if containerYAML.LivenessProbe.Handler != emptyHandler {
		s.HealthConfig, err = probeToHealthConfig(containerYAML.LivenessProbe, containerYAML.Ports)
		if err != nil {
			return err
		}
		// if restart policy is in place, ensure the health check enforces it
		if restartPolicy == "always" || restartPolicy == "onfailure" {
			var hcAct define.HealthCheckOnFailureAction = define.HealthCheckOnFailureActionRestart
			s.HealthCheckOnFailureAction = &hcAct
		}
		return nil
	}
	return nil
}

func setupStartupProbe(s *specgen.SpecGenerator, containerYAML v1.Container, restartPolicy string) error {
	if containerYAML.StartupProbe == nil {
		return nil
	}
	emptyHandler := v1.Handler{}
	if containerYAML.StartupProbe.Handler != emptyHandler {
		healthConfig, err := probeToHealthConfig(containerYAML.StartupProbe, containerYAML.Ports)
		if err != nil {
			return err
		}

		// currently, StartupProbe still an optional feature, and it requires HealthConfig.
		if s.HealthConfig == nil {
			probe := containerYAML.StartupProbe
			s.HealthConfig, err = makeHealthCheck("exit 0", probe.PeriodSeconds, probe.FailureThreshold, probe.TimeoutSeconds, probe.InitialDelaySeconds)
			if err != nil {
				return err
			}
		}
		s.StartupHealthConfig = &define.StartupHealthCheck{
			Schema2HealthConfig: *healthConfig,
			Successes:           int(containerYAML.StartupProbe.SuccessThreshold),
		}
		// if restart policy is in place, ensure the health check enforces it
		if restartPolicy == "always" || restartPolicy == "onfailure" {
			var hcAct define.HealthCheckOnFailureAction = define.HealthCheckOnFailureActionRestart
			s.HealthCheckOnFailureAction = &hcAct
		}
		return nil
	}
	return nil
}

func makeHealthCheck(inCmd string, interval int32, retries int32, timeout int32, startPeriod int32) (*manifest.Schema2HealthConfig, error) {
	// Every healthcheck requires a command
	if len(inCmd) == 0 {
		return nil, errors.New("must define a healthcheck command for all healthchecks")
	}

	// first try to parse option value as JSON array of strings...
	cmd := []string{}

	if inCmd == "none" {
		cmd = []string{define.HealthConfigTestNone}
	} else {
		err := json.Unmarshal([]byte(inCmd), &cmd)
		if err != nil {
			// ...otherwise pass it to "/bin/sh -c" inside the container
			cmd = []string{define.HealthConfigTestCmdShell}
			cmd = append(cmd, strings.Split(inCmd, " ")...)
		} else {
			cmd = append([]string{define.HealthConfigTestCmd}, cmd...)
		}
	}
	hc := manifest.Schema2HealthConfig{
		Test: cmd,
	}

	if interval < 1 {
		// kubernetes interval defaults to 10 sec and cannot be less than 1
		interval = 10
	}
	hc.Interval = time.Duration(interval) * time.Second
	if retries < 1 {
		// kubernetes retries defaults to 3
		retries = 3
	}
	hc.Retries = int(retries)
	if timeout < 1 {
		// kubernetes timeout defaults to 1
		timeout = 1
	}
	timeoutDuration := time.Duration(timeout) * time.Second
	if timeoutDuration < time.Duration(1) {
		return nil, errors.New("healthcheck-timeout must be at least 1 second")
	}
	hc.Timeout = timeoutDuration

	startPeriodDuration := time.Duration(startPeriod) * time.Second
	if startPeriodDuration < time.Duration(0) {
		return nil, errors.New("healthcheck-start-period must be 0 seconds or greater")
	}
	hc.StartPeriod = startPeriodDuration

	return &hc, nil
}

func setupSecurityContext(s *specgen.SpecGenerator, securityContext *v1.SecurityContext, podSecurityContext *v1.PodSecurityContext) {
	if securityContext == nil {
		securityContext = &v1.SecurityContext{}
	}
	if podSecurityContext == nil {
		podSecurityContext = &v1.PodSecurityContext{}
	}

	s.ReadOnlyFilesystem = securityContext.ReadOnlyRootFilesystem
	s.Privileged = securityContext.Privileged

	if securityContext.AllowPrivilegeEscalation != nil {
		localNNP := !*securityContext.AllowPrivilegeEscalation
		s.NoNewPrivileges = &localNNP
	}

	if securityContext.ProcMount != nil && *securityContext.ProcMount == v1.UnmaskedProcMount {
		s.ContainerSecurityConfig.Unmask = append(s.ContainerSecurityConfig.Unmask, []string{"ALL"}...)
	}

	seopt := securityContext.SELinuxOptions
	if seopt == nil {
		seopt = podSecurityContext.SELinuxOptions
	}
	if seopt != nil {
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
		if seopt.FileType != "" {
			s.SelinuxOpts = append(s.SelinuxOpts, fmt.Sprintf("filetype:%s", seopt.FileType))
		}
	}
	if caps := securityContext.Capabilities; caps != nil {
		for _, capability := range caps.Add {
			s.CapAdd = append(s.CapAdd, string(capability))
		}
		for _, capability := range caps.Drop {
			s.CapDrop = append(s.CapDrop, string(capability))
		}
	}
	runAsUser := securityContext.RunAsUser
	if runAsUser == nil {
		runAsUser = podSecurityContext.RunAsUser
	}
	if runAsUser != nil {
		localUser := strconv.FormatInt(*runAsUser, 10)
		s.User = &localUser
	}

	runAsGroup := securityContext.RunAsGroup
	if runAsGroup == nil {
		runAsGroup = podSecurityContext.RunAsGroup
	}
	if runAsGroup != nil {
		if s.User == nil {
			localZero := "0"
			s.User = &localZero
		}
		localUser := fmt.Sprintf("%s:%d", s.User, *runAsGroup)
		s.User = &localUser
	}
	for _, group := range podSecurityContext.SupplementalGroups {
		s.Groups = append(s.Groups, strconv.FormatInt(group, 10))
	}
}

func quantityToInt64(quantity *resource.Quantity) (int64, error) {
	if i, ok := quantity.AsInt64(); ok {
		return i, nil
	}

	if i, ok := quantity.AsDec().Unscaled(); ok {
		return i, nil
	}

	return 0, fmt.Errorf("quantity cannot be represented as int64: %v", quantity)
}

// read a k8s secret in JSON/YAML format from the secret manager
// k8s secret is stored as YAML, we have to read data as JSON for backward compatibility
func k8sSecretFromSecretManager(name string, secretsManager *secrets.SecretsManager) (map[string][]byte, error) {
	_, inputSecret, err := secretsManager.LookupSecretData(name)
	if err != nil {
		return nil, err
	}

	var secrets map[string][]byte
	if err := json.Unmarshal(inputSecret, &secrets); err != nil {
		secrets = make(map[string][]byte)
		var secret v1.Secret
		if err := yaml.Unmarshal(inputSecret, &secret); err != nil {
			return nil, fmt.Errorf("secret %v is not valid JSON/YAML: %v", name, err)
		}

		for key, val := range secret.Data {
			secrets[key] = val
		}

		for key, val := range secret.StringData {
			secrets[key] = []byte(val)
		}
	}

	return secrets, nil
}

// envVarsFrom returns all key-value pairs as env vars from a configMap or secret that matches the envFrom setting of a container
func envVarsFrom(envFrom v1.EnvFromSource, opts *CtrSpecGenOptions) (map[string]string, error) {
	envs := map[string]string{}

	if envFrom.ConfigMapRef != nil {
		cmRef := envFrom.ConfigMapRef
		err := fmt.Errorf("configmap %v not found", cmRef.Name)

		for _, c := range opts.ConfigMaps {
			if cmRef.Name == c.Name {
				envs = c.Data
				err = nil
				break
			}
		}

		if err != nil && (cmRef.Optional == nil || !*cmRef.Optional) {
			return nil, err
		}
	}

	if envFrom.SecretRef != nil {
		secRef := envFrom.SecretRef
		secret, err := k8sSecretFromSecretManager(secRef.Name, opts.SecretsManager)
		if err == nil {
			for k, v := range secret {
				envs[k] = string(v)
			}
		} else if secRef.Optional == nil || !*secRef.Optional {
			return nil, err
		}
	}

	return envs, nil
}

// envVarValue returns the environment variable value configured within the container's env setting.
// It gets the value from a configMap or secret if specified, otherwise returns env.Value
func envVarValue(env v1.EnvVar, opts *CtrSpecGenOptions) (*string, error) {
	if env.ValueFrom != nil {
		if env.ValueFrom.ConfigMapKeyRef != nil {
			cmKeyRef := env.ValueFrom.ConfigMapKeyRef
			err := fmt.Errorf("cannot set env %v: configmap %v not found", env.Name, cmKeyRef.Name)

			for _, c := range opts.ConfigMaps {
				if cmKeyRef.Name == c.Name {
					if value, ok := c.Data[cmKeyRef.Key]; ok {
						return &value, nil
					}
					err = fmt.Errorf("cannot set env %v: key %s not found in configmap %v", env.Name, cmKeyRef.Key, cmKeyRef.Name)
					break
				}
			}
			if cmKeyRef.Optional == nil || !*cmKeyRef.Optional {
				return nil, err
			}
			return nil, nil
		}

		if env.ValueFrom.SecretKeyRef != nil {
			secKeyRef := env.ValueFrom.SecretKeyRef
			secret, err := k8sSecretFromSecretManager(secKeyRef.Name, opts.SecretsManager)
			if err == nil {
				if val, ok := secret[secKeyRef.Key]; ok {
					value := string(val)
					return &value, nil
				}
				err = fmt.Errorf("secret %v has not %v key", secKeyRef.Name, secKeyRef.Key)
			}
			if secKeyRef.Optional == nil || !*secKeyRef.Optional {
				return nil, fmt.Errorf("cannot set env %v: %v", env.Name, err)
			}
			return nil, nil
		}

		if env.ValueFrom.FieldRef != nil {
			return envVarValueFieldRef(env, opts)
		}

		if env.ValueFrom.ResourceFieldRef != nil {
			return envVarValueResourceFieldRef(env, opts)
		}
	}

	return &env.Value, nil
}

func envVarValueFieldRef(env v1.EnvVar, opts *CtrSpecGenOptions) (*string, error) {
	fieldRef := env.ValueFrom.FieldRef

	fieldPathLabelPattern := `^metadata.labels\['(.+)'\]$`
	fieldPathLabelRegex := regexp.MustCompile(fieldPathLabelPattern)
	fieldPathAnnotationPattern := `^metadata.annotations\['(.+)'\]$`
	fieldPathAnnotationRegex := regexp.MustCompile(fieldPathAnnotationPattern)

	fieldPath := fieldRef.FieldPath

	if fieldPath == "metadata.name" {
		return &opts.PodName, nil
	}
	if fieldPath == "metadata.uid" {
		return &opts.PodID, nil
	}
	fieldPathMatches := fieldPathLabelRegex.FindStringSubmatch(fieldPath)
	if len(fieldPathMatches) == 2 { // 1 for entire regex and 1 for subexp
		labelValue := opts.Labels[fieldPathMatches[1]] // not existent label is OK
		return &labelValue, nil
	}
	fieldPathMatches = fieldPathAnnotationRegex.FindStringSubmatch(fieldPath)
	if len(fieldPathMatches) == 2 { // 1 for entire regex and 1 for subexp
		annotationValue := opts.Annotations[fieldPathMatches[1]] // not existent annotation is OK
		return &annotationValue, nil
	}

	return nil, fmt.Errorf(
		"can not set env %v. Reason: fieldPath %v is either not valid or not supported",
		env.Name, fieldPath,
	)
}

func envVarValueResourceFieldRef(env v1.EnvVar, opts *CtrSpecGenOptions) (*string, error) {
	divisor := env.ValueFrom.ResourceFieldRef.Divisor
	if divisor.IsZero() { // divisor not set, use default
		divisor.Set(1)
	}

	resources, err := getContainerResources(opts.Container)
	if err != nil {
		return nil, err
	}

	var value *resource.Quantity
	resourceName := env.ValueFrom.ResourceFieldRef.Resource
	var isValidDivisor bool

	switch resourceName {
	case "limits.memory":
		value = resources.Limits.Memory()
		isValidDivisor = isMemoryDivisor(divisor)
	case "limits.cpu":
		value = resources.Limits.Cpu()
		isValidDivisor = isCPUDivisor(divisor)
	case "requests.memory":
		value = resources.Requests.Memory()
		isValidDivisor = isMemoryDivisor(divisor)
	case "requests.cpu":
		value = resources.Requests.Cpu()
		isValidDivisor = isCPUDivisor(divisor)
	default:
		return nil, fmt.Errorf(
			"can not set env %v. Reason: resource %v is either not valid or not supported",
			env.Name, resourceName,
		)
	}

	if !isValidDivisor {
		return nil, fmt.Errorf(
			"can not set env %s. Reason: divisor value %s is not valid",
			env.Name, divisor.String(),
		)
	}

	// k8s rounds up the result to the nearest integer
	intValue := int64(math.Ceil(value.AsApproximateFloat64() / divisor.AsApproximateFloat64()))
	stringValue := strconv.FormatInt(intValue, 10)

	return &stringValue, nil
}

func isMemoryDivisor(divisor resource.Quantity) bool {
	switch divisor.String() {
	case "1", "1k", "1M", "1G", "1T", "1P", "1E", "1Ki", "1Mi", "1Gi", "1Ti", "1Pi", "1Ei":
		return true
	default:
		return false
	}
}

func isCPUDivisor(divisor resource.Quantity) bool {
	switch divisor.String() {
	case "1", "1m":
		return true
	default:
		return false
	}
}

func getContainerResources(container v1.Container) (v1.ResourceRequirements, error) {
	result := v1.ResourceRequirements{
		Limits:   v1.ResourceList{},
		Requests: v1.ResourceList{},
	}

	limits := container.Resources.Limits
	requests := container.Resources.Requests

	if limits == nil || limits.Memory().IsZero() {
		mi, err := meminfo.Read()
		if err != nil {
			return result, err
		}
		result.Limits[v1.ResourceMemory] = *resource.NewQuantity(mi.MemTotal, resource.DecimalSI)
	} else {
		result.Limits[v1.ResourceMemory] = limits[v1.ResourceMemory]
	}

	if limits == nil || limits.Cpu().IsZero() {
		result.Limits[v1.ResourceCPU] = *resource.NewQuantity(int64(runtime.NumCPU()), resource.DecimalSI)
	} else {
		result.Limits[v1.ResourceCPU] = limits[v1.ResourceCPU]
	}

	if requests == nil || requests.Memory().IsZero() {
		result.Requests[v1.ResourceMemory] = result.Limits[v1.ResourceMemory]
	} else {
		result.Requests[v1.ResourceMemory] = requests[v1.ResourceMemory]
	}

	if requests == nil || requests.Cpu().IsZero() {
		result.Requests[v1.ResourceCPU] = result.Limits[v1.ResourceCPU]
	} else {
		result.Requests[v1.ResourceCPU] = requests[v1.ResourceCPU]
	}

	return result, nil
}

// getPodPorts converts a slice of kube container descriptions to an
// array of portmapping
func getPodPorts(containers []v1.Container, publishAll bool) []types.PortMapping {
	var infraPorts []types.PortMapping
	for _, container := range containers {
		for _, p := range container.Ports {
			if p.HostPort != 0 && p.ContainerPort == 0 {
				p.ContainerPort = p.HostPort
			}
			if p.HostPort == 0 && p.ContainerPort != 0 && publishAll {
				p.HostPort = p.ContainerPort
			}
			if p.Protocol == "" {
				p.Protocol = "tcp"
			}
			portBinding := types.PortMapping{
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
