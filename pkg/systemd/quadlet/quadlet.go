package quadlet

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/containers/podman/v4/pkg/systemd/parser"
)

// Overwritten at build time:
var (
	QuadletUserName = "quadlet" // Name of user used to look up subuid/subgid for remap uids
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

	// Fallbacks uid/gid ranges if the above username doesn't exist or has no subuids
	FallbackUIDStart  = 1879048192
	FallbackUIDLength = 165536
	FallbackGIDStart  = 1879048192
	FallbackGIDLength = 165536
)

var validPortRange = regexp.MustCompile(`\d+(-\d+)?(/udp|/tcp)?$`)

// All the supported quadlet keys
const (
	KeyContainerName   = "ContainerName"
	KeyImage           = "Image"
	KeyEnvironment     = "Environment"
	KeyExec            = "Exec"
	KeyNoNewPrivileges = "NoNewPrivileges"
	KeyDropCapability  = "DropCapability"
	KeyAddCapability   = "AddCapability"
	KeyReadOnly        = "ReadOnly"
	KeyRemapUsers      = "RemapUsers"
	KeyRemapUIDStart   = "RemapUidStart"
	KeyRemapGIDStart   = "RemapGidStart"
	KeyRemapUIDRanges  = "RemapUidRanges"
	KeyRemapGIDRanges  = "RemapGidRanges"
	KeyNotify          = "Notify"
	KeyExposeHostPort  = "ExposeHostPort"
	KeyPublishPort     = "PublishPort"
	KeyKeepID          = "KeepId"
	KeyUser            = "User"
	KeyGroup           = "Group"
	KeyHostUser        = "HostUser"
	KeyHostGroup       = "HostGroup"
	KeyVolume          = "Volume"
	KeyPodmanArgs      = "PodmanArgs"
	KeyLabel           = "Label"
	KeyAnnotation      = "Annotation"
	KeyRunInit         = "RunInit"
	KeyVolatileTmp     = "VolatileTmp"
	KeyTimezone        = "Timezone"
	KeySeccompProfile  = "SeccompProfile"
)

// Supported keys in "Container" group
var supportedContainerKeys = map[string]bool{
	KeyContainerName:   true,
	KeyImage:           true,
	KeyEnvironment:     true,
	KeyExec:            true,
	KeyNoNewPrivileges: true,
	KeyDropCapability:  true,
	KeyAddCapability:   true,
	KeyReadOnly:        true,
	KeyRemapUsers:      true,
	KeyRemapUIDStart:   true,
	KeyRemapGIDStart:   true,
	KeyRemapUIDRanges:  true,
	KeyRemapGIDRanges:  true,
	KeyNotify:          true,
	KeyExposeHostPort:  true,
	KeyPublishPort:     true,
	KeyKeepID:          true,
	KeyUser:            true,
	KeyGroup:           true,
	KeyHostUser:        true,
	KeyHostGroup:       true,
	KeyVolume:          true,
	KeyPodmanArgs:      true,
	KeyLabel:           true,
	KeyAnnotation:      true,
	KeyRunInit:         true,
	KeyVolatileTmp:     true,
	KeyTimezone:        true,
	KeySeccompProfile:  true,
}

// Supported keys in "Volume" group
var supportedVolumeKeys = map[string]bool{
	KeyUser:  true,
	KeyGroup: true,
	KeyLabel: true,
}

func replaceExtension(name string, extension string, extraPrefix string, extraSuffix string) string {
	baseName := name

	dot := strings.LastIndexByte(name, '.')
	if dot > 0 {
		baseName = name[:dot]
	}

	return extraPrefix + baseName + extraSuffix + extension
}

var defaultRemapUIDs, defaultRemapGIDs *Ranges

func getDefaultRemapUids() *Ranges {
	if defaultRemapUIDs == nil {
		defaultRemapUIDs = lookupHostSubuid(QuadletUserName)
		if defaultRemapUIDs == nil {
			defaultRemapUIDs =
				NewRanges(FallbackUIDStart, FallbackUIDLength)
		}
	}
	return defaultRemapUIDs
}

func getDefaultRemapGids() *Ranges {
	if defaultRemapGIDs == nil {
		defaultRemapGIDs = lookupHostSubgid(QuadletUserName)
		if defaultRemapGIDs == nil {
			defaultRemapGIDs =
				NewRanges(FallbackGIDStart, FallbackGIDLength)
		}
	}
	return defaultRemapGIDs
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

func lookupRanges(unit *parser.UnitFile, groupName string, key string, nameLookup func(string) *Ranges, defaultValue *Ranges) *Ranges {
	v, ok := unit.Lookup(groupName, key)
	if !ok {
		if defaultValue != nil {
			return defaultValue.Copy()
		}

		return NewRangesEmpty()
	}

	if len(v) == 0 {
		return NewRangesEmpty()
	}

	if !unicode.IsDigit(rune(v[0])) {
		if nameLookup != nil {
			r := nameLookup(v)
			if r != nil {
				return r
			}
		}
		return NewRangesEmpty()
	}

	return ParseRanges(v)
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

func addIDMaps(podman *PodmanCmdline, argPrefix string, containerID, hostID, remapStartID uint32, availableHostIDs *Ranges) {
	if availableHostIDs == nil {
		// Map everything by default
		availableHostIDs = NewRangesEmpty()
	}

	// Map the first ids up to remapStartID to the host equivalent
	unmappedIds := NewRanges(0, remapStartID)

	// The rest we want to map to availableHostIDs. Note that this
	// overlaps unmappedIds, because below we may remove ranges from
	// unmapped ids and we want to backfill those.
	mappedIds := NewRanges(0, math.MaxUint32)

	// Always map specified uid to specified host_uid
	podman.addIDMap(argPrefix, containerID, hostID, 1)

	// We no longer want to map this container id as its already mapped
	mappedIds.Remove(containerID, 1)
	unmappedIds.Remove(containerID, 1)

	// But also, we don't want to use the *host* id again, as we can only map it once
	unmappedIds.Remove(hostID, 1)
	availableHostIDs.Remove(hostID, 1)

	// Map unmapped ids to equivalent host range, and remove from mappedIds to avoid double-mapping
	for _, r := range unmappedIds.Ranges {
		start := r.Start
		length := r.Length

		podman.addIDMap(argPrefix, start, start, length)
		mappedIds.Remove(start, length)
		availableHostIDs.Remove(start, length)
	}

	for cIdx := 0; cIdx < len(mappedIds.Ranges) && len(availableHostIDs.Ranges) > 0; cIdx++ {
		cRange := &mappedIds.Ranges[cIdx]
		cStart := cRange.Start
		cLength := cRange.Length

		for cLength > 0 && len(availableHostIDs.Ranges) > 0 {
			hRange := &availableHostIDs.Ranges[0]
			hStart := hRange.Start
			hLength := hRange.Length

			nextLength := minUint32(hLength, cLength)

			podman.addIDMap(argPrefix, cStart, hStart, nextLength)
			availableHostIDs.Remove(hStart, nextLength)
			cStart += nextLength
			cLength -= nextLength
		}
	}
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

	// Remove any leftover cid file before starting, just to be sure.
	// We remove any actual pre-existing container by name with --replace=true.
	// But --cidfile will fail if the target exists.
	service.Add(ServiceGroup, "ExecStartPre", "-rm -f %t/%N.cid")

	// If the conman exited uncleanly it may not have removed the container, so force it,
	// -i makes it ignore non-existing files.
	service.Add(ServiceGroup, "ExecStopPost", "-/usr/bin/podman rm -f -i --cidfile=%t/%N.cid")

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

		// Detach from container, we don't need the podman process to hang around
		"-d",

		// But we still want output to the journal, so use the log driver.
		"--log-driver", "passthrough",

		// Never try to pull the image during service start
		"--pull=never")

	// We use crun as the runtime and delegated groups to it
	service.Add(ServiceGroup, "Delegate", "yes")
	podman.add(
		"--runtime", "/usr/bin/crun",
		"--cgroups=split")

	timezone, ok := container.Lookup(ContainerGroup, KeyTimezone)
	if ok && len(timezone) > 0 {
		podman.addf("--tz=%s", timezone)
	}

	// Run with a pid1 init to reap zombies by default (as most apps don't do that)
	runInit := container.LookupBoolean(ContainerGroup, KeyRunInit, true)
	if runInit {
		podman.add("--init")
	}

	// By default we handle startup notification with conmon, but allow passing it to the container with Notify=yes
	notify := container.LookupBoolean(ContainerGroup, KeyNotify, false)
	if notify {
		podman.add("--sdnotify=container")
	} else {
		podman.add("--sdnotify=conmon")
	}
	service.Setv(ServiceGroup,
		"Type", "notify",
		"NotifyAccess", "all")

	if !container.HasKey(ServiceGroup, "SyslogIdentifier") {
		service.Set(ServiceGroup, "SyslogIdentifier", "%N")
	}

	// Default to no higher level privileges or caps
	noNewPrivileges := container.LookupBoolean(ContainerGroup, KeyNoNewPrivileges, true)
	if noNewPrivileges {
		podman.add("--security-opt=no-new-privileges")
	}

	// Default to no higher level privileges or caps
	seccompProfile, hasSeccompProfile := container.Lookup(ContainerGroup, KeySeccompProfile)
	if hasSeccompProfile {
		podman.add("--security-opt", fmt.Sprintf("seccomp=%s", seccompProfile))
	}

	dropCaps := []string{"all"} // Default
	if container.HasKey(ContainerGroup, KeyDropCapability) {
		dropCaps = container.LookupAllStrv(ContainerGroup, KeyDropCapability)
	}

	for _, caps := range dropCaps {
		podman.addf("--cap-drop=%s", strings.ToLower(caps))
	}

	// But allow overrides with AddCapability
	addCaps := container.LookupAllStrv(ContainerGroup, KeyAddCapability)
	for _, caps := range addCaps {
		podman.addf("--cap-add=%s", strings.ToLower(caps))
	}

	readOnly := container.LookupBoolean(ContainerGroup, KeyReadOnly, true)
	if readOnly {
		podman.add("--read-only")
	}

	// We want /tmp to be a tmpfs, like on rhel host
	volatileTmp := container.LookupBoolean(ContainerGroup, KeyVolatileTmp, true)
	if volatileTmp {
		/* Read only mode already has a tmpfs by default */
		if !readOnly {
			podman.add("--tmpfs", "/tmp:rw,size=512M,mode=1777")
		}
	} else if readOnly {
		/* !volatileTmp, disable the default tmpfs from --read-only */
		podman.add("--read-only-tmpfs=false")
	}

	defaultContainerUID := uint32(0)
	defaultContainerGID := uint32(0)

	keepID := container.LookupBoolean(ContainerGroup, KeyKeepID, false)
	if keepID {
		if isUser {
			defaultContainerUID = uint32(os.Getuid())
			defaultContainerGID = uint32(os.Getgid())
			podman.add("--userns", "keep-id")
		} else {
			return nil, fmt.Errorf("key 'KeepId' in '%s' unsupported for system units", container.Path)
		}
	}

	uid := container.LookupUint32(ContainerGroup, KeyUser, defaultContainerUID)
	gid := container.LookupUint32(ContainerGroup, KeyGroup, defaultContainerGID)

	hostUID, err := container.LookupUID(ContainerGroup, KeyHostUser, uid)
	if err != nil {
		return nil, fmt.Errorf("key 'HostUser' invalid: %s", err)
	}

	hostGID, err := container.LookupGID(ContainerGroup, KeyHostGroup, gid)
	if err != nil {
		return nil, fmt.Errorf("key 'HostGroup' invalid: %s", err)
	}

	if uid != defaultContainerUID || gid != defaultContainerGID {
		podman.add("--user")
		if gid == defaultContainerGID {
			podman.addf("%d", uid)
		} else {
			podman.addf("%d:%d", uid, gid)
		}
	}

	var remapUsers bool
	if isUser {
		remapUsers = false
	} else {
		remapUsers = container.LookupBoolean(ContainerGroup, KeyRemapUsers, false)
	}

	if !remapUsers {
		// No remapping of users, although we still need maps if the
		//   main user/group is remapped, even if most ids map one-to-one.
		if uid != hostUID {
			addIDMaps(podman, "--uidmap", uid, hostUID, math.MaxUint32, nil)
		}
		if gid != hostGID {
			addIDMaps(podman, "--gidmap", gid, hostGID, math.MaxUint32, nil)
		}
	} else {
		uidRemapIDs := lookupRanges(container, ContainerGroup, KeyRemapUIDRanges, lookupHostSubuid, getDefaultRemapUids())
		gidRemapIDs := lookupRanges(container, ContainerGroup, KeyRemapGIDRanges, lookupHostSubgid, getDefaultRemapGids())
		remapUIDStart := container.LookupUint32(ContainerGroup, KeyRemapUIDStart, 1)
		remapGIDStart := container.LookupUint32(ContainerGroup, KeyRemapGIDStart, 1)

		addIDMaps(podman, "--uidmap", uid, hostUID, remapUIDStart, uidRemapIDs)
		addIDMaps(podman, "--gidmap", gid, hostGID, remapGIDStart, gidRemapIDs)
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

	publishPorts := container.LookupAll(ContainerGroup, KeyPublishPort)
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
			return nil, fmt.Errorf("invalid published port '%s'", publishPort)
		}

		if ip == "0.0.0.0" {
			ip = ""
		}

		if len(hostPort) > 0 && !isPortRange(hostPort) {
			return nil, fmt.Errorf("invalid port format '%s'", hostPort)
		}

		if len(containerPort) > 0 && !isPortRange(containerPort) {
			return nil, fmt.Errorf("invalid port format '%s'", containerPort)
		}

		switch {
		case len(ip) > 0 && len(hostPort) > 0:
			podman.addf("-p=%s:%s:%s", ip, hostPort, containerPort)
		case len(ip) > 0:
			podman.addf("-p=%s::%s", ip, containerPort)
		case len(hostPort) > 0:
			podman.addf("-p=%s:%s", hostPort, containerPort)
		default:
			podman.addf("-p=%s", containerPort)
		}
	}

	podman.addEnv(podmanEnv)

	labels := container.LookupAllKeyVal(ContainerGroup, KeyLabel)
	podman.addLabels(labels)

	annotations := container.LookupAllKeyVal(ContainerGroup, KeyAnnotation)
	podman.addAnnotations(annotations)

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

// Convert a quadlet volume file (unit file with a Volume group) to a systemd
// service file (unit file with Service group) based on the options in the
// Volume group.
// The original Container group is kept around as X-Container.
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

	execCond := fmt.Sprintf("/usr/bin/bash -c \"! /usr/bin/podman volume exists %s\"", volumeName)

	labels := volume.LookupAllKeyVal(VolumeGroup, "Label")

	podman := NewPodmanCmdline("volume", "create")

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
		"ExecCondition", execCond,

		// The default syslog identifier is the exec basename (podman) which isn't very useful here
		"SyslogIdentifier", "%N")

	return service, nil
}
