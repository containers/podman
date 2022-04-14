package specgen

import (
	"net"
	"strings"
	"syscall"

	"github.com/containers/common/libimage"
	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/storage/types"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

//  LogConfig describes the logging characteristics for a container
type LogConfig struct {
	// LogDriver is the container's log driver.
	// Optional.
	Driver string `json:"driver,omitempty"`
	// LogPath is the path the container's logs will be stored at.
	// Only available if LogDriver is set to "json-file" or "k8s-file".
	// Optional.
	Path string `json:"path,omitempty"`
	// Size is the maximum size of the log file
	// Optional.
	Size int64 `json:"size,omitempty"`
	// A set of options to accompany the log driver.
	// Optional.
	Options map[string]string `json:"options,omitempty"`
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
	// EnvHost indicates that the host environment should be added to container
	// Optional.
	EnvHost bool `json:"env_host,omitempty"`
	// EnvHTTPProxy indicates that the http host proxy environment variables
	// should be added to container
	// Optional.
	HTTPProxy bool `json:"httpproxy,omitempty"`
	// Env is a set of environment variables that will be set in the
	// container.
	// Optional.
	Env map[string]string `json:"env,omitempty"`
	// Terminal is whether the container will create a PTY.
	// Optional.
	Terminal bool `json:"terminal,omitempty"`
	// Stdin is whether the container will keep its STDIN open.
	Stdin bool `json:"stdin,omitempty"`
	// Labels are key-value pairs that are used to add metadata to
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
	StopSignal *syscall.Signal `json:"stop_signal,omitempty"`
	// StopTimeout is a timeout between the container's stop signal being
	// sent and SIGKILL being sent.
	// If not provided, the default will be used.
	// If 0 is used, stop signal will not be sent, and SIGKILL will be sent
	// instead.
	// Optional.
	StopTimeout *uint `json:"stop_timeout,omitempty"`
	// Timeout is a maximum time in seconds the container will run before
	// main process is sent SIGKILL.
	// If 0 is used, signal will not be sent. Container can run indefinitely
	// Optional.
	Timeout uint `json:"timeout,omitempty"`
	// LogConfiguration describes the logging for a container including
	// driver, path, and options.
	// Optional
	LogConfiguration *LogConfig `json:"log_configuration,omitempty"`
	// ConmonPidFile is a path at which a PID file for Conmon will be
	// placed.
	// If not given, a default location will be used.
	// Optional.
	ConmonPidFile string `json:"conmon_pid_file,omitempty"`
	// RawImageName is the user-specified and unprocessed input referring
	// to a local or a remote image.
	RawImageName string `json:"raw_image_name,omitempty"`
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
	// Determine how to handle the NOTIFY_SOCKET - do we participate or pass it through
	// "container" - let the OCI runtime deal with it, advertise conmon's MAINPID
	// "conmon-only" - advertise conmon's MAINPID, send READY when started, don't pass to OCI
	// "ignore" - unset NOTIFY_SOCKET
	SdNotifyMode string `json:"sdnotifyMode,omitempty"`
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
	// HostUses is a list of host usernames or UIDs to add to the container
	// /etc/passwd file
	HostUsers []string `json:"hostusers,omitempty"`
	// Sysctl sets kernel parameters for the container
	Sysctl map[string]string `json:"sysctl,omitempty"`
	// Remove indicates if the container should be removed once it has been started
	// and exits
	Remove bool `json:"remove,omitempty"`
	// ContainerCreateCommand is the command that was used to create this
	// container.
	// This will be shown in the output of Inspect() on the container, and
	// may also be used by some tools that wish to recreate the container
	// (e.g. `podman generate systemd --new`).
	// Optional.
	ContainerCreateCommand []string `json:"containerCreateCommand,omitempty"`
	// PreserveFDs is a number of additional file descriptors (in addition
	// to 0, 1, 2) that will be passed to the executed process. The total FDs
	// passed will be 3 + PreserveFDs.
	// set tags as `json:"-"` for not supported remote
	// Optional.
	PreserveFDs uint `json:"-"`
	// Timezone is the timezone inside the container.
	// Local means it has the same timezone as the host machine
	// Optional.
	Timezone string `json:"timezone,omitempty"`
	// DependencyContainers is an array of containers this container
	// depends on. Dependency containers must be started before this
	// container. Dependencies can be specified by name or full/partial ID.
	// Optional.
	DependencyContainers []string `json:"dependencyContainers,omitempty"`
	// PidFile is the file that saves container process id.
	// set tags as `json:"-"` for not supported remote
	// Optional.
	PidFile string `json:"-"`
	// EnvSecrets are secrets that will be set as environment variables
	// Optional.
	EnvSecrets map[string]string `json:"secret_env,omitempty"`
	// InitContainerType describes if this container is an init container
	// and if so, what type: always or once
	InitContainerType string `json:"init_container_type"`
	// Personality allows users to configure different execution domains.
	// Execution domains tell Linux how to map signal numbers into signal actions.
	// The execution domain system allows Linux to provide limited support
	// for binaries compiled under other UNIX-like operating systems.
	Personality *spec.LinuxPersonality `json:"personality,omitempty"`
	// UnsetEnv unsets the specified default environment variables from the image or from buildin or containers.conf
	// Optional.
	UnsetEnv []string `json:"unsetenv,omitempty"`
	// UnsetEnvAll unsetall default environment variables from the image or from buildin or containers.conf
	// UnsetEnvAll unsets all default environment variables from the image or from buildin
	// Optional.
	UnsetEnvAll bool `json:"unsetenvall,omitempty"`
	// Passwd is a container run option that determines if we are validating users/groups before running the container
	Passwd *bool `json:"manage_password,omitempty"`
	// PasswdEntry specifies arbitrary data to append to a file.
	PasswdEntry string `json:"passwd_entry,omitempty"`
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
	// RootfsOverlay tells if rootfs is actually an overlay on top of base path
	RootfsOverlay bool `json:"rootfs_overlay,omitempty"`
	// ImageVolumeMode indicates how image volumes will be created.
	// Supported modes are "ignore" (do not create), "tmpfs" (create as
	// tmpfs), and "anonymous" (create as anonymous volumes).
	// The default if unset is anonymous.
	// Optional.
	ImageVolumeMode string `json:"image_volume_mode,omitempty"`
	// VolumesFrom is a set of containers whose volumes will be added to
	// this container. The name or ID of the container must be provided, and
	// may optionally be followed by a : and then one or more
	// comma-separated options. Valid options are 'ro', 'rw', and 'z'.
	// Options will be used for all volumes sourced from the container.
	VolumesFrom []string `json:"volumes_from,omitempty"`
	// Init specifies that an init binary will be mounted into the
	// container, and will be used as PID1.
	Init bool `json:"init,omitempty"`
	// InitPath specifies the path to the init binary that will be added if
	// Init is specified above. If not specified, the default set in the
	// Libpod config will be used. Ignored if Init above is not set.
	// Optional.
	InitPath string `json:"init_path,omitempty"`
	// Mounts are mounts that will be added to the container.
	// These will supersede Image Volumes and VolumesFrom volumes where
	// there are conflicts.
	// Optional.
	Mounts []spec.Mount `json:"mounts,omitempty"`
	// Volumes are named volumes that will be added to the container.
	// These will supersede Image Volumes and VolumesFrom volumes where
	// there are conflicts.
	// Optional.
	Volumes []*NamedVolume `json:"volumes,omitempty"`
	// Overlay volumes are named volumes that will be added to the container.
	// Optional.
	OverlayVolumes []*OverlayVolume `json:"overlay_volumes,omitempty"`
	// Image volumes bind-mount a container-image mount into the container.
	// Optional.
	ImageVolumes []*ImageVolume `json:"image_volumes,omitempty"`
	// Devices are devices that will be added to the container.
	// Optional.
	Devices []spec.LinuxDevice `json:"devices,omitempty"`
	// DeviceCgroupRule are device cgroup rules that allow containers
	// to use additional types of devices.
	DeviceCgroupRule []spec.LinuxDeviceCgroup `json:"device_cgroup_rule,omitempty"`
	// DevicesFrom is a way to ensure your container inherits device specific information from another container
	DevicesFrom []string `json:"devices_from,omitempty"`
	// HostDeviceList is used to recreate the mounted device on inherited containers
	HostDeviceList []spec.LinuxDevice `json:"host_device_list,omitempty"`
	// IpcNS is the container's IPC namespace.
	// Default is private.
	// Conflicts with ShmSize if not set to private.
	// Mandatory.
	IpcNS Namespace `json:"ipcns,omitempty"`
	// ShmSize is the size of the tmpfs to mount in at /dev/shm, in bytes.
	// Conflicts with ShmSize if IpcNS is not private.
	// Optional.
	ShmSize *int64 `json:"shm_size,omitempty"`
	// WorkDir is the container's working directory.
	// If unset, the default, /, will be used.
	// Optional.
	WorkDir string `json:"work_dir,omitempty"`
	// Create the working directory if it doesn't exist.
	// If unset, it doesn't create it.
	// Optional.
	CreateWorkingDir bool `json:"create_working_dir,omitempty"`
	// StorageOpts is the container's storage options
	// Optional.
	StorageOpts map[string]string `json:"storage_opts,omitempty"`
	// RootfsPropagation is the rootfs propagation mode for the container.
	// If not set, the default of rslave will be used.
	// Optional.
	RootfsPropagation string `json:"rootfs_propagation,omitempty"`
	// Secrets are the secrets that will be added to the container
	// Optional.
	Secrets []Secret `json:"secrets,omitempty"`
	// Volatile specifies whether the container storage can be optimized
	// at the cost of not syncing all the dirty files in memory.
	Volatile bool `json:"volatile,omitempty"`
	// ChrootDirs is an additional set of directories that need to be
	// treated as root directories. Standard bind mounts will be mounted
	// into paths relative to these directories.
	ChrootDirs []string `json:"chroot_directories,omitempty"`
}

// ContainerSecurityConfig is a container's security features, including
// SELinux, Apparmor, and Seccomp.
type ContainerSecurityConfig struct {
	// Privileged is whether the container is privileged.
	// Privileged does the following:
	// - Adds all devices on the system to the container.
	// - Adds all capabilities to the container.
	// - Disables Seccomp, SELinux, and Apparmor confinement.
	//   (Though SELinux can be manually re-enabled).
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
	SelinuxOpts []string `json:"selinux_opts,omitempty"`
	// ApparmorProfile is the name of the Apparmor profile the container
	// will use.
	// Optional.
	ApparmorProfile string `json:"apparmor_profile,omitempty"`
	// SeccompPolicy determines which seccomp profile gets applied
	// the container. valid values: empty,default,image
	SeccompPolicy string `json:"seccomp_policy,omitempty"`
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
	IDMappings *types.IDMappingOptions `json:"idmappings,omitempty"`
	// ReadOnlyFilesystem indicates that everything will be mounted
	// as read-only
	ReadOnlyFilesystem bool `json:"read_only_filesystem,omitempty"`
	// Umask is the umask the init process of the container will be run with.
	Umask string `json:"umask,omitempty"`
	// ProcOpts are the options used for the proc mount.
	ProcOpts []string `json:"procfs_opts,omitempty"`
	// Mask is the path we want to mask in the container. This masks the paths
	// given in addition to the default list.
	// Optional
	Mask []string `json:"mask,omitempty"`
	// Unmask is the path we want to unmask in the container. To override
	// all the default paths that are masked, set unmask=ALL.
	Unmask []string `json:"unmask,omitempty"`
}

// ContainerCgroupConfig contains configuration information about a container's
// cgroups.
type ContainerCgroupConfig struct {
	// CgroupNS is the container's cgroup namespace.
	// It defaults to private.
	// Mandatory.
	CgroupNS Namespace `json:"cgroupns,omitempty"`
	// CgroupsMode sets a policy for how cgroups will be created in the
	// container, including the ability to disable creation entirely.
	CgroupsMode string `json:"cgroups_mode,omitempty"`
	// CgroupParent is the container's Cgroup parent.
	// If not set, the default for the current cgroup driver will be used.
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
	// PortBindings is a set of ports to map into the container.
	// Only available if NetNS is set to bridge or slirp.
	// Optional.
	PortMappings []nettypes.PortMapping `json:"portmappings,omitempty"`
	// PublishExposedPorts will publish ports specified in the image to
	// random unused ports (guaranteed to be above 1024) on the host.
	// This is based on ports set in Expose below, and any ports specified
	// by the Image (if one is given).
	// Only available if NetNS is set to Bridge or Slirp.
	PublishExposedPorts bool `json:"publish_image_ports,omitempty"`
	// Expose is a number of ports that will be forwarded to the container
	// if PublishExposedPorts is set.
	// Expose is a map of uint16 (port number) to a string representing
	// protocol i.e map[uint16]string. Allowed protocols are "tcp", "udp", and "sctp", or some
	// combination of the three separated by commas.
	// If protocol is set to "" we will assume TCP.
	// Only available if NetNS is set to Bridge or Slirp, and
	// PublishExposedPorts is set.
	// Optional.
	Expose map[uint16]string `json:"expose,omitempty"`
	// Map of networks names or ids that the container should join.
	// You can request additional settings for each network, you can
	// set network aliases, static ips, static mac address  and the
	// network interface name for this container on the specific network.
	// If the map is empty and the bridge network mode is set the container
	// will be joined to the default network.
	Networks map[string]nettypes.PerNetworkOptions
	// CNINetworks is a list of CNI networks to join the container to.
	// If this list is empty, the default CNI network will be joined
	// instead. If at least one entry is present, we will not join the
	// default network (unless it is part of this list).
	// Only available if NetNS is set to bridge.
	// Optional.
	// Deprecated: as of podman 4.0 use "Networks" instead.
	CNINetworks []string `json:"cni_networks,omitempty"`
	// UseImageResolvConf indicates that resolv.conf should not be managed
	// by Podman, but instead sourced from the image.
	// Conflicts with DNSServer, DNSSearch, DNSOption.
	UseImageResolvConf bool `json:"use_image_resolve_conf,omitempty"`
	// DNSServers is a set of DNS servers that will be used in the
	// container's resolv.conf, replacing the host's DNS Servers which are
	// used by default.
	// Conflicts with UseImageResolvConf.
	// Optional.
	DNSServers []net.IP `json:"dns_server,omitempty"`
	// DNSSearch is a set of DNS search domains that will be used in the
	// container's resolv.conf, replacing the host's DNS search domains
	// which are used by default.
	// Conflicts with UseImageResolvConf.
	// Optional.
	DNSSearch []string `json:"dns_search,omitempty"`
	// DNSOptions is a set of DNS options that will be used in the
	// container's resolv.conf, replacing the host's DNS options which are
	// used by default.
	// Conflicts with UseImageResolvConf.
	// Optional.
	DNSOptions []string `json:"dns_option,omitempty"`
	// UseImageHosts indicates that /etc/hosts should not be managed by
	// Podman, and instead sourced from the image.
	// Conflicts with HostAdd.
	// Do not set omitempty here, if this is false it should be set to not get
	// the server default.
	// Ideally this would be a pointer so we could differentiate between an
	// explicitly false/true and unset (containers.conf default). However
	// specgen is stable so we can not change this right now.
	// TODO (5.0): change to pointer
	UseImageHosts bool `json:"use_image_hosts"`
	// HostAdd is a set of hosts which will be added to the container's
	// /etc/hosts file.
	// Conflicts with UseImageHosts.
	// Optional.
	HostAdd []string `json:"hostadd,omitempty"`
	// NetworkOptions are additional options for each network
	// Optional.
	NetworkOptions map[string][]string `json:"network_options,omitempty"`
}

// ContainerResourceConfig contains information on container resource limits.
type ContainerResourceConfig struct {
	// ResourceLimits are resource limits to apply to the container.,
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
	// Weight per cgroup per device, can override BlkioWeight
	WeightDevice map[string]spec.LinuxWeightDevice `json:"weightDevice,omitempty"`
	// IO read rate limit per cgroup per device, bytes per second
	ThrottleReadBpsDevice map[string]spec.LinuxThrottleDevice `json:"throttleReadBpsDevice,omitempty"`
	// IO write rate limit per cgroup per device, bytes per second
	ThrottleWriteBpsDevice map[string]spec.LinuxThrottleDevice `json:"throttleWriteBpsDevice,omitempty"`
	// IO read rate limit per cgroup per device, IO per second
	ThrottleReadIOPSDevice map[string]spec.LinuxThrottleDevice `json:"throttleReadIOPSDevice,omitempty"`
	// IO write rate limit per cgroup per device, IO per second
	ThrottleWriteIOPSDevice map[string]spec.LinuxThrottleDevice `json:"throttleWriteIOPSDevice,omitempty"`
	// CgroupConf are key-value options passed into the container runtime
	// that are used to configure cgroup v2.
	// Optional.
	CgroupConf map[string]string `json:"unified,omitempty"`
	// CPU period of the cpuset, determined by --cpus
	CPUPeriod uint64 `json:"cpu_period,omitempty"`
	// CPU quota of the cpuset, determined by --cpus
	CPUQuota int64 `json:"cpu_quota,omitempty"`
}

// ContainerHealthCheckConfig describes a container healthcheck with attributes
// like command, retries, interval, start period, and timeout.
type ContainerHealthCheckConfig struct {
	HealthConfig *manifest.Schema2HealthConfig `json:"healthconfig,omitempty"`
}

// SpecGenerator creates an OCI spec and Libpod configuration options to create
// a container based on the given configuration.
// swagger:model SpecGenerator
type SpecGenerator struct {
	ContainerBasicConfig
	ContainerStorageConfig
	ContainerSecurityConfig
	ContainerCgroupConfig
	ContainerNetworkConfig
	ContainerResourceConfig
	ContainerHealthCheckConfig

	image             *libimage.Image `json:"-"`
	resolvedImageName string          `json:"-"`
}

// SetImage sets the associated for the generator.
func (s *SpecGenerator) SetImage(image *libimage.Image, resolvedImageName string) {
	s.image = image
	s.resolvedImageName = resolvedImageName
}

// Image returns the associated image for the generator.
// May be nil if no image has been set yet.
func (s *SpecGenerator) GetImage() (*libimage.Image, string) {
	return s.image, s.resolvedImageName
}

type Secret struct {
	Source string
	Target string
	UID    uint32
	GID    uint32
	Mode   uint32
}

var (
	// ErrNoStaticIPRootless is used when a rootless user requests to assign a static IP address
	// to a pod or container
	ErrNoStaticIPRootless error = errors.New("rootless containers and pods cannot be assigned static IP addresses")
	// ErrNoStaticMACRootless is used when a rootless user requests to assign a static MAC address
	// to a pod or container
	ErrNoStaticMACRootless error = errors.New("rootless containers and pods cannot be assigned static MAC addresses")
)

// NewSpecGenerator returns a SpecGenerator struct given one of two mandatory inputs
func NewSpecGenerator(arg string, rootfs bool) *SpecGenerator {
	csc := ContainerStorageConfig{}
	if rootfs {
		csc.Rootfs = arg
		// check if rootfs should use overlay
		lastColonIndex := strings.LastIndex(csc.Rootfs, ":")
		if lastColonIndex != -1 && lastColonIndex+1 < len(csc.Rootfs) && csc.Rootfs[lastColonIndex+1:] == "O" {
			csc.RootfsOverlay = true
			csc.Rootfs = csc.Rootfs[:lastColonIndex]
		}
	} else {
		csc.Image = arg
	}
	return &SpecGenerator{
		ContainerStorageConfig: csc,
	}
}

// NewSpecGenerator returns a SpecGenerator struct given one of two mandatory inputs
func NewSpecGeneratorWithRootfs(rootfs string) *SpecGenerator {
	csc := ContainerStorageConfig{Rootfs: rootfs}
	return &SpecGenerator{ContainerStorageConfig: csc}
}
