package specgen

import (
	"net"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// TODO
// mheon provided this an off the cuff suggestion. Adding it here to retain
// for history as we implement it.  When this struct is implemented, we need
// to remove the nolints.
type Namespace struct {
	isHost      bool   //nolint
	isPath      string //nolint
	isContainer string //nolint
	isPod       bool   //nolint
}

// ContainerBasicConfig contains the basic parts of a container.
type ContainerBasicConfig struct {
	// Name is the name the container will be given.
	// If no name is provided, one will be randomly generated.
	// Optional.
	Name string `json:"name,omitempty"`
	// Pod is the ID of the pod the container will join.
	// Optional.
	Pod string `json:"pod,omitempty"`
	// Entrypoint is the container's entrypoint.
	// If not given and Image is specified, this will be populated by the
	// image's configuration.
	// Optional.
	Entrypoint []string `json:"entrypoint,omitempty"`
	// Command is the container's command.
	// If not given and Image is specified, this will be populated by the
	// image's configuration.
	// Optional.
	Command []string `json:"command,omitempty"`
	// Env is a set of environment variables that will be set in the
	// container.
	// Optional.
	Env map[string]string `json:"env,omitempty"`
	// Terminal is whether the container will create a PTY.
	Terminal bool `json:"terminal,omitempty"`
	// Stdin is whether the container will keep its STDIN open.
	Stdin bool `json:"stdin,omitempty"`
	// Labels are key-valid labels that are used to add metadata to
	// containers.
	// Optional.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations are key-value options passed into the container runtime
	// that can be used to trigger special behavior.
	// Optional.
	Annotations map[string]string `json:"annotations,omitempty"`
	// StopSignal is the signal that will be used to stop the container.
	// Must be a non-zero integer below SIGRTMAX.
	// If not provided, the default, SIGTERM, will be used.
	// Will conflict with Systemd if Systemd is set to "true" or "always".
	// Optional.
	StopSignal *uint `json:"stop_signal,omitempty"`
	// StopTimeout is a timeout between the container's stop signal being
	// sent and SIGKILL being sent.
	// If not provided, the default will be used.
	// If 0 is used, stop signal will not be sent, and SIGKILL will be sent
	// instead.
	// Optional.
	StopTimeout *uint `json:"stop_timeout,omitempty"`
	// LogDriver is the container's log driver.
	// Optional.
	LogDriver string `json:"log_driver,omitempty"`
	// LogPath is the path the container's logs will be stored at.
	// Only available if LogDriver is set to "json-file" or "k8s-file".
	// Optional.
	LogPath string `json:"log_path,omitempty"`
	// ConmonPidFile is a path at which a PID file for Conmon will be
	// placed.
	// If not given, a default location will be used.
	// Optional.
	ConmonPidFile string `json:"conmon_pid_file,omitempty"`
	// RestartPolicy is the container's restart policy - an action which
	// will be taken when the container exits.
	// If not given, the default policy, which does nothing, will be used.
	// Optional.
	RestartPolicy string `json:"restart_policy,omitempty"`
	// RestartRetries is the number of attempts that will be made to restart
	// the container.
	// Only available when RestartPolicy is set to "on-failure".
	// Optional.
	RestartRetries *uint `json:"restart_tries,omitempty"`
	// OCIRuntime is the name of the OCI runtime that will be used to create
	// the container.
	// If not specified, the default will be used.
	// Optional.
	OCIRuntime string `json:"oci_runtime,omitempty"`
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
	Systemd string `json:"systemd,omitempty"`
	// Namespace is the libpod namespace the container will be placed in.
	// Optional.
	Namespace string `json:"namespace,omitempty"`

	// PidNS is the container's PID namespace.
	// It defaults to private.
	// Mandatory.
	PidNS Namespace `json:"pidns,omitempty"`

	// UtsNS is the container's UTS namespace.
	// It defaults to private.
	// Must be set to Private to set Hostname.
	// Mandatory.
	UtsNS Namespace `json:"utsns,omitempty"`
	// Hostname is the container's hostname. If not set, the hostname will
	// not be modified (if UtsNS is not private) or will be set to the
	// container ID (if UtsNS is private).
	// Conflicts with UtsNS if UtsNS is not set to private.
	// Optional.
	Hostname string `json:"hostname,omitempty"`
}

// ContainerStorageConfig contains information on the storage configuration of a
// container.
type ContainerStorageConfig struct {
	// Image is the image the container will be based on. The image will be
	// used as the container's root filesystem, and its environment vars,
	// volumes, and other configuration will be applied to the container.
	// Conflicts with Rootfs.
	// At least one of Image or Rootfs must be specified.
	Image string `json:"image"`
	// Rootfs is the path to a directory that will be used as the
	// container's root filesystem. No modification will be made to the
	// directory, it will be directly mounted into the container as root.
	// Conflicts with Image.
	// At least one of Image or Rootfs must be specified.
	Rootfs string `json:"rootfs,omitempty"`
	// ImageVolumeMode indicates how image volumes will be created.
	// Supported modes are "ignore" (do not create), "tmpfs" (create as
	// tmpfs), and "anonymous" (create as anonymous volumes).
	// The default is anonymous.
	// Optional.
	ImageVolumeMode string `json:"image_volume_mode,omitempty"`
	// VolumesFrom is a list of containers whose volumes will be added to
	// this container. Supported mount options may be added after the
	// container name with a : and include "ro" and "rw".
	// Optional.
	VolumesFrom []string `json:"volumes_from,omitempty"`
	// Mounts are mounts that will be added to the container.
	// These will supersede Image Volumes and VolumesFrom volumes where
	// there are conflicts.
	// Optional.
	Mounts []spec.Mount `json:"mounts,omitempty"`
	// Volumes are named volumes that will be added to the container.
	// These will supersede Image Volumes and VolumesFrom volumes where
	// there are conflicts.
	// Optional.
	Volumes []*libpod.ContainerNamedVolume `json:"volumes,omitempty"`
	// Devices are devices that will be added to the container.
	// Optional.
	Devices []spec.LinuxDevice `json:"devices,omitempty"`
	// IpcNS is the container's IPC namespace.
	// Default is private.
	// Conflicts with ShmSize if not set to private.
	// Mandatory.
	IpcNS Namespace `json:"ipcns,omitempty"`
	// ShmSize is the size of the tmpfs to mount in at /dev/shm, in bytes.
	// Conflicts with ShmSize if ShmSize is not private.
	// Optional.
	ShmSize *int64 `json:"shm_size,omitempty"`
	// WorkDir is the container's working directory.
	// If unset, the default, /, will be used.
	// Optional.
	WorkDir string `json:"work_dir,omitempty"`
	// RootfsPropagation is the rootfs propagation mode for the container.
	// If not set, the default of rslave will be used.
	// Optional.
	RootfsPropagation string `json:"rootfs_propagation,omitempty"`
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
	Privileged bool `json:"privileged,omitempty"`
	// User is the user the container will be run as.
	// Can be given as a UID or a username; if a username, it will be
	// resolved within the container, using the container's /etc/passwd.
	// If unset, the container will be run as root.
	// Optional.
	User string `json:"user,omitempty"`
	// Groups are a list of supplemental groups the container's user will
	// be granted access to.
	// Optional.
	Groups []string `json:"groups,omitempty"`
	// CapAdd are capabilities which will be added to the container.
	// Conflicts with Privileged.
	// Optional.
	CapAdd []string `json:"cap_add,omitempty"`
	// CapDrop are capabilities which will be removed from the container.
	// Conflicts with Privileged.
	// Optional.
	CapDrop []string `json:"cap_drop,omitempty"`
	// SelinuxProcessLabel is the process label the container will use.
	// If SELinux is enabled and this is not specified, a label will be
	// automatically generated if not specified.
	// Optional.
	SelinuxProcessLabel string `json:"selinux_process_label,omitempty"`
	// SelinuxMountLabel is the mount label the container will use.
	// If SELinux is enabled and this is not specified, a label will be
	// automatically generated if not specified.
	// Optional.
	SelinuxMountLabel string `json:"selinux_mount_label,omitempty"`
	// SelinuxOpts are options for configuring SELinux.
	// Optional.
	SelinuxOpts []string `json:"selinux_opts,omitempty"`
	// ApparmorProfile is the name of the Apparmor profile the container
	// will use.
	// Optional.
	ApparmorProfile string `json:"apparmor_profile,omitempty"`
	// SeccompProfilePath is the path to a JSON file containing the
	// container's Seccomp profile.
	// If not specified, no Seccomp profile will be used.
	// Optional.
	SeccompProfilePath string `json:"seccomp_profile_path,omitempty"`
	// NoNewPrivileges is whether the container will set the no new
	// privileges flag on create, which disables gaining additional
	// privileges (e.g. via setuid) in the container.
	NoNewPrivileges bool `json:"no_new_privileges,omitempty"`
	// UserNS is the container's user namespace.
	// It defaults to host, indicating that no user namespace will be
	// created.
	// If set to private, IDMappings must be set.
	// Mandatory.
	UserNS Namespace `json:"userns,omitempty"`
	// IDMappings are UID and GID mappings that will be used by user
	// namespaces.
	// Required if UserNS is private.
	IDMappings storage.IDMappingOptions `json:"idmappings,omitempty"`
}

// ContainerCgroupConfig contains configuration information about a container's
// cgroups.
type ContainerCgroupConfig struct {
	// CgroupNS is the container's cgroup namespace.
	// It defaults to private.
	// Conflicts with NoCgroups if not set to host.
	// Mandatory.
	CgroupNS Namespace `json:"cgroupns,omitempty"`
	// NoCgroups indicates that the container should not create CGroups.
	// Conflicts with CgroupParent and CgroupNS if CgroupNS is not set to
	// host.
	NoCgroups bool `json:"no_cgroups,omitempty"`
	// CgroupParent is the container's CGroup parent.
	// If not set, the default for the current cgroup driver will be used.
	// Conflicts with NoCgroups.
	// Optional.
	CgroupParent string `json:"cgroup_parent,omitempty"`
}

// ContainerNetworkConfig contains information on a container's network
// configuration.
type ContainerNetworkConfig struct {
	// NetNS is the configuration to use for the container's network
	// namespace.
	// Mandatory.
	NetNS Namespace `json:"netns,omitempty"`
	// ConfigureNetNS is whether Libpod will configure the container's
	// network namespace to send and receive traffic.
	// Only available is NetNS is private - conflicts with other NetNS
	// modes.
	ConfigureNetNS bool `json:"configure_netns,omitempty"`
	// StaticIP is the a IPv4 address of the container.
	// Only available if ConfigureNetNS is true.
	// Optional.
	StaticIP *net.IP `json:"static_ip,omitempty"`
	// StaticIPv6 is a static IPv6 address to set in the container.
	// Only available if ConfigureNetNS is true.
	// Optional.
	StaticIPv6 *net.IP `json:"static_ipv6,omitempty"`
	// StaticMAC is a static MAC address to set in the container.
	// Only available if ConfigureNetNS is true.
	// Optional.
	StaticMAC *net.HardwareAddr `json:"static_mac,omitempty"`
	// PortBindings is a set of ports to map into the container.
	// Only available if ConfigureNetNS is true.
	// Optional.
	PortMappings []ocicni.PortMapping `json:"portmappings,omitempty"`
	// PublishImagePorts will publish ports specified in the image to random
	// ports outside.
	// Requires Image to be set.
	PublishImagePorts bool `json:"publish_image_ports,omitempty"`
	// CNINetworks is a list of CNI networks to join the container to.
	// If this list is empty, the default CNI network will be joined
	// instead. If at least one entry is present, we will not join the
	// default network (unless it is part of this list).
	// Only available if ConfigureNetNS is true.
	// Optional.
	CNINetworks []string `json:"cni_networks,omitempty"`
	// UseImageResolvConf indicates that resolv.conf should not be managed
	// by Podman, but instead sourced from the image.
	// Conflicts with DNSServer, DNSSearch, DNSOption.
	UseImageResolvConf bool `json:"use_image_resolve_conf,omitempty"`
	// DNSServer is a set of DNS servers that will be used in the
	// container's resolv.conf, replacing the host's DNS Servers which are
	// used by default.
	// Conflicts with UseImageResolvConf.
	// Optional.
	DNSServer []net.IP `json:"dns_server,omitempty"`
	// DNSSearch is a set of DNS search domains that will be used in the
	// container's resolv.conf, replacing the host's DNS search domains
	// which are used by default.
	// Conflicts with UseImageResolvConf.
	// Optional.
	DNSSearch []string `json:"dns_search,omitempty"`
	// DNSOption is a set of DNS options that will be used in the
	// container's resolv.conf, replacing the host's DNS options which are
	// used by default.
	// Conflicts with UseImageResolvConf.
	// Optional.
	DNSOption []string `json:"dns_option,omitempty"`
	// UseImageHosts indicates that /etc/hosts should not be managed by
	// Podman, and instead sourced from the image.
	// Conflicts with HostAdd.
	UseImageHosts bool `json:"use_image_hosts,omitempty"`
	// HostAdd is a set of hosts which will be added to the container's
	// /etc/hosts file.
	// Conflicts with UseImageHosts.
	// Optional.
	HostAdd []string `json:"hostadd,omitempty"`
}

// ContainerResourceConfig contains information on container resource limits.
type ContainerResourceConfig struct {
	// ResourceLimits are resource limits to apply to the container.
	// Can only be set as root on cgroups v1 systems, but can be set as
	// rootless as well for cgroups v2.
	// Optional.
	ResourceLimits *spec.LinuxResources `json:"resource_limits,omitempty"`
	// Rlimits are POSIX rlimits to apply to the container.
	// Optional.
	Rlimits []spec.POSIXRlimit `json:"r_limits,omitempty"`
	// OOMScoreAdj adjusts the score used by the OOM killer to determine
	// processes to kill for the container's process.
	// Optional.
	OOMScoreAdj *int `json:"oom_score_adj,omitempty"`
}

// ContainerHealthCheckConfig describes a container healthcheck with attributes
// like command, retries, interval, start period, and timeout.
type ContainerHealthCheckConfig struct {
	HealthConfig manifest.Schema2HealthConfig `json:"healthconfig,omitempty"`
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
	ContainerHealthCheckConfig
}

// NewSpecGenerator returns a SpecGenerator struct given one of two mandatory inputs
func NewSpecGenerator(image, rootfs *string) (*SpecGenerator, error) {
	_ = image
	_ = rootfs
	return &SpecGenerator{}, define.ErrNotImplemented
}

// Validate verifies that the given SpecGenerator is valid and satisfies required
// input for creating a container.
func (s *SpecGenerator) Validate() error {
	return define.ErrNotImplemented
}

// MakeContainer creates a container based on the SpecGenerator
func (s *SpecGenerator) MakeContainer() (*libpod.Container, error) {
	return nil, define.ErrNotImplemented
}
