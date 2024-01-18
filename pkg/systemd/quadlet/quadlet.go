package quadlet

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v4/pkg/specgenutilexternal"
	"github.com/containers/podman/v4/pkg/systemd/parser"
	"github.com/containers/storage/pkg/regexp"
)

const (
	// Fixme should use
	// github.com/containers/podman/v4/libpod/define.AutoUpdateLabel
	// but it is causing bloat
	autoUpdateLabel = "io.containers.autoupdate"
	// Directory for global Quadlet files (sysadmin owned)
	UnitDirAdmin = "/etc/containers/systemd"
	// Directory for global Quadlet files (distro owned)
	UnitDirDistro = "/usr/share/containers/systemd"

	// Names of commonly used systemd/quadlet group names
	ContainerGroup  = "Container"
	InstallGroup    = "Install"
	KubeGroup       = "Kube"
	NetworkGroup    = "Network"
	PodGroup        = "Pod"
	ServiceGroup    = "Service"
	UnitGroup       = "Unit"
	VolumeGroup     = "Volume"
	ImageGroup      = "Image"
	XContainerGroup = "X-Container"
	XKubeGroup      = "X-Kube"
	XNetworkGroup   = "X-Network"
	XPodGroup       = "X-Pod"
	XVolumeGroup    = "X-Volume"
	XImageGroup     = "X-Image"
)

// Systemd Unit file keys
const (
	ServiceKeyWorkingDirectory = "WorkingDirectory"
)

// All the supported quadlet keys
const (
	KeyAddCapability         = "AddCapability"
	KeyAddDevice             = "AddDevice"
	KeyAllTags               = "AllTags"
	KeyAnnotation            = "Annotation"
	KeyArch                  = "Arch"
	KeyAuthFile              = "AuthFile"
	KeyAutoUpdate            = "AutoUpdate"
	KeyCertDir               = "CertDir"
	KeyConfigMap             = "ConfigMap"
	KeyContainerName         = "ContainerName"
	KeyContainersConfModule  = "ContainersConfModule"
	KeyCopy                  = "Copy"
	KeyCreds                 = "Creds"
	KeyDecryptionKey         = "DecryptionKey"
	KeyDevice                = "Device"
	KeyDisableDNS            = "DisableDNS"
	KeyDNS                   = "DNS"
	KeyDNSOption             = "DNSOption"
	KeyDNSSearch             = "DNSSearch"
	KeyDriver                = "Driver"
	KeyDropCapability        = "DropCapability"
	KeyEntrypoint            = "Entrypoint"
	KeyEnvironment           = "Environment"
	KeyEnvironmentFile       = "EnvironmentFile"
	KeyEnvironmentHost       = "EnvironmentHost"
	KeyExec                  = "Exec"
	KeyExitCodePropagation   = "ExitCodePropagation"
	KeyExposeHostPort        = "ExposeHostPort"
	KeyGateway               = "Gateway"
	KeyGIDMap                = "GIDMap"
	KeyGlobalArgs            = "GlobalArgs"
	KeyGroup                 = "Group"
	KeyHealthCmd             = "HealthCmd"
	KeyHealthInterval        = "HealthInterval"
	KeyHealthOnFailure       = "HealthOnFailure"
	KeyHealthRetries         = "HealthRetries"
	KeyHealthStartPeriod     = "HealthStartPeriod"
	KeyHealthStartupCmd      = "HealthStartupCmd"
	KeyHealthStartupInterval = "HealthStartupInterval"
	KeyHealthStartupRetries  = "HealthStartupRetries"
	KeyHealthStartupSuccess  = "HealthStartupSuccess"
	KeyHealthStartupTimeout  = "HealthStartupTimeout"
	KeyHealthTimeout         = "HealthTimeout"
	KeyHostName              = "HostName"
	KeyImage                 = "Image"
	KeyImageTag              = "ImageTag"
	KeyInternal              = "Internal"
	KeyIP                    = "IP"
	KeyIP6                   = "IP6"
	KeyIPAMDriver            = "IPAMDriver"
	KeyIPRange               = "IPRange"
	KeyIPv6                  = "IPv6"
	KeyKubeDownForce         = "KubeDownForce"
	KeyLabel                 = "Label"
	KeyLogDriver             = "LogDriver"
	KeyMask                  = "Mask"
	KeyMount                 = "Mount"
	KeyNetwork               = "Network"
	KeyNetworkName           = "NetworkName"
	KeyNoNewPrivileges       = "NoNewPrivileges"
	KeyNotify                = "Notify"
	KeyOptions               = "Options"
	KeyOS                    = "OS"
	KeyPidsLimit             = "PidsLimit"
	KeyPod                   = "Pod"
	KeyPodmanArgs            = "PodmanArgs"
	KeyPodName               = "PodName"
	KeyPublishPort           = "PublishPort"
	KeyPull                  = "Pull"
	KeyReadOnly              = "ReadOnly"
	KeyReadOnlyTmpfs         = "ReadOnlyTmpfs"
	KeyRemapGid              = "RemapGid"     //nolint:stylecheck // deprecated
	KeyRemapUid              = "RemapUid"     //nolint:stylecheck // deprecated
	KeyRemapUidSize          = "RemapUidSize" //nolint:stylecheck // deprecated
	KeyRemapUsers            = "RemapUsers"   // deprecated
	KeyRootfs                = "Rootfs"
	KeyRunInit               = "RunInit"
	KeySeccompProfile        = "SeccompProfile"
	KeySecret                = "Secret"
	KeySecurityLabelDisable  = "SecurityLabelDisable"
	KeySecurityLabelFileType = "SecurityLabelFileType"
	KeySecurityLabelLevel    = "SecurityLabelLevel"
	KeySecurityLabelNested   = "SecurityLabelNested"
	KeySecurityLabelType     = "SecurityLabelType"
	KeySetWorkingDirectory   = "SetWorkingDirectory"
	KeyShmSize               = "ShmSize"
	KeyStopTimeout           = "StopTimeout"
	KeySubGIDMap             = "SubGIDMap"
	KeySubnet                = "Subnet"
	KeySubUIDMap             = "SubUIDMap"
	KeySysctl                = "Sysctl"
	KeyTimezone              = "Timezone"
	KeyTLSVerify             = "TLSVerify"
	KeyTmpfs                 = "Tmpfs"
	KeyType                  = "Type"
	KeyUIDMap                = "UIDMap"
	KeyUlimit                = "Ulimit"
	KeyUnmask                = "Unmask"
	KeyUser                  = "User"
	KeyUserNS                = "UserNS"
	KeyVariant               = "Variant"
	KeyVolatileTmp           = "VolatileTmp" // deprecated
	KeyVolume                = "Volume"
	KeyVolumeName            = "VolumeName"
	KeyWorkingDir            = "WorkingDir"
	KeyYaml                  = "Yaml"
)

type PodInfo struct {
	ServiceName string
	Containers  []string
}

var (
	validPortRange = regexp.Delayed(`\d+(-\d+)?(/udp|/tcp)?$`)

	// Supported keys in "Container" group
	supportedContainerKeys = map[string]bool{
		KeyAddCapability:         true,
		KeyAddDevice:             true,
		KeyAnnotation:            true,
		KeyAutoUpdate:            true,
		KeyContainerName:         true,
		KeyContainersConfModule:  true,
		KeyDNS:                   true,
		KeyDNSOption:             true,
		KeyDNSSearch:             true,
		KeyDropCapability:        true,
		KeyEnvironment:           true,
		KeyEnvironmentFile:       true,
		KeyEnvironmentHost:       true,
		KeyEntrypoint:            true,
		KeyExec:                  true,
		KeyExposeHostPort:        true,
		KeyGIDMap:                true,
		KeyGlobalArgs:            true,
		KeyGroup:                 true,
		KeyHealthCmd:             true,
		KeyHealthInterval:        true,
		KeyHealthOnFailure:       true,
		KeyHealthRetries:         true,
		KeyHealthStartPeriod:     true,
		KeyHealthStartupCmd:      true,
		KeyHealthStartupInterval: true,
		KeyHealthStartupRetries:  true,
		KeyHealthStartupSuccess:  true,
		KeyHealthStartupTimeout:  true,
		KeyHealthTimeout:         true,
		KeyHostName:              true,
		KeyIP6:                   true,
		KeyIP:                    true,
		KeyImage:                 true,
		KeyLabel:                 true,
		KeyLogDriver:             true,
		KeyMask:                  true,
		KeyMount:                 true,
		KeyNetwork:               true,
		KeyNoNewPrivileges:       true,
		KeyNotify:                true,
		KeyPidsLimit:             true,
		KeyPod:                   true,
		KeyPodmanArgs:            true,
		KeyPublishPort:           true,
		KeyPull:                  true,
		KeyReadOnly:              true,
		KeyReadOnlyTmpfs:         true,
		KeyRemapGid:              true,
		KeyRemapUid:              true,
		KeyRemapUidSize:          true,
		KeyRemapUsers:            true,
		KeyRootfs:                true,
		KeyRunInit:               true,
		KeySeccompProfile:        true,
		KeySecret:                true,
		KeySecurityLabelDisable:  true,
		KeySecurityLabelFileType: true,
		KeySecurityLabelLevel:    true,
		KeySecurityLabelNested:   true,
		KeySecurityLabelType:     true,
		KeyShmSize:               true,
		KeyStopTimeout:           true,
		KeySubGIDMap:             true,
		KeySubUIDMap:             true,
		KeySysctl:                true,
		KeyTimezone:              true,
		KeyTmpfs:                 true,
		KeyUIDMap:                true,
		KeyUlimit:                true,
		KeyUnmask:                true,
		KeyUser:                  true,
		KeyUserNS:                true,
		KeyVolatileTmp:           true,
		KeyVolume:                true,
		KeyWorkingDir:            true,
	}

	// Supported keys in "Volume" group
	supportedVolumeKeys = map[string]bool{
		KeyContainersConfModule: true,
		KeyCopy:                 true,
		KeyDevice:               true,
		KeyDriver:               true,
		KeyGlobalArgs:           true,
		KeyGroup:                true,
		KeyImage:                true,
		KeyLabel:                true,
		KeyOptions:              true,
		KeyPodmanArgs:           true,
		KeyType:                 true,
		KeyUser:                 true,
		KeyVolumeName:           true,
	}

	// Supported keys in "Network" group
	supportedNetworkKeys = map[string]bool{
		KeyLabel:                true,
		KeyDNS:                  true,
		KeyContainersConfModule: true,
		KeyGlobalArgs:           true,
		KeyDisableDNS:           true,
		KeyDriver:               true,
		KeyGateway:              true,
		KeyIPAMDriver:           true,
		KeyIPRange:              true,
		KeyIPv6:                 true,
		KeyInternal:             true,
		KeyNetworkName:          true,
		KeyOptions:              true,
		KeySubnet:               true,
		KeyPodmanArgs:           true,
	}

	// Supported keys in "Kube" group
	supportedKubeKeys = map[string]bool{
		KeyAutoUpdate:           true,
		KeyConfigMap:            true,
		KeyContainersConfModule: true,
		KeyExitCodePropagation:  true,
		KeyGlobalArgs:           true,
		KeyKubeDownForce:        true,
		KeyLogDriver:            true,
		KeyNetwork:              true,
		KeyPodmanArgs:           true,
		KeyPublishPort:          true,
		KeyRemapGid:             true,
		KeyRemapUid:             true,
		KeyRemapUidSize:         true,
		KeyRemapUsers:           true,
		KeySetWorkingDirectory:  true,
		KeyUserNS:               true,
		KeyYaml:                 true,
	}

	// Supported keys in "Image" group
	supportedImageKeys = map[string]bool{
		KeyAllTags:              true,
		KeyArch:                 true,
		KeyAuthFile:             true,
		KeyCertDir:              true,
		KeyContainersConfModule: true,
		KeyCreds:                true,
		KeyDecryptionKey:        true,
		KeyGlobalArgs:           true,
		KeyImage:                true,
		KeyImageTag:             true,
		KeyOS:                   true,
		KeyPodmanArgs:           true,
		KeyTLSVerify:            true,
		KeyVariant:              true,
	}

	supportedPodKeys = map[string]bool{
		KeyContainersConfModule: true,
		KeyGlobalArgs:           true,
		KeyNetwork:              true,
		KeyPodName:              true,
		KeyPodmanArgs:           true,
		KeyPublishPort:          true,
		KeyVolume:               true,
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
func ConvertContainer(container *parser.UnitFile, names map[string]string, isUser bool, podsInfoMap map[string]*PodInfo) (*parser.UnitFile, error) {
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

	// One image or rootfs must be specified for the container
	image, _ := container.Lookup(ContainerGroup, KeyImage)
	rootfs, _ := container.Lookup(ContainerGroup, KeyRootfs)
	if len(image) == 0 && len(rootfs) == 0 {
		return nil, fmt.Errorf("no Image or Rootfs key specified")
	}
	if len(image) > 0 && len(rootfs) > 0 {
		return nil, fmt.Errorf("the Image And Rootfs keys conflict can not be specified together")
	}

	if len(image) > 0 {
		var err error
		if image, err = handleImageSource(image, service, names); err != nil {
			return nil, err
		}
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

	// If conmon exited uncleanly it may not have removed the container, so
	// force it, -i makes it ignore non-existing files.
	serviceStopCmd := createBasePodmanCommand(container, ContainerGroup)
	serviceStopCmd.add("rm", "-v", "-f", "-i", "--cidfile=%t/%N.cid")
	service.AddCmdline(ServiceGroup, "ExecStop", serviceStopCmd.Args)
	// The ExecStopPost is needed when the main PID (i.e., conmon) gets killed.
	// In that case, ExecStop is not executed but *Post only.  If both are
	// fired in sequence, *Post will exit when detecting that the --cidfile
	// has already been removed by the previous `rm`..
	serviceStopCmd.Args[0] = fmt.Sprintf("-%s", serviceStopCmd.Args[0])
	service.AddCmdline(ServiceGroup, "ExecStopPost", serviceStopCmd.Args)

	podman := createBasePodmanCommand(container, ContainerGroup)

	podman.add("run")

	podman.addf("--name=%s", containerName)

	podman.add(
		// We store the container id so we can clean it up in case of failure
		"--cidfile=%t/%N.cid",

		// And replace any previous container with the same name, not fail
		"--replace",

		// On clean shutdown, remove container
		"--rm",
	)

	handleLogDriver(container, ContainerGroup, podman)

	// We delegate groups to the runtime
	service.Add(ServiceGroup, "Delegate", "yes")
	podman.add("--cgroups=split")

	timezone, ok := container.Lookup(ContainerGroup, KeyTimezone)
	if ok && len(timezone) > 0 {
		podman.addf("--tz=%s", timezone)
	}

	addNetworks(container, ContainerGroup, service, names, podman)

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
		notify, ok := container.Lookup(ContainerGroup, KeyNotify)
		switch {
		case ok && strings.EqualFold(notify, "healthy"):
			podman.add("--sdnotify=healthy")
		case container.LookupBooleanWithDefault(ContainerGroup, KeyNotify, false):
			podman.add("--sdnotify=container")
		default:
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

	securityLabelDisable := container.LookupBooleanWithDefault(ContainerGroup, KeySecurityLabelDisable, false)
	if securityLabelDisable {
		podman.add("--security-opt", "label:disable")
	}

	securityLabelNested := container.LookupBooleanWithDefault(ContainerGroup, KeySecurityLabelNested, false)
	if securityLabelNested {
		podman.add("--security-opt", "label:nested")
	}

	pidsLimit, ok := container.Lookup(ContainerGroup, KeyPidsLimit)
	if ok && len(pidsLimit) > 0 {
		podman.add("--pids-limit", pidsLimit)
	}

	securityLabelType, ok := container.Lookup(ContainerGroup, KeySecurityLabelType)
	if ok && len(securityLabelType) > 0 {
		podman.add("--security-opt", fmt.Sprintf("label=type:%s", securityLabelType))
	}

	securityLabelFileType, ok := container.Lookup(ContainerGroup, KeySecurityLabelFileType)
	if ok && len(securityLabelFileType) > 0 {
		podman.add("--security-opt", fmt.Sprintf("label=filetype:%s", securityLabelFileType))
	}

	securityLabelLevel, ok := container.Lookup(ContainerGroup, KeySecurityLabelLevel)
	if ok && len(securityLabelLevel) > 0 {
		podman.add("--security-opt", fmt.Sprintf("label=level:%s", securityLabelLevel))
	}

	ulimits := container.LookupAll(ContainerGroup, KeyUlimit)
	for _, ulimit := range ulimits {
		podman.add("--ulimit", ulimit)
	}

	// But allow overrides with AddCapability
	devices := container.LookupAllStrv(ContainerGroup, KeyAddDevice)
	for _, device := range devices {
		if device[0] == '-' {
			device = device[1:]
			_, err := os.Stat(strings.Split(device, ":")[0])
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
		podman.addf("--device=%s", device)
	}

	// Default to no higher level privileges or caps
	seccompProfile, hasSeccompProfile := container.Lookup(ContainerGroup, KeySeccompProfile)
	if hasSeccompProfile {
		podman.add("--security-opt", fmt.Sprintf("seccomp=%s", seccompProfile))
	}

	dns := container.LookupAll(ContainerGroup, KeyDNS)
	for _, ipAddr := range dns {
		podman.addf("--dns=%s", ipAddr)
	}

	dnsOptions := container.LookupAll(ContainerGroup, KeyDNSOption)
	for _, dnsOption := range dnsOptions {
		podman.addf("--dns-option=%s", dnsOption)
	}

	dnsSearches := container.LookupAll(ContainerGroup, KeyDNSSearch)
	for _, dnsSearch := range dnsSearches {
		podman.addf("--dns-search=%s", dnsSearch)
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

	shmSize, hasShmSize := container.Lookup(ContainerGroup, KeyShmSize)
	if hasShmSize {
		podman.addf("--shm-size=%s", shmSize)
	}

	entrypoint, hasEntrypoint := container.Lookup(ContainerGroup, KeyEntrypoint)
	if hasEntrypoint {
		podman.addf("--entrypoint=%s", entrypoint)
	}

	sysctl := container.LookupAllStrv(ContainerGroup, KeySysctl)
	for _, sysctlItem := range sysctl {
		podman.addf("--sysctl=%s", sysctlItem)
	}

	readOnly, ok := container.LookupBoolean(ContainerGroup, KeyReadOnly)
	if ok {
		podman.addBool("--read-only", readOnly)
	}

	if readOnlyTmpfs, ok := container.LookupBoolean(ContainerGroup, KeyReadOnlyTmpfs); ok {
		podman.addBool("--read-only-tmpfs", readOnlyTmpfs)
	}

	volatileTmp := container.LookupBooleanWithDefault(ContainerGroup, KeyVolatileTmp, false)
	if volatileTmp && !readOnly {
		podman.add("--tmpfs", "/tmp:rw,size=512M,mode=1777")
	}

	if err := handleUser(container, ContainerGroup, podman); err != nil {
		return nil, err
	}

	if workdir, exists := container.Lookup(ContainerGroup, KeyWorkingDir); exists {
		podman.addf("-w=%s", workdir)
	}

	if err := handleUserMappings(container, ContainerGroup, podman, isUser, true); err != nil {
		return nil, err
	}

	tmpfsValues := container.LookupAll(ContainerGroup, KeyTmpfs)
	for _, tmpfs := range tmpfsValues {
		if strings.Count(tmpfs, ":") > 1 {
			return nil, fmt.Errorf("invalid tmpfs format '%s'", tmpfs)
		}

		podman.add("--tmpfs", tmpfs)
	}

	if err := addVolumes(container, service, ContainerGroup, names, podman); err != nil {
		return nil, err
	}

	update, ok := container.Lookup(ContainerGroup, KeyAutoUpdate)
	if ok && len(update) > 0 {
		podman.addLabels(map[string]string{
			autoUpdateLabel: update,
		})
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

	ip, ok := container.Lookup(ContainerGroup, KeyIP)
	if ok && len(ip) > 0 {
		podman.add("--ip", ip)
	}

	ip6, ok := container.Lookup(ContainerGroup, KeyIP6)
	if ok && len(ip6) > 0 {
		podman.add("--ip6", ip6)
	}

	labels := container.LookupAllKeyVal(ContainerGroup, KeyLabel)
	podman.addLabels(labels)

	annotations := container.LookupAllKeyVal(ContainerGroup, KeyAnnotation)
	podman.addAnnotations(annotations)

	masks := container.LookupAllArgs(ContainerGroup, KeyMask)
	for _, mask := range masks {
		podman.add("--security-opt", fmt.Sprintf("mask=%s", mask))
	}

	unmasks := container.LookupAllArgs(ContainerGroup, KeyUnmask)
	for _, unmask := range unmasks {
		podman.add("--security-opt", fmt.Sprintf("unmask=%s", unmask))
	}

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

	secrets := container.LookupAllArgs(ContainerGroup, KeySecret)
	for _, secret := range secrets {
		podman.add("--secret", secret)
	}

	mounts := container.LookupAllArgs(ContainerGroup, KeyMount)
	for _, mount := range mounts {
		mountStr, err := resolveContainerMountParams(container, service, mount, names)
		if err != nil {
			return nil, err
		}
		podman.add("--mount", mountStr)
	}

	handleHealth(container, ContainerGroup, podman)

	if hostname, ok := container.Lookup(ContainerGroup, KeyHostName); ok {
		podman.add("--hostname", hostname)
	}

	pull, ok := container.Lookup(ContainerGroup, KeyPull)
	if ok && len(pull) > 0 {
		podman.add("--pull", pull)
	}

	if err := handlePod(container, service, ContainerGroup, podsInfoMap, podman); err != nil {
		return nil, err
	}

	if stopTimeout, ok := container.Lookup(ContainerGroup, KeyStopTimeout); ok && len(stopTimeout) > 0 {
		podman.add("--stop-timeout", stopTimeout)
	}

	handlePodmanArgs(container, ContainerGroup, podman)

	if len(image) > 0 {
		podman.add(image)
	} else {
		podman.add("--rootfs", rootfs)
	}

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
// Also returns the canonical network name, either auto-generated or user-defined via the
// NetworkName key-value.
func ConvertNetwork(network *parser.UnitFile, name string) (*parser.UnitFile, string, error) {
	service := network.Dup()
	service.Filename = replaceExtension(network.Filename, ".service", "", "-network")

	if err := checkForUnknownKeys(network, NetworkGroup, supportedNetworkKeys); err != nil {
		return nil, "", err
	}

	/* Rename old Network group to x-Network so that systemd ignores it */
	service.RenameGroup(NetworkGroup, XNetworkGroup)

	// Derive network name from unit name (with added prefix), or use user-provided name.
	networkName, ok := network.Lookup(NetworkGroup, KeyNetworkName)
	if !ok || len(networkName) == 0 {
		networkName = replaceExtension(name, "", "systemd-", "")
	}

	// Need the containers filesystem mounted to start podman
	service.Add(UnitGroup, "RequiresMountsFor", "%t/containers")

	podman := createBasePodmanCommand(network, NetworkGroup)

	podman.add("network", "create", "--ignore")

	if disableDNS := network.LookupBooleanWithDefault(NetworkGroup, KeyDisableDNS, false); disableDNS {
		podman.add("--disable-dns")
	}

	dns := network.LookupAll(NetworkGroup, KeyDNS)
	for _, ipAddr := range dns {
		podman.addf("--dns=%s", ipAddr)
	}

	driver, ok := network.Lookup(NetworkGroup, KeyDriver)
	if ok && len(driver) > 0 {
		podman.addf("--driver=%s", driver)
	}

	subnets := network.LookupAll(NetworkGroup, KeySubnet)
	gateways := network.LookupAll(NetworkGroup, KeyGateway)
	ipRanges := network.LookupAll(NetworkGroup, KeyIPRange)
	if len(subnets) > 0 {
		if len(gateways) > len(subnets) {
			return nil, "", fmt.Errorf("cannot set more gateways than subnets")
		}
		if len(ipRanges) > len(subnets) {
			return nil, "", fmt.Errorf("cannot set more ranges than subnets")
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
		return nil, "", fmt.Errorf("cannot set gateway or range without subnet")
	}

	if internal := network.LookupBooleanWithDefault(NetworkGroup, KeyInternal, false); internal {
		podman.add("--internal")
	}

	if ipamDriver, ok := network.Lookup(NetworkGroup, KeyIPAMDriver); ok && len(ipamDriver) > 0 {
		podman.addf("--ipam-driver=%s", ipamDriver)
	}

	if ipv6 := network.LookupBooleanWithDefault(NetworkGroup, KeyIPv6, false); ipv6 {
		podman.add("--ipv6")
	}

	networkOptions := network.LookupAllKeyVal(NetworkGroup, KeyOptions)
	if len(networkOptions) > 0 {
		podman.addKeys("--opt", networkOptions)
	}

	if labels := network.LookupAllKeyVal(NetworkGroup, KeyLabel); len(labels) > 0 {
		podman.addLabels(labels)
	}

	handlePodmanArgs(network, NetworkGroup, podman)

	podman.add(networkName)

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	service.Setv(ServiceGroup,
		"Type", "oneshot",
		"RemainAfterExit", "yes",

		// The default syslog identifier is the exec basename (podman) which isn't very useful here
		"SyslogIdentifier", "%N")

	return service, networkName, nil
}

// Convert a quadlet volume file (unit file with a Volume group) to a systemd
// service file (unit file with Service group) based on the options in the
// Volume group.
// The original Volume group is kept around as X-Volume.
// Also returns the canonical volume name, either auto-generated or user-defined via the VolumeName
// key-value.
func ConvertVolume(volume *parser.UnitFile, name string, names map[string]string) (*parser.UnitFile, string, error) {
	service := volume.Dup()
	service.Filename = replaceExtension(volume.Filename, ".service", "", "-volume")

	if err := checkForUnknownKeys(volume, VolumeGroup, supportedVolumeKeys); err != nil {
		return nil, "", err
	}

	/* Rename old Volume group to x-Volume so that systemd ignores it */
	service.RenameGroup(VolumeGroup, XVolumeGroup)

	// Derive volume name from unit name (with added prefix), or use user-provided name.
	volumeName, ok := volume.Lookup(VolumeGroup, KeyVolumeName)
	if !ok || len(volumeName) == 0 {
		volumeName = replaceExtension(name, "", "systemd-", "")
	}

	// Need the containers filesystem mounted to start podman
	service.Add(UnitGroup, "RequiresMountsFor", "%t/containers")

	labels := volume.LookupAllKeyVal(VolumeGroup, "Label")

	podman := createBasePodmanCommand(volume, VolumeGroup)

	podman.add("volume", "create", "--ignore")

	driver, ok := volume.Lookup(VolumeGroup, KeyDriver)
	if ok {
		podman.addf("--driver=%s", driver)
	}

	var opts strings.Builder

	if driver == "image" {
		opts.WriteString("image=")

		imageName, ok := volume.Lookup(VolumeGroup, KeyImage)
		if !ok {
			return nil, "", fmt.Errorf("the key %s is mandatory when using the image driver", KeyImage)
		}
		imageName, err := handleImageSource(imageName, service, names)
		if err != nil {
			return nil, "", err
		}

		opts.WriteString(imageName)
	} else {
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

		copy, ok := volume.LookupBoolean(VolumeGroup, KeyCopy)
		if ok {
			if copy {
				podman.add("--opt", "copy")
			} else {
				podman.add("--opt", "nocopy")
			}
		}

		devValid := false

		dev, ok := volume.Lookup(VolumeGroup, KeyDevice)
		if ok && len(dev) != 0 {
			podman.add("--opt", fmt.Sprintf("device=%s", dev))
			devValid = true
		}

		devType, ok := volume.Lookup(VolumeGroup, KeyType)
		if ok && len(devType) != 0 {
			if devValid {
				podman.add("--opt", fmt.Sprintf("type=%s", devType))
			} else {
				return nil, "", fmt.Errorf("key Type can't be used without Device")
			}
		}

		mountOpts, ok := volume.Lookup(VolumeGroup, KeyOptions)
		if ok && len(mountOpts) != 0 {
			if devValid {
				if opts.Len() > 2 {
					opts.WriteString(",")
				}
				opts.WriteString(mountOpts)
			} else {
				return nil, "", fmt.Errorf("key Options can't be used without Device")
			}
		}
	}

	if opts.Len() > 2 {
		podman.add("--opt", opts.String())
	}

	podman.addLabels(labels)

	handlePodmanArgs(volume, VolumeGroup, podman)

	podman.add(volumeName)

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	service.Setv(ServiceGroup,
		"Type", "oneshot",
		"RemainAfterExit", "yes",

		// The default syslog identifier is the exec basename (podman) which isn't very useful here
		"SyslogIdentifier", "%N")

	return service, volumeName, nil
}

func ConvertKube(kube *parser.UnitFile, names map[string]string, isUser bool) (*parser.UnitFile, error) {
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

	// Allow users to set the Service Type to oneshot to allow resources only kube yaml
	serviceType, ok := service.Lookup(ServiceGroup, "Type")
	if ok && serviceType != "notify" && serviceType != "oneshot" {
		return nil, fmt.Errorf("invalid service Type '%s'", serviceType)
	}

	if serviceType != "oneshot" {
		service.Setv(ServiceGroup,
			"Type", "notify",
			"NotifyAccess", "all")
	}

	if !kube.HasKey(ServiceGroup, "SyslogIdentifier") {
		service.Set(ServiceGroup, "SyslogIdentifier", "%N")
	}

	execStart := createBasePodmanCommand(kube, KubeGroup)

	execStart.add("kube", "play")

	execStart.add(
		// Replace any previous container with the same name, not fail
		"--replace",

		// Use a service container
		"--service-container=true",
	)

	if ecp, ok := kube.Lookup(KubeGroup, KeyExitCodePropagation); ok && len(ecp) > 0 {
		execStart.addf("--service-exit-code-propagation=%s", ecp)
	}

	handleLogDriver(kube, KubeGroup, execStart)

	if err := handleUserMappings(kube, KubeGroup, execStart, isUser, false); err != nil {
		return nil, err
	}

	addNetworks(kube, KubeGroup, service, names, execStart)

	updateMaps := kube.LookupAllStrv(KubeGroup, KeyAutoUpdate)
	for _, update := range updateMaps {
		annotation := fmt.Sprintf("--annotation=%s", autoUpdateLabel)
		updateType := update
		annoValue, typ, hasSlash := strings.Cut(update, "/")
		if hasSlash {
			annotation = annotation + "/" + annoValue
			updateType = typ
		}
		execStart.addf("%s=%s", annotation, updateType)
	}

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

	handlePodmanArgs(kube, KubeGroup, execStart)

	execStart.add(yamlPath)

	service.AddCmdline(ServiceGroup, "ExecStart", execStart.Args)

	// Use `ExecStopPost` to make sure cleanup happens even in case of
	// errors; otherwise containers, pods, etc. would be left behind.
	execStop := createBasePodmanCommand(kube, KubeGroup)

	execStop.add("kube", "down")

	if kubeDownForce, ok := kube.LookupBoolean(KubeGroup, KeyKubeDownForce); ok {
		execStop.addBool("--force", kubeDownForce)
	}

	execStop.add(yamlPath)
	service.AddCmdline(ServiceGroup, "ExecStopPost", execStop.Args)

	err = handleSetWorkingDirectory(kube, service)
	if err != nil {
		return nil, err
	}

	return service, nil
}

func ConvertImage(image *parser.UnitFile) (*parser.UnitFile, string, error) {
	service := image.Dup()
	service.Filename = replaceExtension(image.Filename, ".service", "", "-image")

	if image.Path != "" {
		service.Add(UnitGroup, "SourcePath", image.Path)
	}

	if err := checkForUnknownKeys(image, ImageGroup, supportedImageKeys); err != nil {
		return nil, "", err
	}

	imageName, ok := image.Lookup(ImageGroup, KeyImage)
	if !ok || len(imageName) == 0 {
		return nil, "", fmt.Errorf("no Image key specified")
	}

	/* Rename old Network group to x-Network so that systemd ignores it */
	service.RenameGroup(ImageGroup, XImageGroup)

	// Need the containers filesystem mounted to start podman
	service.Add(UnitGroup, "RequiresMountsFor", "%t/containers")

	podman := createBasePodmanCommand(image, ImageGroup)

	podman.add("image", "pull")

	stringKeys := map[string]string{
		KeyArch:          "--arch",
		KeyAuthFile:      "--authfile",
		KeyCertDir:       "--cert-dir",
		KeyCreds:         "--creds",
		KeyDecryptionKey: "--decryption-key",
		KeyOS:            "--os",
		KeyVariant:       "--variant",
	}

	boolKeys := map[string]string{
		KeyAllTags:   "--all-tags",
		KeyTLSVerify: "--tls-verify",
	}

	for key, flag := range stringKeys {
		lookupAndAddString(image, ImageGroup, key, flag, podman)
	}

	for key, flag := range boolKeys {
		lookupAndAddBoolean(image, ImageGroup, key, flag, podman)
	}

	handlePodmanArgs(image, ImageGroup, podman)

	podman.add(imageName)

	service.AddCmdline(ServiceGroup, "ExecStart", podman.Args)

	service.Setv(ServiceGroup,
		"Type", "oneshot",
		"RemainAfterExit", "yes",

		// The default syslog identifier is the exec basename (podman) which isn't very useful here
		"SyslogIdentifier", "%N")

	if name, ok := image.Lookup(ImageGroup, KeyImageTag); ok && len(name) > 0 {
		imageName = name
	}

	return service, imageName, nil
}

func GetPodServiceName(podUnit *parser.UnitFile) string {
	return replaceExtension(podUnit.Filename, "", "", "-pod")
}

func ConvertPod(podUnit *parser.UnitFile, name string, podsInfoMap map[string]*PodInfo, names map[string]string) (*parser.UnitFile, error) {
	podInfo, ok := podsInfoMap[podUnit.Filename]
	if !ok {
		return nil, fmt.Errorf("internal error while processing pod %s", podUnit.Filename)
	}

	service := podUnit.Dup()
	service.Filename = fmt.Sprintf("%s.service", podInfo.ServiceName)

	if podUnit.Path != "" {
		service.Add(UnitGroup, "SourcePath", podUnit.Path)
	}

	if err := checkForUnknownKeys(podUnit, PodGroup, supportedPodKeys); err != nil {
		return nil, err
	}

	// Derive pod name from unit name (with added prefix), or use user-provided name.
	podName, ok := podUnit.Lookup(PodGroup, KeyPodName)
	if !ok || len(podName) == 0 {
		podName = replaceExtension(name, "", "systemd-", "")
	}

	/* Rename old Pod group to x-Pod so that systemd ignores it */
	service.RenameGroup(PodGroup, XPodGroup)

	// Need the containers filesystem mounted to start podman
	service.Add(UnitGroup, "RequiresMountsFor", "%t/containers")

	for _, containerService := range podInfo.Containers {
		service.Add(UnitGroup, "Wants", containerService)
		service.Add(UnitGroup, "Before", containerService)
	}

	if !podUnit.HasKey(ServiceGroup, "SyslogIdentifier") {
		service.Set(ServiceGroup, "SyslogIdentifier", "%N")
	}

	execStart := createBasePodmanCommand(podUnit, PodGroup)
	execStart.add("pod", "start", "--pod-id-file=%t/%N.pod-id")
	service.AddCmdline(ServiceGroup, "ExecStart", execStart.Args)

	execStop := createBasePodmanCommand(podUnit, PodGroup)
	execStop.add("pod", "stop")
	execStop.add(
		"--pod-id-file=%t/%N.pod-id",
		"--ignore",
		"--time=10",
	)
	service.AddCmdline(ServiceGroup, "ExecStop", execStop.Args)

	execStopPost := createBasePodmanCommand(podUnit, PodGroup)
	execStopPost.add("pod", "rm")
	execStopPost.add(
		"--pod-id-file=%t/%N.pod-id",
		"--ignore",
		"--force",
	)
	service.AddCmdline(ServiceGroup, "ExecStopPost", execStopPost.Args)

	execStartPre := createBasePodmanCommand(podUnit, PodGroup)
	execStartPre.add("pod", "create")
	execStartPre.add(
		"--infra-conmon-pidfile=%t/%N.pid",
		"--pod-id-file=%t/%N.pod-id",
		"--exit-policy=stop",
		"--replace",
	)

	if err := handlePublishPorts(podUnit, PodGroup, execStartPre); err != nil {
		return nil, err
	}

	addNetworks(podUnit, PodGroup, service, names, execStartPre)

	if err := addVolumes(podUnit, service, PodGroup, names, execStartPre); err != nil {
		return nil, err
	}

	execStartPre.addf("--name=%s", podName)

	handlePodmanArgs(podUnit, PodGroup, execStartPre)

	service.AddCmdline(ServiceGroup, "ExecStartPre", execStartPre.Args)

	service.Setv(ServiceGroup,
		"Environment", "PODMAN_SYSTEMD_UNIT=%n",
		"Type", "forking",
		"Restart", "on-failure",
		"PIDFile", "%t/%N.pid",
	)

	return service, nil
}

func handleUser(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) error {
	user, hasUser := unitFile.Lookup(groupName, KeyUser)
	okUser := hasUser && len(user) > 0

	group, hasGroup := unitFile.Lookup(groupName, KeyGroup)
	okGroup := hasGroup && len(group) > 0

	if !okUser {
		if okGroup {
			return fmt.Errorf("invalid Group set without User")
		}
		return nil
	}

	if !okGroup {
		podman.add("--user", user)
		return nil
	}

	podman.addf("--user=%s:%s", user, group)

	return nil
}

func handleUserMappings(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline, isUser, supportManual bool) error {
	mappingsDefined := false

	if userns, ok := unitFile.Lookup(groupName, KeyUserNS); ok && len(userns) > 0 {
		podman.add("--userns", userns)
		mappingsDefined = true
	}

	uidMaps := unitFile.LookupAllStrv(groupName, KeyUIDMap)
	mappingsDefined = mappingsDefined || len(uidMaps) > 0
	for _, uidMap := range uidMaps {
		podman.addf("--uidmap=%s", uidMap)
	}

	gidMaps := unitFile.LookupAllStrv(groupName, KeyGIDMap)
	mappingsDefined = mappingsDefined || len(gidMaps) > 0
	for _, gidMap := range gidMaps {
		podman.addf("--gidmap=%s", gidMap)
	}

	if subUIDMap, ok := unitFile.Lookup(groupName, KeySubUIDMap); ok && len(subUIDMap) > 0 {
		podman.add("--subuidname", subUIDMap)
		mappingsDefined = true
	}

	if subGIDMap, ok := unitFile.Lookup(groupName, KeySubGIDMap); ok && len(subGIDMap) > 0 {
		podman.add("--subgidname", subGIDMap)
		mappingsDefined = true
	}

	if mappingsDefined {
		_, hasRemapUID := unitFile.Lookup(groupName, KeyRemapUid)
		_, hasRemapGID := unitFile.Lookup(groupName, KeyRemapGid)
		_, RemapUsers := unitFile.LookupLast(groupName, KeyRemapUsers)
		if hasRemapUID || hasRemapGID || RemapUsers {
			return fmt.Errorf("deprecated Remap keys are set along with explicit mapping keys")
		}
		return nil
	}

	return handleUserRemap(unitFile, groupName, podman, isUser, supportManual)
}

func handleUserRemap(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline, isUser, supportManual bool) error {
	uidMaps := unitFile.LookupAllStrv(groupName, KeyRemapUid)
	gidMaps := unitFile.LookupAllStrv(groupName, KeyRemapGid)
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
		uidSize := unitFile.LookupUint32(groupName, KeyRemapUidSize, 0)
		if uidSize > 0 {
			autoOpts = append(autoOpts, fmt.Sprintf("size=%v", uidSize))
		}

		podman.addf("--userns=" + usernsOpts("auto", autoOpts))
	case "keep-id":
		if !isUser {
			return fmt.Errorf("RemapUsers=keep-id is unsupported for system units")
		}

		keepidOpts := make([]string, 0)
		if len(uidMaps) > 0 {
			if len(uidMaps) > 1 {
				return fmt.Errorf("RemapUsers=keep-id supports only a single value for UID mapping")
			}
			keepidOpts = append(keepidOpts, "uid="+uidMaps[0])
		}
		if len(gidMaps) > 0 {
			if len(gidMaps) > 1 {
				return fmt.Errorf("RemapUsers=keep-id supports only a single value for GID mapping")
			}
			keepidOpts = append(keepidOpts, "gid="+gidMaps[0])
		}

		podman.addf("--userns=" + usernsOpts("keep-id", keepidOpts))

	default:
		return fmt.Errorf("unsupported RemapUsers option '%s'", remapUsers)
	}

	return nil
}

func addNetworks(quadletUnitFile *parser.UnitFile, groupName string, serviceUnitFile *parser.UnitFile, names map[string]string, podman *PodmanCmdline) {
	networks := quadletUnitFile.LookupAll(groupName, KeyNetwork)
	for _, network := range networks {
		if len(network) > 0 {
			quadletNetworkName, options, found := strings.Cut(network, ":")
			if strings.HasSuffix(quadletNetworkName, ".network") {
				// the podman network name is systemd-$name if none is specified by the user.
				networkName := names[quadletNetworkName]
				if networkName == "" {
					networkName = replaceExtension(quadletNetworkName, "", "systemd-", "")
				}

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

// Systemd Specifiers start with % with the exception of %%
func startsWithSystemdSpecifier(filePath string) bool {
	if len(filePath) == 0 || filePath[0] != '%' {
		return false
	}

	if len(filePath) > 1 && filePath[1] == '%' {
		return false
	}

	return true
}

func getAbsolutePath(quadletUnitFile *parser.UnitFile, filePath string) (string, error) {
	// When the path starts with a Systemd specifier do not resolve what looks like a relative address
	if !startsWithSystemdSpecifier(filePath) && !filepath.IsAbs(filePath) {
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

func handleLogDriver(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) {
	logDriver, found := unitFile.Lookup(groupName, KeyLogDriver)
	if found {
		podman.add("--log-driver", logDriver)
	}
}

func handleStorageSource(quadletUnitFile, serviceUnitFile *parser.UnitFile, source string, names map[string]string) (string, error) {
	if source[0] == '.' {
		var err error
		source, err = getAbsolutePath(quadletUnitFile, source)
		if err != nil {
			return "", err
		}
	}
	if source[0] == '/' {
		// Absolute path
		serviceUnitFile.Add(UnitGroup, "RequiresMountsFor", source)
	} else if strings.HasSuffix(source, ".volume") {
		// the podman volume name is systemd-$name if none has been provided by the user.
		volumeName := names[source]
		if volumeName == "" {
			volumeName = replaceExtension(source, "", "systemd-", "")
		}

		// the systemd unit name is $name-volume.service
		volumeServiceName := replaceExtension(source, ".service", "", "-volume")

		source = volumeName

		serviceUnitFile.Add(UnitGroup, "Requires", volumeServiceName)
		serviceUnitFile.Add(UnitGroup, "After", volumeServiceName)
	}

	return source, nil
}

func handleHealth(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) {
	keyArgMap := [][2]string{
		{KeyHealthCmd, "cmd"},
		{KeyHealthInterval, "interval"},
		{KeyHealthOnFailure, "on-failure"},
		{KeyHealthRetries, "retries"},
		{KeyHealthStartPeriod, "start-period"},
		{KeyHealthTimeout, "timeout"},
		{KeyHealthStartupCmd, "startup-cmd"},
		{KeyHealthStartupInterval, "startup-interval"},
		{KeyHealthStartupRetries, "startup-retries"},
		{KeyHealthStartupSuccess, "startup-success"},
		{KeyHealthStartupTimeout, "startup-timeout"},
	}

	for _, keyArg := range keyArgMap {
		val, found := unitFile.Lookup(groupName, keyArg[0])
		if found && len(val) > 0 {
			podman.addf("--health-%s", keyArg[1])
			podman.addf("%s", val)
		}
	}
}

func handlePodmanArgs(unitFile *parser.UnitFile, groupName string, podman *PodmanCmdline) {
	podmanArgs := unitFile.LookupAllArgs(groupName, KeyPodmanArgs)
	if len(podmanArgs) > 0 {
		podman.add(podmanArgs...)
	}
}

func handleSetWorkingDirectory(kube, serviceUnitFile *parser.UnitFile) error {
	// If WorkingDirectory is already set in the Service section do not change it
	workingDir, ok := kube.Lookup(ServiceGroup, ServiceKeyWorkingDirectory)
	if ok && len(workingDir) > 0 {
		return nil
	}

	setWorkingDirectory, ok := kube.Lookup(KubeGroup, KeySetWorkingDirectory)
	if !ok || len(setWorkingDirectory) == 0 {
		return nil
	}

	var relativeToFile string
	switch strings.ToLower(setWorkingDirectory) {
	case "yaml":
		relativeToFile, ok = kube.Lookup(KubeGroup, KeyYaml)
		if !ok {
			return fmt.Errorf("no Yaml key specified")
		}
	case "unit":
		relativeToFile = kube.Path
	default:
		return fmt.Errorf("unsupported value for %s: %s ", ServiceKeyWorkingDirectory, setWorkingDirectory)
	}

	fileInWorkingDir, err := getAbsolutePath(kube, relativeToFile)
	if err != nil {
		return err
	}

	serviceUnitFile.Add(ServiceGroup, ServiceKeyWorkingDirectory, filepath.Dir(fileInWorkingDir))

	return nil
}

func lookupAndAddString(unit *parser.UnitFile, group, key, flag string, podman *PodmanCmdline) {
	val, ok := unit.Lookup(group, key)
	if ok && len(val) > 0 {
		podman.addf("%s=%s", flag, val)
	}
}

func lookupAndAddBoolean(unit *parser.UnitFile, group, key, flag string, podman *PodmanCmdline) {
	val, ok := unit.LookupBoolean(group, key)
	if ok {
		podman.addBool(flag, val)
	}
}

func handleImageSource(quadletImageName string, serviceUnitFile *parser.UnitFile, names map[string]string) (string, error) {
	if strings.HasSuffix(quadletImageName, ".image") {
		// since there is no default name conversion, the actual image name must exist in the names map
		imageName, ok := names[quadletImageName]
		if !ok {
			return "", fmt.Errorf("requested Quadlet image %s was not found", imageName)
		}

		// the systemd unit name is $name-image.service
		imageServiceName := replaceExtension(quadletImageName, ".service", "", "-image")

		serviceUnitFile.Add(UnitGroup, "Requires", imageServiceName)
		serviceUnitFile.Add(UnitGroup, "After", imageServiceName)

		quadletImageName = imageName
	}

	return quadletImageName, nil
}

func resolveContainerMountParams(containerUnitFile, serviceUnitFile *parser.UnitFile, mount string, names map[string]string) (string, error) {
	mountType, tokens, err := specgenutilexternal.FindMountType(mount)
	if err != nil {
		return "", err
	}

	// Source resolution is required only for these types of mounts
	if !(mountType == "volume" || mountType == "bind" || mountType == "glob") {
		return mount, nil
	}

	sourceIndex := -1
	originalSource := ""
	for i, token := range tokens {
		key, val, hasVal := strings.Cut(token, "=")
		if key == "source" || key == "src" {
			if !hasVal {
				return "", fmt.Errorf("source parameter does not include a value")
			}
			sourceIndex = i
			originalSource = val
		}
	}

	resolvedSource, err := handleStorageSource(containerUnitFile, serviceUnitFile, originalSource, names)
	if err != nil {
		return "", err
	}
	tokens[sourceIndex] = fmt.Sprintf("source=%s", resolvedSource)

	tokens = append([]string{fmt.Sprintf("type=%s", mountType)}, tokens...)

	return convertToCSV(tokens)
}

func convertToCSV(s []string) (string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	err := writer.Write(s)
	if err != nil {
		return "", err
	}
	writer.Flush()

	ret := buf.String()
	if ret[len(ret)-1] == '\n' {
		ret = ret[:len(ret)-1]
	}

	return ret, nil
}

func createBasePodmanCommand(unitFile *parser.UnitFile, groupName string) *PodmanCmdline {
	podman := NewPodmanCmdline()

	containersConfModules := unitFile.LookupAll(groupName, KeyContainersConfModule)
	for _, containersConfModule := range containersConfModules {
		podman.addf("--module=%s", containersConfModule)
	}

	globalArgs := unitFile.LookupAllArgs(groupName, KeyGlobalArgs)
	if len(globalArgs) > 0 {
		podman.add(globalArgs...)
	}

	return podman
}

func handlePod(quadletUnitFile, serviceUnitFile *parser.UnitFile, groupName string, podsInfoMap map[string]*PodInfo, podman *PodmanCmdline) error {
	pod, ok := quadletUnitFile.Lookup(groupName, KeyPod)
	if ok && len(pod) > 0 {
		if !strings.HasSuffix(pod, ".pod") {
			return fmt.Errorf("pod %s is not Quadlet based", pod)
		}

		podInfo, ok := podsInfoMap[pod]
		if !ok {
			return fmt.Errorf("quadlet pod unit %s does not exist", pod)
		}

		podman.add("--pod-id-file", fmt.Sprintf("%%t/%s.pod-id", podInfo.ServiceName))

		podServiceName := fmt.Sprintf("%s.service", podInfo.ServiceName)
		serviceUnitFile.Add(UnitGroup, "BindsTo", podServiceName)
		serviceUnitFile.Add(UnitGroup, "After", podServiceName)

		podInfo.Containers = append(podInfo.Containers, serviceUnitFile.Filename)
	}
	return nil
}

func addVolumes(quadletUnitFile, serviceUnitFile *parser.UnitFile, groupName string, names map[string]string, podman *PodmanCmdline) error {
	volumes := quadletUnitFile.LookupAll(groupName, KeyVolume)
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
			var err error
			source, err = handleStorageSource(quadletUnitFile, serviceUnitFile, source, names)
			if err != nil {
				return err
			}
		}

		podman.add("-v")
		if source == "" {
			podman.add(dest)
		} else {
			podman.addf("%s:%s%s", source, dest, options)
		}
	}

	return nil
}
