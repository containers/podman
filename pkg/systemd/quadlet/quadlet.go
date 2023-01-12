package quadlet

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/containers/podman/v4/pkg/systemd/parser"
)

const (
	// Directory for global Quadlet files (sysadmin owned)
	UnitDirAdmin = "/etc/containers/systemd"
	// Directory for global Quadlet files (distro owned)
	UnitDirDistro = "/usr/share/containers/systemd"

	// Names of commonly used systemd/quadlet group names
	UnitGroup       = "Unit"
	InstallGroup    = "Install"
	ServiceGroup    = "Service"
	ContainerGroup  = "Container"
	XContainerGroup = "X-Container"
	VolumeGroup     = "Volume"
	XVolumeGroup    = "X-Volume"
	KubeGroup       = "Kube"
	XKubeGroup      = "X-Kube"
	NetworkGroup    = "Network"
	XNetworkGroup   = "X-Network"
)

// All the supported quadlet keys
const (
	KeyContainerName     = "ContainerName"
	KeyImage             = "Image"
	KeyEnvironment       = "Environment"
	KeyEnvironmentFile   = "EnvironmentFile"
	KeyEnvironmentHost   = "EnvironmentHost"
	KeyExec              = "Exec"
	KeyNoNewPrivileges   = "NoNewPrivileges"
	KeyDropCapability    = "DropCapability"
	KeyAddCapability     = "AddCapability"
	KeyReadOnly          = "ReadOnly"
	KeyRemapUsers        = "RemapUsers"
	KeyRemapUID          = "RemapUid"
	KeyRemapGID          = "RemapGid"
	KeyRemapUIDSize      = "RemapUidSize"
	KeyNotify            = "Notify"
	KeyExposeHostPort    = "ExposeHostPort"
	KeyPublishPort       = "PublishPort"
	KeyUser              = "User"
	KeyGroup             = "Group"
	KeyVolume            = "Volume"
	KeyPodmanArgs        = "PodmanArgs"
	KeyLabel             = "Label"
	KeyAnnotation        = "Annotation"
	KeyRunInit           = "RunInit"
	KeyVolatileTmp       = "VolatileTmp"
	KeyTimezone          = "Timezone"
	KeySeccompProfile    = "SeccompProfile"
	KeyAddDevice         = "AddDevice"
	KeyNetwork           = "Network"
	KeyYaml              = "Yaml"
	KeyNetworkDisableDNS = "DisableDNS"
	KeyNetworkDriver     = "Driver"
	KeyNetworkGateway    = "Gateway"
	KeyNetworkInternal   = "Internal"
	KeyNetworkIPRange    = "IPRange"
	KeyNetworkIPAMDriver = "IPAMDriver"
	KeyNetworkIPv6       = "IPv6"
	KeyNetworkOptions    = "Options"
	KeyNetworkSubnet     = "Subnet"
	KeyConfigMap         = "ConfigMap"
)

var (
	onceRegex      sync.Once
	validPortRange *regexp.Regexp

	// Supported keys in "Container" group
	supportedContainerKeys = map[string]bool{
		KeyContainerName:   true,
		KeyImage:           true,
		KeyEnvironment:     true,
		KeyEnvironmentFile: true,
		KeyEnvironmentHost: true,
		KeyExec:            true,
		KeyNoNewPrivileges: true,
		KeyDropCapability:  true,
		KeyAddCapability:   true,
		KeyReadOnly:        true,
		KeyRemapUsers:      true,
		KeyRemapUID:        true,
		KeyRemapGID:        true,
		KeyRemapUIDSize:    true,
		KeyNotify:          true,
		KeyExposeHostPort:  true,
		KeyPublishPort:     true,
		KeyUser:            true,
		KeyGroup:           true,
		KeyVolume:          true,
		KeyPodmanArgs:      true,
		KeyLabel:           true,
		KeyAnnotation:      true,
		KeyRunInit:         true,
		KeyVolatileTmp:     true,
		KeyTimezone:        true,
		KeySeccompProfile:  true,
		KeyAddDevice:       true,
		KeyNetwork:         true,
	}

	// Supported keys in "Volume" group
	supportedVolumeKeys = map[string]bool{
		KeyUser:  true,
		KeyGroup: true,
		KeyLabel: true,
	}

	// Supported keys in "Volume" group
	supportedNetworkKeys = map[string]bool{
		KeyNetworkDisableDNS: true,
		KeyNetworkDriver:     true,
		KeyNetworkGateway:    true,
		KeyNetworkInternal:   true,
		KeyNetworkIPRange:    true,
		KeyNetworkIPAMDriver: true,
		KeyNetworkIPv6:       true,
		KeyNetworkOptions:    true,
		KeyNetworkSubnet:     true,
		KeyLabel:             true,
	}

	// Supported keys in "Kube" group
	supportedKubeKeys = map[string]bool{
		KeyYaml:         true,
		KeyRemapUID:     true,
		KeyRemapGID:     true,
		KeyRemapUsers:   true,
		KeyRemapUIDSize: true,
		KeyNetwork:      true,
		KeyConfigMap:    true,
		KeyPublishPort:  true,
	}
)

func replaceExtension(name string, extension string, extraPrefix string, extraSuffix string) string {
	baseName := name

	dot := strings.LastIndexByte(name, '.')
	if dot > 0 {
		baseName = name[:dot]
	}

	return extraPrefix + baseName + extraSuffix + extension
}

func isPortRange(port string) bool {
	onceRegex.Do(func() {
		validPortRange = regexp.MustCompile(`\d+(-\d+)?(/udp|/tcp)?$`)
	})
	return validPortRange.MatchString(port)
}

func checkForUnknownKeys(unit *parser.UnitFile, groupName string, supportedKeys map[string]bool) error {
	keys := unit.ListKeys(groupName)
	for _, key := range keys {
		if !supportedKeys[key] {
			return fmt.Errorf("unsupported key '%s' in group '%s' in %s", key, groupName, unit.Path)
		}
	}
	return nil
}

func splitPorts(ports string) []string {
	parts := make([]string, 0)

	// IP address could have colons in it. For example: "[::]:8080:80/tcp, so we split carefully
	start := 0
	end := 0
	for end < len(ports) {
		switch ports[end] {
		case '[':
			end++
			for end < len(ports) && ports[end] != ']' {
				end++
			}
			if end < len(ports) {
				end++ // Skip ]
			}
		case ':':
			parts = append(parts, ports[start:end])
			end++
			start = end
		default:
			end++
		}
	}

	parts = append(parts, ports[start:end])
	return parts
}

func usernsOpts(kind string, opts []string) string {
	var res strings.Builder
	res.WriteString(kind)
	if len(opts) > 0 {
		res.WriteString(":")
	}
	for i, opt := range opts {
		if i != 0 {
			res.WriteString(",")
		}
		res.WriteString(opt)
	}
	return res.String()
}

// Convert a quadlet container file (unit file with a Container group) to a systemd
// service file (unit file with Service group) based on the options in the
// Container group.
// The original Container group is kept around as X-Container.
func ConvertContainer(container *parser.UnitFile, isUser bool) (*parser.UnitFile, error) {
	service := container.Dup()
	service.Filename = replaceExtension(container.Filename, ".service", "", "")

	if container.Path != "" {
		service.Add(UnitGroup, "SourcePath", container.Path)
	}

	if err := checkForUnknownKeys(container, ContainerGroup, supportedContainerKeys); err != nil {
		return nil, err
	}

	// Rename old Container group to x-Container so that systemd ignores it
	service.RenameGroup(ContainerGroup, XContainerGroup)

	image, ok := container.Lookup(ContainerGroup, KeyImage)
	if !ok || len(image) == 0 {
		return nil, fmt.Errorf("no Image key specified")
	}

	containerName, ok := container.Lookup(ContainerGroup, KeyContainerName)
	if !ok || len(containerName) == 0 {
		// By default, We want to name the container by the service name
		containerName = "systemd-%N"
	}

	// Set PODMAN_SYSTEMD_UNIT so that podman auto-update can restart the service.
	service.Add(ServiceGroup, "Environment", "PODMAN_SYSTEMD_UNIT=%n")

	// Only allow mixed or control-group, as nothing else works well
	killMode, ok := service.Lookup(ServiceGroup, "KillMode")
	if !ok || !(killMode == "mixed" || killMode == "control-group") {
		if ok {
			return nil, fmt.Errorf("invalid KillMode '%s'", killMode)
		}

		// We default to mixed instead of control-group, because it lets conmon do its thing
		service.Set(ServiceGroup, "KillMode", "mixed")
	}

	// Read env early so we can override it below
	podmanEnv := container.LookupAllKeyVal(ContainerGroup, KeyEnvironment)

	// Need the containers filesystem mounted to start podman
	service.Add(UnitGroup, "RequiresMountsFor", "%t/containers")

	// If the conman exited uncleanly it may not have removed the container, so force it,
	// -i makes it ignore non-existing files.
	service.Add(ServiceGroup, "ExecStopPost", "-"+podmanBinary()+" rm -f -i --cidfile=%t/%N.cid")

	// Remove the cid file, to avoid confusion as the container is no longer running.
	service.Add(ServiceGroup, "ExecStopPost", "-rm -f %t/%N.cid")

	podman := NewPodmanCmdline("run")

	podman.addf("--name=%s", containerName)

	podman.add(
		// We store the container id so we can clean it up in case of failure
		"--cidfile=%t/%N.cid",

		// And replace any previous container with the same name, not fail
		"--replace",

		// On clean shutdown, remove container
		"--rm",

		// But we still want output to the journal, so use the log driver.
		"--log-driver", "passthrough",
	)

	// We use crun as the runtime and delegated groups to it
	service.Add(ServiceGroup, "Delegate", "yes")
	podman.add(
		"--runtime", "/usr/bin/crun",
		"--cgroups=split")

	timezone, ok := container.Lookup(ContainerGroup, KeyTimezone)
	if ok && len(timezone) > 0 {
		podman.addf("--tz=%s", timezone)
	}

	addNetworks(container, ContainerGroup, service, podman)

	// Run with a pid1 init to reap zombies by default (as most apps don't do that)
	runInit, ok := container.LookupBoolean(ContainerGroup, KeyRunInit)
	if ok {
		podman.addBool("--init", runInit)
	}

	serviceType, ok := service.Lookup(ServiceGroup, "Type")
	if ok && serviceType != "notify" && serviceType != "oneshot" {
		return nil, fmt.Errorf("invalid service Type '%s'", serviceType)
	}

	if serviceType != "oneshot" {
		// If we're not in oneshot mode always use some form of sd-notify, normally via conmon,
		// but we also allow passing it to the container by setting Notify=yes
		notify := container.LookupBooleanWithDefault(ContainerGroup, KeyNotify, false)
		if notify {
			podman.add("--sdnotify=container")
		} else {
			podman.add("--sdnotify=conmon")
		}
		service.Setv(ServiceGroup,
			"Type", "notify",
			"NotifyAccess", "all")

		// Detach from container, we don't need the podman process to hang around
		podman.add("-d")
	}

	if !container.HasKey(ServiceGroup, "SyslogIdentifier") {
		service.Set(ServiceGroup, "SyslogIdentifier", "%N")
	}

	// Default to no higher level privileges or caps
	noNewPrivileges := container.LookupBooleanWithDefault(ContainerGroup, KeyNoNewPrivileges, false)
	if noNewPrivileges {
		podman.add("--security-opt=no-new-privileges")
	}

	// But allow overrides with AddCapability
	devices := container.LookupAllStrv(ContainerGroup, KeyAddDevice)
	for _, device := range devices {
		podman.addf("--device=%s", device)
	}

	// Default to no higher level privileges or caps
	seccompProfile, hasSeccompProfile := container.Lookup(ContainerGroup, KeySeccompProfile)
	if hasSeccompProfile {
		podman.add("--security-opt", fmt.Sprintf("seccomp=%s", seccompProfile))
	}

	dropCaps := container.LookupAllStrv(ContainerGroup, KeyDropCapability)

	for _, caps := range dropCaps {
		podman.addf("--cap-drop=%s", strings.ToLower(caps))
	}

	// But allow overrides with AddCapability
	addCaps := container.LookupAllStrv(ContainerGroup, KeyAddCapability)
	for _, caps := range addCaps {
		podman.addf("--cap-add=%s", strings.ToLower(caps))
	}

	readOnly, ok := container.LookupBoolean(ContainerGroup, KeyReadOnly)
	if ok {
		podman.addBool("--read-only", readOnly)
	}

	volatileTmp := container.LookupBooleanWithDefault(ContainerGroup, KeyVolatileTmp, false)
	if volatileTmp {
		/* Read only mode already has a tmpfs by default */
		if !readOnly {
			podman.add("--tmpfs", "/tmp:rw,size=512M,mode=1777")
		}
	} else if readOnly {
		/* !volatileTmp, disable the default tmpfs from --read-only */
		podman.add("--read-only-tmpfs=false")
	}

	hasUser := container.HasKey(ContainerGroup, KeyUser)
	hasGroup := container.HasKey(ContainerGroup, KeyGroup)
	if hasUser || hasGroup {
		uid := container.LookupUint32(ContainerGroup, KeyUser, 0)
		gid := container.LookupUint32(ContainerGroup, KeyGroup, 0)

		podman.add("--user")
		if hasGroup {
			podman.addf("%d:%d", uid, gid)
		} else {
			podman.addf("%d", uid)
		}
	}

	if err := handleUserRemap(container, ContainerGroup, podman, isUser, true); err != nil {
		return nil, err
	}

	volumes := container.LookupAll(ContainerGroup, KeyVolume)
	for _, volume := range volumes {
		parts := strings.SplitN(volume, ":", 3)

		source := ""
		var dest string
		options := ""
		if len(parts) >= 2 {
			source = parts[0]
			dest = parts[1]
		} else {
			dest = parts[0]
		}
		if len(parts) >= 3 {
			options = ":" + parts[2]
		}

		if source != "" {
			if source[0] == '/' {
				// Absolute path
				service.Add(UnitGroup, "RequiresMountsFor", source)
			} else if strings.HasSuffix(source, ".volume") {
				// the podman volume name is systemd-$name
				volumeName := replaceExtension(source, "", "systemd-", "")

				// the systemd unit name is $name-volume.service
				volumeServiceName := replaceExtension(source, ".service", "", "-volume")

				source = volumeName

				service.Add(UnitGroup, "Requires", volumeServiceName)
				service.Add(UnitGroup, "After", volumeServiceName)
			}
		}

		podman.add("-v")
		if source == "" {
			podman.add(dest)
		} else {
			podman.addf("%s:%s%s", source, dest, options)
		}
	}

	exposedPorts := container.LookupAll(ContainerGroup, KeyExposeHostPort)
	for _, exposedPort := range exposedPorts {
		exposedPort = strings.TrimSpace(exposedPort) // Allow whitespace after

		if !isPortRange(exposedPort) {
			return nil, fmt.Errorf("invalid port format '%s'", exposedPort)
		}

		podman.addf("--expose=%s", exposedPort)
	}

	if err := handlePublishPorts(container, ContainerGroup, podman); err != nil {
		return nil, err
	}

	podman.addEnv(podmanEnv)

	labels := container.LookupAllKeyVal(ContainerGroup, KeyLabel)
	podman.addLabels(labels)

	annotations := container.LookupAllKeyVal(ContainerGroup, KeyAnnotation)
	podman.addAnnotations(annotations)

	envFiles := container.LookupAllArgs(ContainerGroup, KeyEnvironmentFile)
	for _, envFile := range envFiles {
		filePath, err := getAbsolutePath(container, envFile)
		if err != nil {
			return nil, err
		}
		podman.add("--env-file", filePath)
	}

	if envHost, ok := container.LookupBoolean(ContainerGroup, KeyEnvironmentHost); ok {
		podman.addBool("--env-host", envHost)
	}

	podmanArgs := container.LookupAllArgs(ContainerGroup, KeyPodmanArgs)
	podman.add(podmanArgs...)

	podman.add(image)

	execArgs, ok := container.LookupLastArgs(ContainerGroup, KeyExec)
	if ok {
		podman.add(execArgs...)
	}

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	return service, nil
}

// Convert a quadlet network file (unit file with a Network group) to a systemd
// service file (unit file with Service group) based on the options in the
// Network group.
// The original Network group is kept around as X-Network.
func ConvertNetwork(network *parser.UnitFile, name string) (*parser.UnitFile, error) {
	service := network.Dup()
	service.Filename = replaceExtension(network.Filename, ".service", "", "-network")

	if err := checkForUnknownKeys(network, NetworkGroup, supportedNetworkKeys); err != nil {
		return nil, err
	}

	/* Rename old Network group to x-Network so that systemd ignores it */
	service.RenameGroup(NetworkGroup, XNetworkGroup)

	networkName := replaceExtension(name, "", "systemd-", "")

	// Need the containers filesystem mounted to start podman
	service.Add(UnitGroup, "RequiresMountsFor", "%t/containers")

	podman := NewPodmanCmdline("network", "create", "--ignore")

	if disableDNS := network.LookupBooleanWithDefault(NetworkGroup, KeyNetworkDisableDNS, false); disableDNS {
		podman.add("--disable-dns")
	}

	driver, ok := network.Lookup(NetworkGroup, KeyNetworkDriver)
	if ok && len(driver) > 0 {
		podman.addf("--driver=%s", driver)
	}

	subnets := network.LookupAll(NetworkGroup, KeyNetworkSubnet)
	gateways := network.LookupAll(NetworkGroup, KeyNetworkGateway)
	ipRanges := network.LookupAll(NetworkGroup, KeyNetworkIPRange)
	if len(subnets) > 0 {
		if len(gateways) > len(subnets) {
			return nil, fmt.Errorf("cannot set more gateways than subnets")
		}
		if len(ipRanges) > len(subnets) {
			return nil, fmt.Errorf("cannot set more ranges than subnets")
		}
		for i := range subnets {
			podman.addf("--subnet=%s", subnets[i])
			if len(gateways) > i {
				podman.addf("--gateway=%s", gateways[i])
			}
			if len(ipRanges) > i {
				podman.addf("--ip-range=%s", ipRanges[i])
			}
		}
	} else if len(ipRanges) > 0 || len(gateways) > 0 {
		return nil, fmt.Errorf("cannot set gateway or range without subnet")
	}

	if internal := network.LookupBooleanWithDefault(NetworkGroup, KeyNetworkInternal, false); internal {
		podman.add("--internal")
	}

	if ipamDriver, ok := network.Lookup(NetworkGroup, KeyNetworkIPAMDriver); ok && len(ipamDriver) > 0 {
		podman.addf("--ipam-driver=%s", ipamDriver)
	}

	if ipv6 := network.LookupBooleanWithDefault(NetworkGroup, KeyNetworkIPv6, false); ipv6 {
		podman.add("--ipv6")
	}

	networkOptions := network.LookupAllKeyVal(NetworkGroup, KeyNetworkOptions)
	if len(networkOptions) > 0 {
		podman.addKeys("--opt", networkOptions)
	}

	if labels := network.LookupAllKeyVal(NetworkGroup, KeyLabel); len(labels) > 0 {
		podman.addLabels(labels)
	}

	podman.add(networkName)

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	service.Setv(ServiceGroup,
		"Type", "oneshot",
		"RemainAfterExit", "yes",

		// The default syslog identifier is the exec basename (podman) which isn't very useful here
		"SyslogIdentifier", "%N")

	return service, nil
}

// Convert a quadlet volume file (unit file with a Volume group) to a systemd
// service file (unit file with Service group) based on the options in the
// Volume group.
// The original Volume group is kept around as X-Volume.
func ConvertVolume(volume *parser.UnitFile, name string) (*parser.UnitFile, error) {
	service := volume.Dup()
	service.Filename = replaceExtension(volume.Filename, ".service", "", "-volume")

	if err := checkForUnknownKeys(volume, VolumeGroup, supportedVolumeKeys); err != nil {
		return nil, err
	}

	/* Rename old Volume group to x-Volume so that systemd ignores it */
	service.RenameGroup(VolumeGroup, XVolumeGroup)

	volumeName := replaceExtension(name, "", "systemd-", "")

	// Need the containers filesystem mounted to start podman
	service.Add(UnitGroup, "RequiresMountsFor", "%t/containers")

	labels := volume.LookupAllKeyVal(VolumeGroup, "Label")

	podman := NewPodmanCmdline("volume", "create", "--ignore")

	var opts strings.Builder
	opts.WriteString("o=")

	if volume.HasKey(VolumeGroup, "User") {
		uid := volume.LookupUint32(VolumeGroup, "User", 0)
		if opts.Len() > 2 {
			opts.WriteString(",")
		}
		opts.WriteString(fmt.Sprintf("uid=%d", uid))
	}

	if volume.HasKey(VolumeGroup, "Group") {
		gid := volume.LookupUint32(VolumeGroup, "Group", 0)
		if opts.Len() > 2 {
			opts.WriteString(",")
		}
		opts.WriteString(fmt.Sprintf("gid=%d", gid))
	}

	if opts.Len() > 2 {
		podman.add("--opt", opts.String())
	}

	podman.addLabels(labels)
	podman.add(volumeName)

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	service.Setv(ServiceGroup,
		"Type", "oneshot",
		"RemainAfterExit", "yes",

		// The default syslog identifier is the exec basename (podman) which isn't very useful here
		"SyslogIdentifier", "%N")

	return service, nil
}

func ConvertKube(kube *parser.UnitFile, isUser bool) (*parser.UnitFile, error) {
	service := kube.Dup()
	service.Filename = replaceExtension(kube.Filename, ".service", "", "")

	if kube.Path != "" {
		service.Add(UnitGroup, "SourcePath", kube.Path)
	}

	if err := checkForUnknownKeys(kube, KubeGroup, supportedKubeKeys); err != nil {
		return nil, err
	}

	// Rename old Kube group to x-Kube so that systemd ignores it
	service.RenameGroup(KubeGroup, XKubeGroup)

	yamlPath, ok := kube.Lookup(KubeGroup, KeyYaml)
	if !ok || len(yamlPath) == 0 {
		return nil, fmt.Errorf("no Yaml key specified")
	}

	yamlPath, err := getAbsolutePath(kube, yamlPath)
	if err != nil {
		return nil, err
	}

	// Only allow mixed or control-group, as nothing else works well
	killMode, ok := service.Lookup(ServiceGroup, "KillMode")
	if !ok || !(killMode == "mixed" || killMode == "control-group") {
		if ok {
			return nil, fmt.Errorf("invalid KillMode '%s'", killMode)
		}

		// We default to mixed instead of control-group, because it lets conmon do its thing
		service.Set(ServiceGroup, "KillMode", "mixed")
	}

	// Set PODMAN_SYSTEMD_UNIT so that podman auto-update can restart the service.
	service.Add(ServiceGroup, "Environment", "PODMAN_SYSTEMD_UNIT=%n")

	// Need the containers filesystem mounted to start podman
	service.Add(UnitGroup, "RequiresMountsFor", "%t/containers")

	service.Setv(ServiceGroup,
		"Type", "notify",
		"NotifyAccess", "all")

	if !kube.HasKey(ServiceGroup, "SyslogIdentifier") {
		service.Set(ServiceGroup, "SyslogIdentifier", "%N")
	}

	execStart := NewPodmanCmdline("kube", "play")

	execStart.add(
		// Replace any previous container with the same name, not fail
		"--replace",

		// Use a service container
		"--service-container=true",
	)

	if err := handleUserRemap(kube, KubeGroup, execStart, isUser, false); err != nil {
		return nil, err
	}

	addNetworks(kube, KubeGroup, service, execStart)

	configMaps := kube.LookupAllStrv(KubeGroup, KeyConfigMap)
	for _, configMap := range configMaps {
		configMapPath, err := getAbsolutePath(kube, configMap)
		if err != nil {
			return nil, err
		}
		execStart.add("--configmap", configMapPath)
	}

	if err := handlePublishPorts(kube, KubeGroup, execStart); err != nil {
		return nil, err
	}

	execStart.add(yamlPath)

	service.AddCmdline(ServiceGroup, "ExecStart", execStart.Args)

	execStop := NewPodmanCmdline("kube", "down")
	execStop.add(yamlPath)
	service.AddCmdline(ServiceGroup, "ExecStop", execStop.Args)

	return service, nil
}

func handleUserRemap(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline, isUser, supportManual bool) error {
	uidMaps := unitFile.LookupAllStrv(groupName, KeyRemapUID)
	gidMaps := unitFile.LookupAllStrv(groupName, KeyRemapGID)
	remapUsers, _ := unitFile.LookupLast(groupName, KeyRemapUsers)
	switch remapUsers {
	case "":
		if len(uidMaps) > 0 {
			return fmt.Errorf("UidMap set without RemapUsers")
		}
		if len(gidMaps) > 0 {
			return fmt.Errorf("GidMap set without RemapUsers")
		}
	case "manual":
		if supportManual {
			for _, uidMap := range uidMaps {
				podman.addf("--uidmap=%s", uidMap)
			}
			for _, gidMap := range gidMaps {
				podman.addf("--gidmap=%s", gidMap)
			}
		} else {
			return fmt.Errorf("RemapUsers=manual is not supported")
		}
	case "auto":
		autoOpts := make([]string, 0)
		for _, uidMap := range uidMaps {
			autoOpts = append(autoOpts, "uidmapping="+uidMap)
		}
		for _, gidMap := range gidMaps {
			autoOpts = append(autoOpts, "gidmapping="+gidMap)
		}
		uidSize := unitFile.LookupUint32(groupName, KeyRemapUIDSize, 0)
		if uidSize > 0 {
			autoOpts = append(autoOpts, fmt.Sprintf("size=%v", uidSize))
		}

		podman.addf("--userns=" + usernsOpts("auto", autoOpts))
	case "keep-id":
		if !isUser {
			return fmt.Errorf("RemapUsers=keep-id is unsupported for system units")
		}
		podman.addf("--userns=keep-id")
	default:
		return fmt.Errorf("unsupported RemapUsers option '%s'", remapUsers)
	}

	return nil
}

func addNetworks(quadletUnitFile *parser.UnitFile, groupName string, serviceUnitFile *parser.UnitFile, podman *PodmanCmdline) {
	networks := quadletUnitFile.LookupAll(groupName, KeyNetwork)
	for _, network := range networks {
		if len(network) > 0 {
			quadletNetworkName, options, found := strings.Cut(network, ":")
			if strings.HasSuffix(quadletNetworkName, ".network") {
				// the podman network name is systemd-$name
				networkName := replaceExtension(quadletNetworkName, "", "systemd-", "")

				// the systemd unit name is $name-network.service
				networkServiceName := replaceExtension(quadletNetworkName, ".service", "", "-network")

				serviceUnitFile.Add(UnitGroup, "Requires", networkServiceName)
				serviceUnitFile.Add(UnitGroup, "After", networkServiceName)

				if found {
					network = fmt.Sprintf("%s:%s", networkName, options)
				} else {
					network = networkName
				}
			}

			podman.addf("--network=%s", network)
		}
	}
}

func getAbsolutePath(quadletUnitFile *parser.UnitFile, filePath string) (string, error) {
	if !filepath.IsAbs(filePath) {
		if len(quadletUnitFile.Path) > 0 {
			filePath = filepath.Join(filepath.Dir(quadletUnitFile.Path), filePath)
		} else {
			var err error
			filePath, err = filepath.Abs(filePath)
			if err != nil {
				return "", err
			}
		}
	}
	return filePath, nil
}

func handlePublishPorts(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) error {
	publishPorts := unitFile.LookupAll(groupName, KeyPublishPort)
	for _, publishPort := range publishPorts {
		publishPort = strings.TrimSpace(publishPort) // Allow whitespace after

		// IP address could have colons in it. For example: "[::]:8080:80/tcp, so use custom splitter
		parts := splitPorts(publishPort)

		var containerPort string
		ip := ""
		hostPort := ""

		// format (from podman run):
		// ip:hostPort:containerPort | ip::containerPort | hostPort:containerPort | containerPort
		//
		// ip could be IPv6 with minimum of these chars "[::]"
		// containerPort can have a suffix of "/tcp" or "/udp"
		//

		switch len(parts) {
		case 1:
			containerPort = parts[0]

		case 2:
			hostPort = parts[0]
			containerPort = parts[1]

		case 3:
			ip = parts[0]
			hostPort = parts[1]
			containerPort = parts[2]

		default:
			return fmt.Errorf("invalid published port '%s'", publishPort)
		}

		if ip == "0.0.0.0" {
			ip = ""
		}

		if len(hostPort) > 0 && !isPortRange(hostPort) {
			return fmt.Errorf("invalid port format '%s'", hostPort)
		}

		if len(containerPort) > 0 && !isPortRange(containerPort) {
			return fmt.Errorf("invalid port format '%s'", containerPort)
		}

		podman.add("--publish")
		switch {
		case len(ip) > 0 && len(hostPort) > 0:
			podman.addf("%s:%s:%s", ip, hostPort, containerPort)
		case len(ip) > 0:
			podman.addf("%s::%s", ip, containerPort)
		case len(hostPort) > 0:
			podman.addf("%s:%s", hostPort, containerPort)
		default:
			podman.addf("%s", containerPort)
		}
	}

	return nil
}
