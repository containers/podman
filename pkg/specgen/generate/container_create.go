package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	cdi "github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	"github.com/containers/common/libimage"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/namespaces"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/util"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
)

// MakeContainer creates a container based on the SpecGenerator.
// Returns the created, container and any warnings resulting from creating the
// container, or an error.
func MakeContainer(ctx context.Context, rt *libpod.Runtime, s *specgen.SpecGenerator, clone bool, c *libpod.Container) (*spec.Spec, *specgen.SpecGenerator, []libpod.CtrCreateOption, error) {
	rtc, err := rt.GetConfigNoCopy()
	if err != nil {
		return nil, nil, nil, err
	}

	// If joining a pod, retrieve the pod for use, and its infra container
	var pod *libpod.Pod
	var infra *libpod.Container
	if s.Pod != "" {
		pod, err = rt.LookupPod(s.Pod)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error retrieving pod %s: %w", s.Pod, err)
		}
		if pod.HasInfraContainer() {
			infra, err = pod.InfraContainer()
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}

	options := []libpod.CtrCreateOption{}
	compatibleOptions := &libpod.InfraInherit{}
	var infraSpec *spec.Spec
	if infra != nil {
		options, infraSpec, compatibleOptions, err = Inherit(*infra, s, rt)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	if err := FinishThrottleDevices(s); err != nil {
		return nil, nil, nil, err
	}
	// Set defaults for unset namespaces
	if s.PidNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("pid", rtc, pod)
		if err != nil {
			return nil, nil, nil, err
		}
		s.PidNS = defaultNS
	}
	if s.IpcNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("ipc", rtc, pod)
		if err != nil {
			return nil, nil, nil, err
		}
		s.IpcNS = defaultNS
	}
	if s.UtsNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("uts", rtc, pod)
		if err != nil {
			return nil, nil, nil, err
		}
		s.UtsNS = defaultNS
	}
	if s.UserNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("user", rtc, pod)
		if err != nil {
			return nil, nil, nil, err
		}
		s.UserNS = defaultNS

		mappings, err := util.ParseIDMapping(namespaces.UsernsMode(s.UserNS.NSMode), nil, nil, "", "")
		if err != nil {
			return nil, nil, nil, err
		}
		s.IDMappings = mappings
	}
	if s.NetNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("net", rtc, pod)
		if err != nil {
			return nil, nil, nil, err
		}
		s.NetNS = defaultNS
	}
	if s.CgroupNS.IsDefault() {
		defaultNS, err := GetDefaultNamespaceMode("cgroup", rtc, pod)
		if err != nil {
			return nil, nil, nil, err
		}
		s.CgroupNS = defaultNS
	}

	if s.ContainerCreateCommand != nil {
		options = append(options, libpod.WithCreateCommand(s.ContainerCreateCommand))
	}

	if s.Rootfs != "" {
		options = append(options, libpod.WithRootFS(s.Rootfs, s.RootfsOverlay))
	}

	newImage, resolvedImageName, imageData, err := getImageFromSpec(ctx, rt, s)
	if err != nil {
		return nil, nil, nil, err
	}
	if newImage != nil {
		// If the input name changed, we could properly resolve the
		// image. Otherwise, it must have been an ID where we're
		// defaulting to the first name or an empty one if no names are
		// present.
		if strings.HasPrefix(newImage.ID(), resolvedImageName) {
			names := newImage.Names()
			if len(names) > 0 {
				resolvedImageName = names[0]
			}
		}

		options = append(options, libpod.WithRootFSFromImage(newImage.ID(), resolvedImageName, s.RawImageName))
	}

	_, err = rt.LookupPod(s.Hostname)
	if len(s.Hostname) > 0 && !s.UtsNS.IsPrivate() && err == nil {
		// ok, we are incorrectly setting the pod as the hostname, lets undo that before validation
		s.Hostname = ""
	}
	if err := s.Validate(); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid config provided: %w", err)
	}

	finalMounts, finalVolumes, finalOverlays, err := finalizeMounts(ctx, s, rt, rtc, newImage)
	if err != nil {
		return nil, nil, nil, err
	}

	if len(s.HostUsers) > 0 {
		options = append(options, libpod.WithHostUsers(s.HostUsers))
	}

	command, err := makeCommand(s, imageData, rtc)
	if err != nil {
		return nil, nil, nil, err
	}

	infraVol := (len(compatibleOptions.Mounts) > 0 || len(compatibleOptions.Volumes) > 0 || len(compatibleOptions.ImageVolumes) > 0 || len(compatibleOptions.OverlayVolumes) > 0)
	opts, err := createContainerOptions(rt, s, pod, finalVolumes, finalOverlays, imageData, command, infraVol, *compatibleOptions)
	if err != nil {
		return nil, nil, nil, err
	}
	options = append(options, opts...)

	if containerType := s.InitContainerType; len(containerType) > 0 {
		options = append(options, libpod.WithInitCtrType(containerType))
	}
	if len(s.Name) > 0 {
		logrus.Debugf("setting container name %s", s.Name)
		options = append(options, libpod.WithName(s.Name))
	}
	if len(s.Devices) > 0 {
		opts = ExtractCDIDevices(s)
		options = append(options, opts...)
	}
	runtimeSpec, err := SpecGenToOCI(ctx, s, rt, rtc, newImage, finalMounts, pod, command, compatibleOptions)
	if clone { // the container fails to start if cloned due to missing Linux spec entries
		if c == nil {
			return nil, nil, nil, errors.New("the given container could not be retrieved")
		}
		conf := c.Config()
		if conf != nil && conf.Spec != nil && conf.Spec.Linux != nil {
			out, err := json.Marshal(conf.Spec.Linux)
			if err != nil {
				return nil, nil, nil, err
			}
			resources := runtimeSpec.Linux.Resources

			// resources get overwrritten similarly to pod inheritance, manually assign here if there is a new value
			marshalRes, err := json.Marshal(resources)
			if err != nil {
				return nil, nil, nil, err
			}

			err = json.Unmarshal(out, runtimeSpec.Linux)
			if err != nil {
				return nil, nil, nil, err
			}

			err = json.Unmarshal(marshalRes, runtimeSpec.Linux.Resources)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		if s.ResourceLimits != nil {
			switch {
			case s.ResourceLimits.CPU != nil:
				runtimeSpec.Linux.Resources.CPU = s.ResourceLimits.CPU
			case s.ResourceLimits.Memory != nil:
				runtimeSpec.Linux.Resources.Memory = s.ResourceLimits.Memory
			case s.ResourceLimits.BlockIO != nil:
				runtimeSpec.Linux.Resources.BlockIO = s.ResourceLimits.BlockIO
			case s.ResourceLimits.Devices != nil:
				runtimeSpec.Linux.Resources.Devices = s.ResourceLimits.Devices
			}
		}
	}
	if len(s.HostDeviceList) > 0 {
		options = append(options, libpod.WithHostDevice(s.HostDeviceList))
	}
	if infraSpec != nil && infraSpec.Linux != nil { // if we are inheriting Linux info from a pod...
		// Pass Security annotations
		if len(infraSpec.Annotations[define.InspectAnnotationLabel]) > 0 && len(runtimeSpec.Annotations[define.InspectAnnotationLabel]) == 0 {
			runtimeSpec.Annotations[define.InspectAnnotationLabel] = infraSpec.Annotations[define.InspectAnnotationLabel]
		}
		if len(infraSpec.Annotations[define.InspectAnnotationSeccomp]) > 0 && len(runtimeSpec.Annotations[define.InspectAnnotationSeccomp]) == 0 {
			runtimeSpec.Annotations[define.InspectAnnotationSeccomp] = infraSpec.Annotations[define.InspectAnnotationSeccomp]
		}
		if len(infraSpec.Annotations[define.InspectAnnotationApparmor]) > 0 && len(runtimeSpec.Annotations[define.InspectAnnotationApparmor]) == 0 {
			runtimeSpec.Annotations[define.InspectAnnotationApparmor] = infraSpec.Annotations[define.InspectAnnotationApparmor]
		}
	}
	return runtimeSpec, s, options, err
}
func ExecuteCreate(ctx context.Context, rt *libpod.Runtime, runtimeSpec *spec.Spec, s *specgen.SpecGenerator, infra bool, options ...libpod.CtrCreateOption) (*libpod.Container, error) {
	ctr, err := rt.NewContainer(ctx, runtimeSpec, s, infra, options...)
	if err != nil {
		return ctr, err
	}

	return ctr, rt.PrepareVolumeOnCreateContainer(ctx, ctr)
}

// ExtractCDIDevices process the list of Devices in the spec and determines if any of these are CDI devices.
// The CDI devices are added to the list of CtrCreateOptions.
// Note that this may modify the device list associated with the spec, which should then only contain non-CDI devices.
func ExtractCDIDevices(s *specgen.SpecGenerator) []libpod.CtrCreateOption {
	devs := make([]spec.LinuxDevice, 0, len(s.Devices))
	var cdiDevs []string
	var options []libpod.CtrCreateOption

	for _, device := range s.Devices {
		if isCDIDevice(device.Path) {
			logrus.Debugf("Identified CDI device %v", device.Path)
			cdiDevs = append(cdiDevs, device.Path)
			continue
		}
		logrus.Debugf("Non-CDI device %v; assuming standard device", device.Path)
		devs = append(devs, device)
	}
	s.Devices = devs
	if len(cdiDevs) > 0 {
		options = append(options, libpod.WithCDI(cdiDevs))
	}
	return options
}

// isCDIDevice checks whether the specified device is a CDI device.
func isCDIDevice(device string) bool {
	return cdi.IsQualifiedName(device)
}

func createContainerOptions(rt *libpod.Runtime, s *specgen.SpecGenerator, pod *libpod.Pod, volumes []*specgen.NamedVolume, overlays []*specgen.OverlayVolume, imageData *libimage.ImageData, command []string, infraVolumes bool, compatibleOptions libpod.InfraInherit) ([]libpod.CtrCreateOption, error) {
	var options []libpod.CtrCreateOption
	var err error

	if s.PreserveFDs > 0 {
		options = append(options, libpod.WithPreserveFDs(s.PreserveFDs))
	}

	if s.Stdin {
		options = append(options, libpod.WithStdin())
	}

	if s.Timezone != "" {
		options = append(options, libpod.WithTimezone(s.Timezone))
	}
	if s.Umask != "" {
		options = append(options, libpod.WithUmask(s.Umask))
	}
	if s.Volatile {
		options = append(options, libpod.WithVolatile())
	}
	if s.PasswdEntry != "" {
		options = append(options, libpod.WithPasswdEntry(s.PasswdEntry))
	}

	if s.Privileged {
		options = append(options, libpod.WithMountAllDevices())
	}

	useSystemd := false
	switch s.Systemd {
	case "always":
		useSystemd = true
	case "false":
		break
	case "", "true":
		if len(command) == 0 && imageData != nil {
			command = imageData.Config.Cmd
		}

		if len(command) > 0 {
			useSystemdCommands := map[string]bool{
				"/sbin/init":           true,
				"/usr/sbin/init":       true,
				"/usr/local/sbin/init": true,
			}
			// Grab last command in case this is launched from a shell
			cmd := command
			if len(command) > 2 {
				// Podman build will add "/bin/sh" "-c" to
				// Entrypoint. Remove and search for systemd
				if command[0] == "/bin/sh" && command[1] == "-c" {
					cmd = command[2:]
				}
			}
			if useSystemdCommands[cmd[0]] || (filepath.Base(cmd[0]) == "systemd") {
				useSystemd = true
			}
		}
	default:
		return nil, fmt.Errorf("invalid value %q systemd option requires 'true, false, always': %w", s.Systemd, err)
	}
	logrus.Debugf("using systemd mode: %t", useSystemd)
	if useSystemd {
		// is StopSignal was not set by the user then set it to systemd
		// expected StopSigal
		if s.StopSignal == nil {
			stopSignal, err := util.ParseSignal("RTMIN+3")
			if err != nil {
				return nil, fmt.Errorf("error parsing systemd signal: %w", err)
			}
			s.StopSignal = &stopSignal
		}

		options = append(options, libpod.WithSystemd())
	}
	if len(s.SdNotifyMode) > 0 {
		options = append(options, libpod.WithSdNotifyMode(s.SdNotifyMode))
	}
	if pod != nil {
		logrus.Debugf("adding container to pod %s", pod.Name())
		options = append(options, rt.WithPod(pod))
	}
	destinations := []string{}
	// Take all mount and named volume destinations.
	for _, mount := range s.Mounts {
		destinations = append(destinations, mount.Destination)
	}
	for _, volume := range volumes {
		destinations = append(destinations, volume.Dest)
	}
	for _, overlayVolume := range overlays {
		destinations = append(destinations, overlayVolume.Destination)
	}
	for _, imageVolume := range s.ImageVolumes {
		destinations = append(destinations, imageVolume.Destination)
	}

	if len(destinations) > 0 || !infraVolumes {
		options = append(options, libpod.WithUserVolumes(destinations))
	}

	if len(volumes) != 0 {
		var vols []*libpod.ContainerNamedVolume
		for _, v := range volumes {
			vols = append(vols, &libpod.ContainerNamedVolume{
				Name:    v.Name,
				Dest:    v.Dest,
				Options: v.Options,
			})
		}
		options = append(options, libpod.WithNamedVolumes(vols))
	}

	if len(overlays) != 0 {
		var vols []*libpod.ContainerOverlayVolume
		for _, v := range overlays {
			vols = append(vols, &libpod.ContainerOverlayVolume{
				Dest:    v.Destination,
				Source:  v.Source,
				Options: v.Options,
			})
		}
		options = append(options, libpod.WithOverlayVolumes(vols))
	}

	if len(s.ImageVolumes) != 0 {
		var vols []*libpod.ContainerImageVolume
		for _, v := range s.ImageVolumes {
			vols = append(vols, &libpod.ContainerImageVolume{
				Dest:      v.Destination,
				Source:    v.Source,
				ReadWrite: v.ReadWrite,
			})
		}
		options = append(options, libpod.WithImageVolumes(vols))
	}

	if s.Command != nil {
		options = append(options, libpod.WithCommand(s.Command))
	}
	if s.Entrypoint != nil {
		options = append(options, libpod.WithEntrypoint(s.Entrypoint))
	}
	if len(s.ContainerStorageConfig.StorageOpts) > 0 {
		options = append(options, libpod.WithStorageOpts(s.StorageOpts))
	}
	// If the user did not specify a workdir on the CLI, let's extract it
	// from the image.
	if s.WorkDir == "" && imageData != nil {
		options = append(options, libpod.WithCreateWorkingDir())
		s.WorkDir = imageData.Config.WorkingDir
	}
	if s.WorkDir == "" {
		s.WorkDir = "/"
	}
	if s.CreateWorkingDir {
		options = append(options, libpod.WithCreateWorkingDir())
	}
	if s.StopSignal != nil {
		options = append(options, libpod.WithStopSignal(*s.StopSignal))
	}
	if s.StopTimeout != nil {
		options = append(options, libpod.WithStopTimeout(*s.StopTimeout))
	}
	if s.Timeout != 0 {
		options = append(options, libpod.WithTimeout(s.Timeout))
	}
	if s.LogConfiguration != nil {
		if len(s.LogConfiguration.Path) > 0 {
			options = append(options, libpod.WithLogPath(s.LogConfiguration.Path))
		}
		if s.LogConfiguration.Size > 0 {
			options = append(options, libpod.WithMaxLogSize(s.LogConfiguration.Size))
		}
		if len(s.LogConfiguration.Options) > 0 && s.LogConfiguration.Options["tag"] != "" {
			options = append(options, libpod.WithLogTag(s.LogConfiguration.Options["tag"]))
		}

		if len(s.LogConfiguration.Driver) > 0 {
			options = append(options, libpod.WithLogDriver(s.LogConfiguration.Driver))
		}
	}
	// Security options
	if len(s.SelinuxOpts) > 0 {
		options = append(options, libpod.WithSecLabels(s.SelinuxOpts))
	} else if pod != nil && len(compatibleOptions.SelinuxOpts) == 0 {
		// duplicate the security options from the pod
		processLabel, err := pod.ProcessLabel()
		if err != nil {
			return nil, err
		}
		if processLabel != "" {
			selinuxOpts, err := label.DupSecOpt(processLabel)
			if err != nil {
				return nil, err
			}
			options = append(options, libpod.WithSecLabels(selinuxOpts))
		}
	}
	options = append(options, libpod.WithPrivileged(s.Privileged))

	// Get namespace related options
	namespaceOpts, err := namespaceOptions(s, rt, pod, imageData)
	if err != nil {
		return nil, err
	}
	options = append(options, namespaceOpts...)

	if len(s.ConmonPidFile) > 0 {
		options = append(options, libpod.WithConmonPidFile(s.ConmonPidFile))
	}
	options = append(options, libpod.WithLabels(s.Labels))
	if s.ShmSize != nil {
		options = append(options, libpod.WithShmSize(*s.ShmSize))
	}
	if s.Rootfs != "" {
		options = append(options, libpod.WithRootFS(s.Rootfs, s.RootfsOverlay))
	}
	// Default used if not overridden on command line

	if s.RestartPolicy != "" {
		if s.RestartRetries != nil {
			options = append(options, libpod.WithRestartRetries(*s.RestartRetries))
		}
		options = append(options, libpod.WithRestartPolicy(s.RestartPolicy))
	}

	if s.ContainerHealthCheckConfig.HealthConfig != nil {
		options = append(options, libpod.WithHealthCheck(s.ContainerHealthCheckConfig.HealthConfig))
		logrus.Debugf("New container has a health check")
	}

	if len(s.Secrets) != 0 {
		manager, err := rt.SecretsManager()
		if err != nil {
			return nil, err
		}
		var secrs []*libpod.ContainerSecret
		for _, s := range s.Secrets {
			secr, err := manager.Lookup(s.Source)
			if err != nil {
				return nil, err
			}
			secrs = append(secrs, &libpod.ContainerSecret{
				Secret: secr,
				UID:    s.UID,
				GID:    s.GID,
				Mode:   s.Mode,
				Target: s.Target,
			})
		}
		options = append(options, libpod.WithSecrets(secrs))
	}

	if len(s.EnvSecrets) != 0 {
		options = append(options, libpod.WithEnvSecrets(s.EnvSecrets))
	}

	if len(s.DependencyContainers) > 0 {
		deps := make([]*libpod.Container, 0, len(s.DependencyContainers))
		for _, ctr := range s.DependencyContainers {
			depCtr, err := rt.LookupContainer(ctr)
			if err != nil {
				return nil, fmt.Errorf("%q is not a valid container, cannot be used as a dependency: %w", ctr, err)
			}
			deps = append(deps, depCtr)
		}
		options = append(options, libpod.WithDependencyCtrs(deps))
	}
	if s.PidFile != "" {
		options = append(options, libpod.WithPidFile(s.PidFile))
	}

	if len(s.ChrootDirs) != 0 {
		options = append(options, libpod.WithChrootDirs(s.ChrootDirs))
	}

	options = append(options, libpod.WithSelectedPasswordManagement(s.Passwd))

	return options, nil
}

func Inherit(infra libpod.Container, s *specgen.SpecGenerator, rt *libpod.Runtime) (opts []libpod.CtrCreateOption, infraS *spec.Spec, compat *libpod.InfraInherit, err error) {
	inheritSpec := &specgen.SpecGenerator{}
	_, compatibleOptions, err := ConfigToSpec(rt, inheritSpec, infra.ID())
	if err != nil {
		return nil, nil, nil, err
	}
	options := []libpod.CtrCreateOption{}
	infraConf := infra.Config()
	infraSpec := infraConf.Spec

	// need to set compatOptions to the currently filled specgenOptions so we do not overwrite
	compatibleOptions.CapAdd = append(compatibleOptions.CapAdd, s.CapAdd...)
	compatibleOptions.CapDrop = append(compatibleOptions.CapDrop, s.CapDrop...)
	compatibleOptions.HostDeviceList = append(compatibleOptions.HostDeviceList, s.HostDeviceList...)
	compatibleOptions.ImageVolumes = append(compatibleOptions.ImageVolumes, s.ImageVolumes...)
	compatibleOptions.Mounts = append(compatibleOptions.Mounts, s.Mounts...)
	compatibleOptions.OverlayVolumes = append(compatibleOptions.OverlayVolumes, s.OverlayVolumes...)
	compatibleOptions.SelinuxOpts = append(compatibleOptions.SelinuxOpts, s.SelinuxOpts...)
	compatibleOptions.Volumes = append(compatibleOptions.Volumes, s.Volumes...)

	compatByte, err := json.Marshal(compatibleOptions)
	if err != nil {
		return nil, nil, nil, err
	}
	err = json.Unmarshal(compatByte, s)
	if err != nil {
		return nil, nil, nil, err
	}

	// this causes errors when shmSize is the default value, it will still get passed down unless we manually override.
	if s.IpcNS.NSMode == specgen.Host && (compatibleOptions.ShmSize != nil && compatibleOptions.IsDefaultShmSize()) {
		s.ShmSize = nil
	}
	return options, infraSpec, compatibleOptions, nil
}
