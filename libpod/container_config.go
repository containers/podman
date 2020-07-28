package libpod

import (
	"net"
	"time"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/podman/v2/pkg/namespaces"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// ContainerConfig contains all information that was used to create the
// container. It may not be changed once created.
// It is stored, read-only, on disk
type ContainerConfig struct {
	Spec *spec.Spec `json:"spec"`

	ID string `json:"id"`

	Name string `json:"name"`

	// Full ID of the pood the container belongs to
	Pod string `json:"pod,omitempty"`

	// Namespace the container is in
	Namespace string `json:"namespace,omitempty"`

	// ID of this container's lock
	LockID uint32 `json:"lockID"`

	// CreateCommand is the full command plus arguments of the process the
	// container has been created with.
	CreateCommand []string `json:"CreateCommand,omitempty"`

	// RawImageName is the raw and unprocessed name of the image when creating
	// the container (as specified by the user).  May or may not be set.  One
	// use case to store this data are auto-updates where we need the _exact_
	// name and not some normalized instance of it.
	RawImageName string `json:"RawImageName,omitempty"`

	// UID/GID mappings used by the storage
	IDMappings storage.IDMappingOptions `json:"idMappingsOptions,omitempty"`

	// IDs of dependency containers.
	// These containers must be started before this container is started.
	Dependencies []string

	// embedded sub-configs
	ContainerRootFSConfig
	ContainerSecurityConfig
	ContainerNameSpaceConfig
	ContainerNetworkConfig
	ContainerImageConfig
	ContainerMiscConfig
}

// ContainerRootFSConfig is an embedded sub-config providing config info
// about the container's root fs.
type ContainerRootFSConfig struct {
	RootfsImageID   string `json:"rootfsImageID,omitempty"`
	RootfsImageName string `json:"rootfsImageName,omitempty"`
	// Rootfs to use for the container, this conflicts with RootfsImageID
	Rootfs string `json:"rootfs,omitempty"`
	// Src path to be mounted on /dev/shm in container.
	ShmDir string `json:"ShmDir,omitempty"`
	// Size of the container's SHM.
	ShmSize int64 `json:"shmSize"`
	// Static directory for container content that will persist across
	// reboot.
	StaticDir string `json:"staticDir"`
	// Mounts list contains all additional mounts into the container rootfs.
	// These include the SHM mount.
	// These must be unmounted before the container's rootfs is unmounted.
	Mounts []string `json:"mounts,omitempty"`
	// NamedVolumes lists the named volumes to mount into the container.
	NamedVolumes []*ContainerNamedVolume `json:"namedVolumes,omitempty"`
	// OverlayVolumes lists the overlay volumes to mount into the container.
	OverlayVolumes []*ContainerOverlayVolume `json:"overlayVolumes,omitempty"`
}

// ContainerSecurityConfig is an embedded sub-config providing security configuration
// to the container.
type ContainerSecurityConfig struct {
	// Whether the container is privileged
	Privileged bool `json:"privileged"`
	// SELinux process label for container
	ProcessLabel string `json:"ProcessLabel,omitempty"`
	// SELinux mount label for root filesystem
	MountLabel string `json:"MountLabel,omitempty"`
	// LabelOpts are options passed in by the user to setup SELinux labels
	LabelOpts []string `json:"labelopts,omitempty"`
	// User and group to use in the container
	// Can be specified by name or UID/GID
	User string `json:"user,omitempty"`
	// Additional groups to add
	Groups []string `json:"groups,omitempty"`
	// AddCurrentUserPasswdEntry indicates that the current user passwd entry
	// should be added to the /etc/passwd within the container
	AddCurrentUserPasswdEntry bool `json:"addCurrentUserPasswdEntry,omitempty"`
}

// ContainerNameSpaceConfig is an embedded sub-config providing
// namespace configuration to the container.
type ContainerNameSpaceConfig struct {
	// IDs of container to share namespaces with
	// NetNsCtr conflicts with the CreateNetNS bool
	// These containers are considered dependencies of the given container
	// They must be started before the given container is started
	IPCNsCtr    string `json:"ipcNsCtr,omitempty"`
	MountNsCtr  string `json:"mountNsCtr,omitempty"`
	NetNsCtr    string `json:"netNsCtr,omitempty"`
	PIDNsCtr    string `json:"pidNsCtr,omitempty"`
	UserNsCtr   string `json:"userNsCtr,omitempty"`
	UTSNsCtr    string `json:"utsNsCtr,omitempty"`
	CgroupNsCtr string `json:"cgroupNsCtr,omitempty"`
}

// ContainerNetworkConfig is an embedded sub-config providing network configuration
// to the container.
type ContainerNetworkConfig struct {
	// CreateNetNS indicates that libpod should create and configure a new
	// network namespace for the container.
	// This cannot be set if NetNsCtr is also set.
	CreateNetNS bool `json:"createNetNS"`
	// StaticIP is a static IP to request for the container.
	// This cannot be set unless CreateNetNS is set.
	// If not set, the container will be dynamically assigned an IP by CNI.
	StaticIP net.IP `json:"staticIP"`
	// StaticMAC is a static MAC to request for the container.
	// This cannot be set unless CreateNetNS is set.
	// If not set, the container will be dynamically assigned a MAC by CNI.
	StaticMAC net.HardwareAddr `json:"staticMAC"`
	// PortMappings are the ports forwarded to the container's network
	// namespace
	// These are not used unless CreateNetNS is true
	PortMappings []ocicni.PortMapping `json:"portMappings,omitempty"`
	// UseImageResolvConf indicates that resolv.conf should not be
	// bind-mounted inside the container.
	// Conflicts with DNSServer, DNSSearch, DNSOption.
	UseImageResolvConf bool
	// DNS servers to use in container resolv.conf
	// Will override servers in host resolv if set
	DNSServer []net.IP `json:"dnsServer,omitempty"`
	// DNS Search domains to use in container resolv.conf
	// Will override search domains in host resolv if set
	DNSSearch []string `json:"dnsSearch,omitempty"`
	// DNS options to be set in container resolv.conf
	// With override options in host resolv if set
	DNSOption []string `json:"dnsOption,omitempty"`
	// UseImageHosts indicates that /etc/hosts should not be
	// bind-mounted inside the container.
	// Conflicts with HostAdd.
	UseImageHosts bool
	// Hosts to add in container
	// Will be appended to host's host file
	HostAdd []string `json:"hostsAdd,omitempty"`
	// Network names (CNI) to add container to. Empty to use default network.
	Networks []string `json:"networks,omitempty"`
	// Network mode specified for the default network.
	NetMode namespaces.NetworkMode `json:"networkMode,omitempty"`
	// NetworkOptions are additional options for each network
	NetworkOptions map[string][]string `json:"network_options,omitempty"`
}

// ContainerImageConfig is an embedded sub-config providing image configuration
// to the container.
type ContainerImageConfig struct {
	// UserVolumes contains user-added volume mounts in the container.
	// These will not be added to the container's spec, as it is assumed
	// they are already present in the spec given to Libpod. Instead, it is
	// used when committing containers to generate the VOLUMES field of the
	// image that is created, and for triggering some OCI hooks which do not
	// fire unless user-added volume mounts are present.
	UserVolumes []string `json:"userVolumes,omitempty"`
	// Entrypoint is the container's entrypoint.
	// It is not used in spec generation, but will be used when the
	// container is committed to populate the entrypoint of the new image.
	Entrypoint []string `json:"entrypoint,omitempty"`
	// Command is the container's command.
	// It is not used in spec generation, but will be used when the
	// container is committed to populate the command of the new image.
	Command []string `json:"command,omitempty"`
}

// ContainerMiscConfig is an embedded sub-config providing misc configuration
// to the container.
type ContainerMiscConfig struct {
	// Whether to keep container STDIN open
	Stdin bool `json:"stdin,omitempty"`
	// Labels is a set of key-value pairs providing additional information
	// about a container
	Labels map[string]string `json:"labels,omitempty"`
	// StopSignal is the signal that will be used to stop the container
	StopSignal uint `json:"stopSignal,omitempty"`
	// StopTimeout is the signal that will be used to stop the container
	StopTimeout uint `json:"stopTimeout,omitempty"`
	// Time container was created
	CreatedTime time.Time `json:"createdTime"`
	// NoCgroups indicates that the container will not create CGroups. It is
	// incompatible with CgroupParent.  Deprecated in favor of CgroupsMode.
	NoCgroups bool `json:"noCgroups,omitempty"`
	// CgroupsMode indicates how the container will create cgroups
	// (disabled, no-conmon, enabled).  It supersedes NoCgroups.
	CgroupsMode string `json:"cgroupsMode,omitempty"`
	// Cgroup parent of the container
	CgroupParent string `json:"cgroupParent"`
	// LogPath log location
	LogPath string `json:"logPath"`
	// LogTag is the tag used for logging
	LogTag string `json:"logTag"`
	// LogDriver driver for logs
	LogDriver string `json:"logDriver"`
	// File containing the conmon PID
	ConmonPidFile string `json:"conmonPidFile,omitempty"`
	// RestartPolicy indicates what action the container will take upon
	// exiting naturally.
	// Allowed options are "no" (take no action), "on-failure" (restart on
	// non-zero exit code, up an a maximum of RestartRetries times),
	// and "always" (always restart the container on any exit code).
	// The empty string is treated as the default ("no")
	RestartPolicy string `json:"restart_policy,omitempty"`
	// RestartRetries indicates the number of attempts that will be made to
	// restart the container. Used only if RestartPolicy is set to
	// "on-failure".
	RestartRetries uint `json:"restart_retries,omitempty"`
	// TODO log options for log drivers
	// PostConfigureNetNS needed when a user namespace is created by an OCI runtime
	// if the network namespace is created before the user namespace it will be
	// owned by the wrong user namespace.
	PostConfigureNetNS bool `json:"postConfigureNetNS"`
	// OCIRuntime used to create the container
	OCIRuntime string `json:"runtime,omitempty"`
	// ExitCommand is the container's exit command.
	// This Command will be executed when the container exits
	ExitCommand []string `json:"exitCommand,omitempty"`
	// IsInfra is a bool indicating whether this container is an infra container used for
	// sharing kernel namespaces in a pod
	IsInfra bool `json:"pause"`
	// SdNotifyMode tells libpod what to do with a NOTIFY_SOCKET if passed
	SdNotifyMode string `json:"sdnotifyMode,omitempty"`
	// Systemd tells libpod to setup the container in systemd mode
	Systemd bool `json:"systemd"`
	// HealthCheckConfig has the health check command and related timings
	HealthCheckConfig *manifest.Schema2HealthConfig `json:"healthcheck"`
	// PreserveFDs is a number of additional file descriptors (in addition
	// to 0, 1, 2) that will be passed to the executed process. The total FDs
	// passed will be 3 + PreserveFDs.
	PreserveFDs uint `json:"preserveFds,omitempty"`
	// Timezone is the timezone inside the container.
	// Local means it has the same timezone as the host machine
	Timezone string `json:"timezone,omitempty"`
	// Umask is the umask inside the container.
	Umask string `json:"umask,omitempty"`
}
