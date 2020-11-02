package libpod

import (
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/pkg/lookup"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenerateForKube takes a slice of libpod containers and generates
// one v1.Pod description that includes just a single container.
func (c *Container) GenerateForKube() (*v1.Pod, error) {
	// Generate the v1.Pod yaml description
	return simplePodWithV1Container(c)
}

// GenerateForKube takes a slice of libpod containers and generates
// one v1.Pod description
func (p *Pod) GenerateForKube() (*v1.Pod, []v1.ServicePort, error) {
	// Generate the v1.Pod yaml description
	var (
		ports        []v1.ContainerPort //nolint
		servicePorts []v1.ServicePort   //nolint
	)

	allContainers, err := p.allContainers()
	if err != nil {
		return nil, servicePorts, err
	}
	// If the pod has no containers, no sense to generate YAML
	if len(allContainers) == 0 {
		return nil, servicePorts, errors.Errorf("pod %s has no containers", p.ID())
	}
	// If only an infra container is present, makes no sense to generate YAML
	if len(allContainers) == 1 && p.HasInfraContainer() {
		return nil, servicePorts, errors.Errorf("pod %s only has an infra container", p.ID())
	}

	extraHost := make([]v1.HostAlias, 0)
	if p.HasInfraContainer() {
		infraContainer, err := p.getInfraContainer()
		if err != nil {
			return nil, servicePorts, err
		}
		for _, host := range infraContainer.config.ContainerNetworkConfig.HostAdd {
			hostSli := strings.SplitN(host, ":", 2)
			if len(hostSli) != 2 {
				return nil, servicePorts, errors.New("invalid hostAdd")
			}
			extraHost = append(extraHost, v1.HostAlias{
				IP:        hostSli[1],
				Hostnames: []string{hostSli[0]},
			})
		}
		ports, err = ocicniPortMappingToContainerPort(infraContainer.config.PortMappings)
		if err != nil {
			return nil, servicePorts, err
		}
		servicePorts = containerPortsToServicePorts(ports)

	}
	pod, err := p.podWithContainers(allContainers, ports)
	if err != nil {
		return nil, servicePorts, err
	}
	pod.Spec.HostAliases = extraHost

	// vendor/k8s.io/api/core/v1/types.go: v1.Container cannot save restartPolicy
	// so set it at here
	for _, ctr := range allContainers {
		if !ctr.IsInfra() {
			switch ctr.Config().RestartPolicy {
			case RestartPolicyAlways:
				pod.Spec.RestartPolicy = v1.RestartPolicyAlways
			case RestartPolicyOnFailure:
				pod.Spec.RestartPolicy = v1.RestartPolicyOnFailure
			case RestartPolicyNo:
				pod.Spec.RestartPolicy = v1.RestartPolicyNever
			default: // some pod create from cmdline, such as "", so set it to Never
				pod.Spec.RestartPolicy = v1.RestartPolicyNever
			}
			break
		}
	}

	if p.SharesPID() {
		// unfortunately, go doesn't have a nice way to specify a pointer to a bool
		b := true
		pod.Spec.ShareProcessNamespace = &b
	}

	return pod, servicePorts, nil
}

func (p *Pod) getInfraContainer() (*Container, error) {
	infraID, err := p.InfraContainerID()
	if err != nil {
		return nil, err
	}
	return p.runtime.GetContainer(infraID)
}

// GenerateKubeServiceFromV1Pod creates a v1 service object from a v1 pod object
func GenerateKubeServiceFromV1Pod(pod *v1.Pod, servicePorts []v1.ServicePort) v1.Service {
	service := v1.Service{}
	selector := make(map[string]string)
	selector["app"] = pod.Labels["app"]
	ports := servicePorts
	if len(ports) == 0 {
		ports = containersToServicePorts(pod.Spec.Containers)
	}
	serviceSpec := v1.ServiceSpec{
		Ports:    ports,
		Selector: selector,
		Type:     v1.ServiceTypeNodePort,
	}
	service.Spec = serviceSpec
	service.ObjectMeta = pod.ObjectMeta
	tm := v12.TypeMeta{
		Kind:       "Service",
		APIVersion: pod.TypeMeta.APIVersion,
	}
	service.TypeMeta = tm
	return service
}

// containerPortsToServicePorts takes a slice of containerports and generates a
// slice of service ports
func containerPortsToServicePorts(containerPorts []v1.ContainerPort) []v1.ServicePort {
	sps := make([]v1.ServicePort, 0, len(containerPorts))
	for _, cp := range containerPorts {
		nodePort := 30000 + rand.Intn(32767-30000+1)
		servicePort := v1.ServicePort{
			Protocol: cp.Protocol,
			Port:     cp.ContainerPort,
			NodePort: int32(nodePort),
			Name:     strconv.Itoa(int(cp.ContainerPort)),
		}
		sps = append(sps, servicePort)
	}
	return sps
}

// containersToServicePorts takes a slice of v1.Containers and generates an
// inclusive list of serviceports to expose
func containersToServicePorts(containers []v1.Container) []v1.ServicePort {
	// Without the call to rand.Seed, a program will produce the same sequence of pseudo-random numbers
	// for each execution. Legal nodeport range is 30000-32767
	rand.Seed(time.Now().UnixNano())

	sps := make([]v1.ServicePort, 0, len(containers))
	for _, ctr := range containers {
		sps = append(sps, containerPortsToServicePorts(ctr.Ports)...)
	}
	return sps
}

func (p *Pod) podWithContainers(containers []*Container, ports []v1.ContainerPort) (*v1.Pod, error) {
	deDupPodVolumes := make(map[string]*v1.Volume)
	first := true
	podContainers := make([]v1.Container, 0, len(containers))
	for _, ctr := range containers {
		if !ctr.IsInfra() {
			ctr, volumes, err := containerToV1Container(ctr)
			if err != nil {
				return nil, err
			}

			// Since port bindings for the pod are handled by the
			// infra container, wipe them here.
			ctr.Ports = nil

			// We add the original port declarations from the libpod infra container
			// to the first kubernetes container description because otherwise we loose
			// the original container/port bindings.
			if first && len(ports) > 0 {
				ctr.Ports = ports
				first = false
			}
			podContainers = append(podContainers, ctr)
			// Deduplicate volumes, so if containers in the pod share a volume, it's only
			// listed in the volumes section once
			for _, vol := range volumes {
				vol := vol
				deDupPodVolumes[vol.Name] = &vol
			}
		}
	}
	podVolumes := make([]v1.Volume, 0, len(deDupPodVolumes))
	for _, vol := range deDupPodVolumes {
		podVolumes = append(podVolumes, *vol)
	}

	return addContainersAndVolumesToPodObject(podContainers, podVolumes, p.Name()), nil
}

func addContainersAndVolumesToPodObject(containers []v1.Container, volumes []v1.Volume, podName string) *v1.Pod {
	tm := v12.TypeMeta{
		Kind:       "Pod",
		APIVersion: "v1",
	}

	// Add a label called "app" with the containers name as a value
	labels := make(map[string]string)
	labels["app"] = removeUnderscores(podName)
	om := v12.ObjectMeta{
		// The name of the pod is container_name-libpod
		Name:   podName,
		Labels: labels,
		// CreationTimestamp seems to be required, so adding it; in doing so, the timestamp
		// will reflect time this is run (not container create time) because the conversion
		// of the container create time to v1 Time is probably not warranted nor worthwhile.
		CreationTimestamp: v12.Now(),
	}
	ps := v1.PodSpec{
		Containers: containers,
		Volumes:    volumes,
	}
	p := v1.Pod{
		TypeMeta:   tm,
		ObjectMeta: om,
		Spec:       ps,
	}
	return &p
}

// simplePodWithV1Container is a function used by inspect when kube yaml needs to be generated
// for a single container.  we "insert" that container description in a pod.
func simplePodWithV1Container(ctr *Container) (*v1.Pod, error) {
	kubeCtr, kubeVols, err := containerToV1Container(ctr)
	if err != nil {
		return nil, err
	}
	return addContainersAndVolumesToPodObject([]v1.Container{kubeCtr}, kubeVols, ctr.Name()), nil

}

// containerToV1Container converts information we know about a libpod container
// to a V1.Container specification.
func containerToV1Container(c *Container) (v1.Container, []v1.Volume, error) {
	kubeContainer := v1.Container{}
	kubeVolumes := []v1.Volume{}
	kubeSec, err := generateKubeSecurityContext(c)
	if err != nil {
		return kubeContainer, kubeVolumes, err
	}

	if len(c.config.Spec.Linux.Devices) > 0 {
		// TODO Enable when we can support devices and their names
		kubeContainer.VolumeDevices = generateKubeVolumeDeviceFromLinuxDevice(c.Spec().Linux.Devices)
		return kubeContainer, kubeVolumes, errors.Wrapf(define.ErrNotImplemented, "linux devices")
	}

	if len(c.config.UserVolumes) > 0 {
		// TODO When we until we can resolve what the volume name should be, this is disabled
		// Volume names need to be coordinated "globally" in the kube files.
		volumeMounts, volumes, err := libpodMountsToKubeVolumeMounts(c)
		if err != nil {
			return kubeContainer, kubeVolumes, err
		}
		kubeContainer.VolumeMounts = volumeMounts
		kubeVolumes = append(kubeVolumes, volumes...)
	}

	envVariables, err := libpodEnvVarsToKubeEnvVars(c.config.Spec.Process.Env)
	if err != nil {
		return kubeContainer, kubeVolumes, err
	}

	portmappings, err := c.PortMappings()
	if err != nil {
		return kubeContainer, kubeVolumes, err
	}
	ports, err := ocicniPortMappingToContainerPort(portmappings)
	if err != nil {
		return kubeContainer, kubeVolumes, err
	}

	containerCommands := c.Command()
	kubeContainer.Name = removeUnderscores(c.Name())

	_, image := c.Image()
	kubeContainer.Image = image
	kubeContainer.Stdin = c.Stdin()
	kubeContainer.Command = containerCommands
	// TODO need to figure out how we handle command vs entry point.  Kube appears to prefer entrypoint.
	// right now we just take the container's command
	//container.Args = args
	kubeContainer.WorkingDir = c.WorkingDir()
	kubeContainer.Ports = ports
	// This should not be applicable
	//container.EnvFromSource =
	kubeContainer.Env = envVariables
	kubeContainer.SecurityContext = kubeSec
	kubeContainer.StdinOnce = false
	kubeContainer.TTY = c.config.Spec.Process.Terminal

	if c.config.Spec.Linux != nil &&
		c.config.Spec.Linux.Resources != nil {
		if c.config.Spec.Linux.Resources.Memory != nil &&
			c.config.Spec.Linux.Resources.Memory.Limit != nil {
			if kubeContainer.Resources.Limits == nil {
				kubeContainer.Resources.Limits = v1.ResourceList{}
			}

			qty := kubeContainer.Resources.Limits.Memory()
			qty.Set(*c.config.Spec.Linux.Resources.Memory.Limit)
			kubeContainer.Resources.Limits[v1.ResourceMemory] = *qty
		}

		if c.config.Spec.Linux.Resources.CPU != nil &&
			c.config.Spec.Linux.Resources.CPU.Quota != nil &&
			c.config.Spec.Linux.Resources.CPU.Period != nil {
			quota := *c.config.Spec.Linux.Resources.CPU.Quota
			period := *c.config.Spec.Linux.Resources.CPU.Period

			if quota > 0 && period > 0 {
				cpuLimitMilli := int64(1000 * util.PeriodAndQuotaToCores(period, quota))

				// Kubernetes: precision finer than 1m is not allowed
				if cpuLimitMilli >= 1 {
					if kubeContainer.Resources.Limits == nil {
						kubeContainer.Resources.Limits = v1.ResourceList{}
					}

					qty := kubeContainer.Resources.Limits.Cpu()
					qty.SetMilli(cpuLimitMilli)
					kubeContainer.Resources.Limits[v1.ResourceCPU] = *qty
				}
			}
		}
	}

	return kubeContainer, kubeVolumes, nil
}

// ocicniPortMappingToContainerPort takes an ocicni portmapping and converts
// it to a v1.ContainerPort format for kube output
func ocicniPortMappingToContainerPort(portMappings []ocicni.PortMapping) ([]v1.ContainerPort, error) {
	containerPorts := make([]v1.ContainerPort, 0, len(portMappings))
	for _, p := range portMappings {
		var protocol v1.Protocol
		switch strings.ToUpper(p.Protocol) {
		case "TCP":
			protocol = v1.ProtocolTCP
		case "UDP":
			protocol = v1.ProtocolUDP
		default:
			return containerPorts, errors.Errorf("unknown network protocol %s", p.Protocol)
		}
		cp := v1.ContainerPort{
			// Name will not be supported
			HostPort:      p.HostPort,
			HostIP:        p.HostIP,
			ContainerPort: p.ContainerPort,
			Protocol:      protocol,
		}
		containerPorts = append(containerPorts, cp)
	}
	return containerPorts, nil
}

// libpodEnvVarsToKubeEnvVars converts a key=value string slice to []v1.EnvVar
func libpodEnvVarsToKubeEnvVars(envs []string) ([]v1.EnvVar, error) {
	envVars := make([]v1.EnvVar, 0, len(envs))
	for _, e := range envs {
		split := strings.SplitN(e, "=", 2)
		if len(split) != 2 {
			return envVars, errors.Errorf("environment variable %s is malformed; should be key=value", e)
		}
		ev := v1.EnvVar{
			Name:  split[0],
			Value: split[1],
		}
		envVars = append(envVars, ev)
	}
	return envVars, nil
}

// libpodMountsToKubeVolumeMounts converts the containers mounts to a struct kube understands
func libpodMountsToKubeVolumeMounts(c *Container) ([]v1.VolumeMount, []v1.Volume, error) {
	// TjDO when named volumes are supported in play kube, also parse named volumes here
	_, mounts := c.sortUserVolumes(c.config.Spec)
	vms := make([]v1.VolumeMount, 0, len(mounts))
	vos := make([]v1.Volume, 0, len(mounts))
	for _, m := range mounts {
		vm, vo, err := generateKubeVolumeMount(m)
		if err != nil {
			return vms, vos, err
		}
		vms = append(vms, vm)
		vos = append(vos, vo)
	}
	return vms, vos, nil
}

// generateKubeVolumeMount takes a user specified mount and returns
// a kubernetes VolumeMount (to be added to the container) and a kubernetes Volume
// (to be added to the pod)
func generateKubeVolumeMount(m specs.Mount) (v1.VolumeMount, v1.Volume, error) {
	vm := v1.VolumeMount{}
	vo := v1.Volume{}

	name, err := convertVolumePathToName(m.Source)
	if err != nil {
		return vm, vo, err
	}
	vm.Name = name
	vm.MountPath = m.Destination
	if util.StringInSlice("ro", m.Options) {
		vm.ReadOnly = true
	}

	vo.Name = name
	vo.HostPath = &v1.HostPathVolumeSource{}
	vo.HostPath.Path = m.Source
	isDir, err := isHostPathDirectory(m.Source)
	// neither a directory or a file lives here, default to creating a directory
	// TODO should this be an error instead?
	var hostPathType v1.HostPathType
	switch {
	case err != nil:
		hostPathType = v1.HostPathDirectoryOrCreate
	case isDir:
		hostPathType = v1.HostPathDirectory
	default:
		hostPathType = v1.HostPathFile
	}
	vo.HostPath.Type = &hostPathType

	return vm, vo, nil
}

func isHostPathDirectory(hostPathSource string) (bool, error) {
	info, err := os.Stat(hostPathSource)
	if err != nil {
		return false, err
	}
	return info.Mode().IsDir(), nil
}

func convertVolumePathToName(hostSourcePath string) (string, error) {
	if len(hostSourcePath) == 0 {
		return "", errors.Errorf("hostSourcePath must be specified to generate volume name")
	}
	if len(hostSourcePath) == 1 {
		if hostSourcePath != "/" {
			return "", errors.Errorf("hostSourcePath malformatted: %s", hostSourcePath)
		}
		// add special case name
		return "root", nil
	}
	// First, trim trailing slashes, then replace slashes with dashes.
	// Thus, /mnt/data/ will become mnt-data
	return strings.Replace(strings.Trim(hostSourcePath, "/"), "/", "-", -1), nil
}

func determineCapAddDropFromCapabilities(defaultCaps, containerCaps []string) *v1.Capabilities {
	var (
		drop = []v1.Capability{}
		add  = []v1.Capability{}
	)
	dedupDrop := make(map[string]bool)
	dedupAdd := make(map[string]bool)
	// Find caps in the defaultCaps but not in the container's
	// those indicate a dropped cap
	for _, capability := range defaultCaps {
		if !util.StringInSlice(capability, containerCaps) {
			if _, ok := dedupDrop[capability]; !ok {
				drop = append(drop, v1.Capability(capability))
				dedupDrop[capability] = true
			}
		}
	}
	// Find caps in the container but not in the defaults; those indicate
	// an added cap
	for _, capability := range containerCaps {
		if !util.StringInSlice(capability, defaultCaps) {
			if _, ok := dedupAdd[capability]; !ok {
				add = append(add, v1.Capability(capability))
				dedupAdd[capability] = true
			}
		}
	}

	return &v1.Capabilities{
		Add:  add,
		Drop: drop,
	}
}

func capAddDrop(caps *specs.LinuxCapabilities) (*v1.Capabilities, error) {
	g, err := generate.New("linux")
	if err != nil {
		return nil, err
	}
	// Combine all the default capabilities into a slice
	defaultCaps := append(g.Config.Process.Capabilities.Ambient, g.Config.Process.Capabilities.Bounding...)
	defaultCaps = append(defaultCaps, g.Config.Process.Capabilities.Effective...)
	defaultCaps = append(defaultCaps, g.Config.Process.Capabilities.Inheritable...)
	defaultCaps = append(defaultCaps, g.Config.Process.Capabilities.Permitted...)

	// Combine all the container's capabilities into a slic
	containerCaps := append(caps.Ambient, caps.Bounding...)
	containerCaps = append(containerCaps, caps.Effective...)
	containerCaps = append(containerCaps, caps.Inheritable...)
	containerCaps = append(containerCaps, caps.Permitted...)

	calculatedCaps := determineCapAddDropFromCapabilities(defaultCaps, containerCaps)
	return calculatedCaps, nil
}

// generateKubeSecurityContext generates a securityContext based on the existing container
func generateKubeSecurityContext(c *Container) (*v1.SecurityContext, error) {
	priv := c.Privileged()
	ro := c.IsReadOnly()
	allowPrivEscalation := !c.config.Spec.Process.NoNewPrivileges

	newCaps, err := capAddDrop(c.config.Spec.Process.Capabilities)
	if err != nil {
		return nil, err
	}

	var selinuxOpts v1.SELinuxOptions
	opts := strings.SplitN(c.config.Spec.Annotations[define.InspectAnnotationLabel], ":", 2)
	if len(opts) == 2 {
		switch opts[0] {
		case "type":
			selinuxOpts.Type = opts[1]
		case "level":
			selinuxOpts.Level = opts[1]
		}
	}
	if len(opts) == 1 {
		if opts[0] == "disable" {
			selinuxOpts.Type = "spc_t"
		}
	}

	sc := v1.SecurityContext{
		Capabilities:   newCaps,
		Privileged:     &priv,
		SELinuxOptions: &selinuxOpts,
		// RunAsNonRoot is an optional parameter; our first implementations should be root only; however
		// I'm leaving this as a bread-crumb for later
		//RunAsNonRoot:             &nonRoot,
		ReadOnlyRootFilesystem:   &ro,
		AllowPrivilegeEscalation: &allowPrivEscalation,
	}

	if c.User() != "" {
		if !c.batched {
			c.lock.Lock()
			defer c.lock.Unlock()
		}
		if err := c.syncContainer(); err != nil {
			return nil, errors.Wrapf(err, "unable to sync container during YAML generation")
		}

		logrus.Debugf("Looking in container for user: %s", c.User())
		execUser, err := lookup.GetUserGroupInfo(c.state.Mountpoint, c.User(), nil)
		if err != nil {
			return nil, err
		}
		uid := int64(execUser.Uid)
		gid := int64(execUser.Gid)
		sc.RunAsUser = &uid
		sc.RunAsGroup = &gid
	}
	return &sc, nil
}

// generateKubeVolumeDeviceFromLinuxDevice takes a list of devices and makes a VolumeDevice struct for kube
func generateKubeVolumeDeviceFromLinuxDevice(devices []specs.LinuxDevice) []v1.VolumeDevice {
	volumeDevices := make([]v1.VolumeDevice, 0, len(devices))
	for _, d := range devices {
		vd := v1.VolumeDevice{
			// TBD How are we going to sync up these names
			//Name:
			DevicePath: d.Path,
		}
		volumeDevices = append(volumeDevices, vd)
	}
	return volumeDevices
}

func removeUnderscores(s string) string {
	return strings.Replace(s, "_", "", -1)
}
