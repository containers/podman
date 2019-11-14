package specgen

import (
	"net"

	"github.com/containers/libpod/libpod"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// TODO
type Namespace struct {
}

// ContainerBasicConfig contains the basic parts of a container.
type ContainerBasicConfig struct {
	// Name is the name the container will be given.
	// If no name is provided, one will be randomly generated.
	// Optional.
	Name string
	// Pod is the ID of the pod the container will join.
	// Optional.
	Pod string
	// Entrypoint is the container's entrypoint.
	// If not given and Image is specified, this will be populated by the
	// image's configuration.
	// Optional.
	Entrypoint []string
	// Command is the container's command.
	// If not given and Image is specified, this will be populated by the
	// image's configuration.
	// Optional.
	Command []string
	// Env is a set of environment variables that will be set in the
	// container.
	// Optional.
	Env map[string]string
	// Terminal is whether the container will create a PTY.
	Terminal bool
	// Stdin is whether the container will keep its STDIN open.
	Stdin bool
	// Labels are key-valid labels that are used to add metadata to
	// containers.
	// Optional.
	Labels map[string]string
	// Annotations are key-value options passed into the container runtime
	// that can be used to trigger special behavior.
	// Optional.
	Annotations map[string]string
	// StopSignal is the signal that will be used to stop the container.
	// Must be a non-zero integer below SIGRTMAX.
	// If not provided, the default, SIGTERM, will be used.
	// Will conflict with Systemd if Systemd is set to "true" or "always".
	// Optional.
	StopSignal *uint
	// StopTimeout is a timeout between the container's stop signal being
	// sent and SIGKILL being sent.
	// If not provided, the default will be used.
	// If 0 is used, stop signal will not be sent, and SIGKILL will be sent
	// instead.
	// Optional.
	StopTimeout *uint
	// LogDriver is the container's log driver.
	// Optional.
	LogDriver string
	// LogPath is the path the container's logs will be stored at.
	// Only available if LogDriver is set to "json-file" or "k8s-file".
	// Optional.
	LogPath string
	// ConmonPidFile is a path at which a PID file for Conmon will be
	// placed.
	// If not given, a default location will be used.
	// Optional.
	ConmonPidFile string
	// RestartPolicy is the container's restart policy - an action which
	// will be taken when the container exits.
	// If not given, the default policy, which does nothing, will be used.
	// Optional.
	RestartPolicy string
	// RestartRetries is the number of attempts that will be made to restart
	// the container.
	// Only available when RestartPolicy is set to "on-failure".
	// Optional.
	RestartRetries *uint
	// OCIRuntime is the name of the OCI runtime that will be used to create
	// the container.
	// If not specified, the default will be used.
	// Optional.
	OCIRuntime string
	// Systemd is whether the container will be started in systemd mode.
	// Valid options are "true", "false", and "always".
	// "true" enables this mode only if the binary run in the container is
	// /sbin/init or systemd. "always" unconditionally enables systemd mode.
	// "false" unconditionally disables systemd mode.
	// If enabled, mounts and stop signal will be modified.
	// If set to "always" or set to "true" and conditionally triggered,
	// conflicts with StopSignal.
	// If not specified, "false" will be assumed.
	// Optional.
	Systemd string
	// Namespace is the libpod namespace the container will be placed in.
	// Optional.
	Namespace string

	// PidNS is the container's PID namespace.
	// It defaults to private.
	// Mandatory.
	PidNS Namespace

	// UtsNS is the container's UTS namespace.
	// It defaults to private.
	// Must be set to Private to set Hostname.
	// Mandatory.
	UtsNS Namespace
	// Hostname is the container's hostname. If not set, the hostname will
	// not be modified (if UtsNS is not private) or will be set to the
	// container ID (if UtsNS is private).
	// Conflicts with UtsNS if UtsNS is not set to private.
	// Optional.
	Hostname string
}

// ContainerStorageConfig contains information on the storage configuration of a
// container.
type ContainerStorageConfig struct {
	// Image is the image the container will be based on. The image will be
	// used as the container's root filesystem, and its environment vars,
	// volumes, and other configuration will be applied to the container.
	// Conflicts with Rootfs.
	// At least one of Image or Rootfs must be specified.
	Image string
	// Rootfs is the path to a directory that will be used as the
	// container's root filesystem. No modification will be made to the
	// directory, it will be directly mounted into the container as root.
	// Conflicts with Image.
	// At least one of Image or Rootfs must be specified.
	Rootfs string
	// ImageVolumeMode indicates how image volumes will be created.
	// Supported modes are "ignore" (do not create), "tmpfs" (create as
	// tmpfs), and "anonymous" (create as anonymous volumes).
	// The default is anonymous.
	// Optional.
	ImageVolumeMode string
	// VolumesFrom is a list of containers whose volumes will be added to
	// this container. Supported mount options may be added after the
	// container name with a : and include "ro" and "rw".
	// Optional.
	VolumesFrom []string
	// Mounts are mounts that will be added to the container.
	// These will supersede Image Volumes and VolumesFrom volumes where
	// there are conflicts.
	// Optional.
	Mounts []spec.Mount
	// Volumes are named volumes that will be added to the container.
	// These will supersede Image Volumes and VolumesFrom volumes where
	// there are conflicts.
	// Optional.
	Volumes []*libpod.ContainerNamedVolume
	// Devices are devices that will be added to the container.
	// Optional.
	Devices []spec.LinuxDevice
	// IpcNS is the container's IPC namespace.
	// Default is private.
	// Conflicts with ShmSize if not set to private.
	// Mandatory.
	IpcNS Namespace
	// ShmSize is the size of the tmpfs to mount in at /dev/shm, in bytes.
	// Conflicts with ShmSize if ShmSize is not private.
	// Optional.
	ShmSize *int64
	// WorkDir is the container's working directory.
	// If unset, the default, /, will be used.
	// Optional.
	WorkDir string
	// RootfsPropagation is the rootfs propagation mode for the container.
	// If not set, the default of rslave will be used.
	// Optional.
	RootfsPropagation string
}

// ContainerSecurityConfig is a container's security features, including
// SELinux, Apparmor, and Seccomp.
type ContainerSecurityConfig struct {
	// Privileged is whether the container is privileged.
	// Privileged does the following:
	// - Adds all devices on the system to the container.
	// - Adds all capabilities to the container.
	// - Disables Seccomp, SELinux, and Apparmor confinement.
	// TODO: this conflicts with things.
	// TODO: this does more.
	Privileged bool
	// User is the user the container will be run as.
	// Can be given as a UID or a username; if a username, it will be
	// resolved within the container, using the container's /etc/passwd.
	// If unset, the container will be run as root.
	// Optional.
	User string
	// Groups are a list of supplemental groups the container's user will
	// be granted access to.
	// Optional.
	Groups []string
	// CapAdd are capabilities which will be added to the container.
	// Conflicts with Privileged.
	// Optional.
	CapAdd []string
	// CapDrop are capabilities which will be removed from the container.
	// Conflicts with Privileged.
	// Optional.
	CapDrop []string
	// SelinuxProcessLabel is the process label the container will use.
	// If SELinux is enabled and this is not specified, a label will be
	// automatically generated if not specified.
	// Optional.
	SelinuxProcessLabel string
	// SelinuxMountLabel is the mount label the container will use.
	// If SELinux is enabled and this is not specified, a label will be
	// automatically generated if not specified.
	// Optional.
	SelinuxMountLabel string
	// SelinuxOpts are options for configuring SELinux.
	// Optional.
	SelinuxOpts []string
	// ApparmorProfile is the name of the Apparmor profile the container
	// will use.
	// Optional.
	ApparmorProfile string
	// SeccompProfilePath is the path to a JSON file containing the
	// container's Seccomp profile.
	// If not specified, no Seccomp profile will be used.
	// Optional.
	SeccompProfilePath string
	// NoNewPrivileges is whether the container will set the no new
	// privileges flag on create, which disables gaining additional
	// privileges (e.g. via setuid) in the container.
	NoNewPrivileges bool
	// UserNS is the container's user namespace.
	// It defaults to host, indicating that no user namespace will be
	// created.
	// If set to private, IDMappings must be set.
	// Mandatory.
	UserNS Namespace
	// IDMappings are UID and GID mappings that will be used by user
	// namespaces.
	// Required if UserNS is private.
	IDMappings storage.IDMappingOptions
}

// ContainerCgroupConfig contains configuration information about a container's
// cgroups.
type ContainerCgroupConfig struct {
	// CgroupNS is the container's cgroup namespace.
	// It defaults to private.
	// Conflicts with NoCgroups if not set to host.
	// Mandatory.
	CgroupNS Namespace
	// NoCgroups indicates that the container should not create CGroups.
	// Conflicts with CgroupParent and CgroupNS if CgroupNS is not set to
	// host.
	NoCgroups bool
	// CgroupParent is the container's CGroup parent.
	// If not set, the default for the current cgroup driver will be used.
	// Conflicts with NoCgroups.
	// Optional.
	CgroupParent string
}

// ContainerNetworkConfig contains information on a container's network
// configuration.
type ContainerNetworkConfig struct {
	// NetNS is the configuration to use for the container's network
	// namespace.
	// Mandatory.
	NetNS Namespace
	// ConfigureNetNS is whether Libpod will configure the container's
	// network namespace to send and receive traffic.
	// Only available is NetNS is private - conflicts with other NetNS
	// modes.
	ConfigureNetNS bool
	// StaticIP is the a IPv4 address of the container.
	// Only available if ConfigureNetNS is true.
	// Optional.
	StaticIP *net.IP
	// StaticIPv6 is a static IPv6 address to set in the container.
	// Only available if ConfigureNetNS is true.
	// Optional.
	StaticIPv6 *net.IP
	// StaticMAC is a static MAC address to set in the container.
	// Only available if ConfigureNetNS is true.
	// Optional.
	StaticMAC *net.HardwareAddr
	// PortBindings is a set of ports to map into the container.
	// Only available if ConfigureNetNS is true.
	// Optional.
	PortMappings []ocicni.PortMapping
	// PublishImagePorts will publish ports specified in the image to random
	// ports outside.
	// Requires Image to be set.
	PublishImagePorts bool
	// CNINetworks is a list of CNI networks to join the container to.
	// If this list is empty, the default CNI network will be joined
	// instead. If at least one entry is present, we will not join the
	// default network (unless it is part of this list).
	// Only available if ConfigureNetNS is true.
	// Optional.
	CNINetworks []string
	// UseImageResolvConf indicates that resolv.conf should not be managed
	// by Podman, but instead sourced from the image.
	// Conflicts with DNSServer, DNSSearch, DNSOption.
	UseImageResolvConf bool
	// DNSServer is a set of DNS servers that will be used in the
	// container's resolv.conf, replacing the host's DNS Servers which are
	// used by default.
	// Conflicts with UseImageResolvConf.
	// Optional.
	DNSServer []net.IP
	// DNSSearch is a set of DNS search domains that will be used in the
	// container's resolv.conf, replacing the host's DNS search domains
	// which are used by default.
	// Conflicts with UseImageResolvConf.
	// Optional.
	DNSSearch []string
	// DNSOption is a set of DNS options that will be used in the
	// container's resolv.conf, replacing the host's DNS options which are
	// used by default.
	// Conflicts with UseImageResolvConf.
	// Optional.
	DNSOption []string
	// UseImageHosts indicates that /etc/hosts should not be managed by
	// Podman, and instead sourced from the image.
	// Conflicts with HostAdd.
	UseImageHosts bool
	// HostAdd is a set of hosts which will be added to the container's
	// /etc/hosts file.
	// Conflicts with UseImageHosts.
	// Optional.
	HostAdd []string
}

// ContainerResourceConfig contains information on container resource limits.
type ContainerResourceConfig struct {
	// ResourceLimits are resource limits to apply to the container.
	// Can only be set as root on cgroups v1 systems, but can be set as
	// rootless as well for cgroups v2.
	// Optional.
	ResourceLimits *spec.LinuxResources
	// Rlimits are POSIX rlimits to apply to the container.
	// Optional.
	Rlimits []spec.POSIXRlimit
	// OOMScoreAdj adjusts the score used by the OOM killer to determine
	// processes to kill for the container's process.
	// Optional.
	OOMScoreAdj *int
}

// SpecGenerator creates an OCI spec and Libpod configuration options to create
// a container based on the given configuration.
type SpecGenerator struct {
	ContainerBasicConfig
	ContainerStorageConfig
	ContainerSecurityConfig
	ContainerCgroupConfig
	ContainerNetworkConfig
	ContainerResourceConfig
}
