package libpod

import (
	"fmt"
	"strings"
	"time"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/driver"
	"github.com/containers/libpod/pkg/util"
	"github.com/cri-o/ocicni/pkg/ocicni"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/runtime-tools/validate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"
)

const (
	// InspectAnnotationCIDFile is used by Inspect to determine if a
	// container ID file was created for the container.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationCIDFile = "io.podman.annotations.cid-file"
	// InspectAnnotationAutoremove is used by Inspect to determine if a
	// container will be automatically removed on exit.
	// If an annotation with this key is found in the OCI spec and is one of
	// the two supported boolean values (InspectResponseTrue and
	// InspectResponseFalse) it will be used in the output of Inspect().
	InspectAnnotationAutoremove = "io.podman.annotations.autoremove"
	// InspectAnnotationVolumesFrom is used by Inspect to identify
	// containers whose volumes are are being used by this container.
	// It is expected to be a comma-separated list of container names and/or
	// IDs.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationVolumesFrom = "io.podman.annotations.volumes-from"
	// InspectAnnotationPrivileged is used by Inspect to identify containers
	// which are privileged (IE, running with elevated privileges).
	// It is expected to be a boolean, populated by one of
	// InspectResponseTrue or InspectResponseFalse.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationPrivileged = "io.podman.annotations.privileged"
	// InspectAnnotationPublishAll is used by Inspect to identify containers
	// which have all the ports from their image published.
	// It is expected to be a boolean, populated by one of
	// InspectResponseTrue or InspectResponseFalse.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationPublishAll = "io.podman.annotations.publish-all"
	// InspectAnnotationInit is used by Inspect to identify containers that
	// mount an init binary in.
	// It is expected to be a boolean, populated by one of
	// InspectResponseTrue or InspectResponseFalse.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationInit = "io.podman.annotations.init"
	// InspectAnnotationLabel is used by Inspect to identify containers with
	// special SELinux-related settings. It is used to populate the output
	// of the SecurityOpt setting.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationLabel = "io.podman.annotations.label"
	// InspectAnnotationSeccomp is used by Inspect to identify containers
	// with special Seccomp-related settings. It is used to populate the
	// output of the SecurityOpt setting in Inspect.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationSeccomp = "io.podman.annotations.seccomp"
	// InspectAnnotationApparmor is used by Inspect to identify containers
	// with special Apparmor-related settings. It is used to populate the
	// output of the SecurityOpt setting.
	// If an annotation with this key is found in the OCI spec, it will be
	// used in the output of Inspect().
	InspectAnnotationApparmor = "io.podman.annotations.apparmor"

	// InspectResponseTrue is a boolean True response for an inspect
	// annotation.
	InspectResponseTrue = "TRUE"
	// InspectResponseFalse is a boolean False response for an inspect
	// annotation.
	InspectResponseFalse = "FALSE"
)

// InspectContainerData provides a detailed record of a container's configuration
// and state as viewed by Libpod.
// Large portions of this structure are defined such that the output is
// compatible with `docker inspect` JSON, but additional fields have been added
// as required to share information not in the original output.
type InspectContainerData struct {
	ID              string                      `json:"Id"`
	Created         time.Time                   `json:"Created"`
	Path            string                      `json:"Path"`
	Args            []string                    `json:"Args"`
	State           *InspectContainerState      `json:"State"`
	Image           string                      `json:"Image"`
	ImageName       string                      `json:"ImageName"`
	Rootfs          string                      `json:"Rootfs"`
	Pod             string                      `json:"Pod"`
	ResolvConfPath  string                      `json:"ResolvConfPath"`
	HostnamePath    string                      `json:"HostnamePath"`
	HostsPath       string                      `json:"HostsPath"`
	StaticDir       string                      `json:"StaticDir"`
	OCIConfigPath   string                      `json:"OCIConfigPath,omitempty"`
	OCIRuntime      string                      `json:"OCIRuntime,omitempty"`
	LogPath         string                      `json:"LogPath"`
	ConmonPidFile   string                      `json:"ConmonPidFile"`
	Name            string                      `json:"Name"`
	RestartCount    int32                       `json:"RestartCount"`
	Driver          string                      `json:"Driver"`
	MountLabel      string                      `json:"MountLabel"`
	ProcessLabel    string                      `json:"ProcessLabel"`
	AppArmorProfile string                      `json:"AppArmorProfile"`
	EffectiveCaps   []string                    `json:"EffectiveCaps"`
	BoundingCaps    []string                    `json:"BoundingCaps"`
	ExecIDs         []string                    `json:"ExecIDs"`
	GraphDriver     *driver.Data                `json:"GraphDriver"`
	SizeRw          int64                       `json:"SizeRw,omitempty"`
	SizeRootFs      int64                       `json:"SizeRootFs,omitempty"`
	Mounts          []InspectMount              `json:"Mounts"`
	Dependencies    []string                    `json:"Dependencies"`
	NetworkSettings *InspectNetworkSettings     `json:"NetworkSettings"` //TODO
	ExitCommand     []string                    `json:"ExitCommand"`
	Namespace       string                      `json:"Namespace"`
	IsInfra         bool                        `json:"IsInfra"`
	Config          *InspectContainerConfig     `json:"Config"`
	HostConfig      *InspectContainerHostConfig `json:"HostConfig"`
}

// InspectContainerConfig holds further data about how a container was initially
// configured.
type InspectContainerConfig struct {
	// Container hostname
	Hostname string `json:"Hostname"`
	// Container domain name - unused at present
	DomainName string `json:"Domainname"`
	// User the container was launched with
	User string `json:"User"`
	// Unused, at present
	AttachStdin bool `json:"AttachStdin"`
	// Unused, at present
	AttachStdout bool `json:"AttachStdout"`
	// Unused, at present
	AttachStderr bool `json:"AttachStderr"`
	// Whether the container creates a TTY
	Tty bool `json:"Tty"`
	// Whether the container leaves STDIN open
	OpenStdin bool `json:"OpenStdin"`
	// Whether STDIN is only left open once.
	// Presently not supported by Podman, unused.
	StdinOnce bool `json:"StdinOnce"`
	// Container environment variables
	Env []string `json:"Env"`
	// Container command
	Cmd []string `json:"Cmd"`
	// Container image
	Image string `json:"Image"`
	// Unused, at present. I've never seen this field populated.
	Volumes map[string]struct{} `json:"Volumes"`
	// Container working directory
	WorkingDir string `json:"WorkingDir"`
	// Container entrypoint
	Entrypoint string `json:"Entrypoint"`
	// On-build arguments - presently unused. More of Buildah's domain.
	OnBuild *string `json:"OnBuild"`
	// Container labels
	Labels map[string]string `json:"Labels"`
	// Container annotations
	Annotations map[string]string `json:"Annotations"`
	// Container stop signal
	StopSignal uint `json:"StopSignal"`
	// Configured healthcheck for the container
	Healthcheck *manifest.Schema2HealthConfig `json:"Healthcheck,omitempty"`
}

// InspectContainerHostConfig holds information used when the container was
// created.
// It's very much a Docker-specific struct, retained (mostly) as-is for
// compatibility. We fill individual fields as best as we can, inferring as much
// as possible from the spec and container config.
// Some things cannot be inferred. These will be populated by spec annotations
// (if available).
// Field names are fixed for compatibility and cannot be changed.
// As such, silence lint warnings about them.
//nolint
type InspectContainerHostConfig struct {
	// Binds contains an array of user-added mounts.
	// Both volume mounts and named volumes are included.
	// Tmpfs mounts are NOT included.
	// In 'docker inspect' this is separated into 'Binds' and 'Mounts' based
	// on how a mount was added. We do not make this distinction and do not
	// include a Mounts field in inspect.
	// Format: <src>:<destination>[:<comma-separated options>]
	Binds []string `json:"Binds"`
	// ContainerIDFile is a file created during container creation to hold
	// the ID of the created container.
	// This is not handled within libpod and is stored in an annotation.
	ContainerIDFile string `json:"ContainerIDFile"`
	// LogConfig contains information on the container's logging backend
	LogConfig *InspectLogConfig `json:"LogConfig"`
	// NetworkMode is the configuration of the container's network
	// namespace.
	// Populated as follows:
	// default - A network namespace is being created and configured via CNI
	// none - A network namespace is being created, not configured via CNI
	// host - No network namespace created
	// container:<id> - Using another container's network namespace
	// ns:<path> - A path to a network namespace has been specified
	NetworkMode string `json:"NetworkMode"`
	// PortBindings contains the container's port bindings.
	// It is formatted as map[string][]InspectHostPort.
	// The string key here is formatted as <integer port number>/<protocol>
	// and represents the container port. A single container port may be
	// bound to multiple host ports (on different IPs).
	PortBindings map[string][]InspectHostPort `json:"PortBindings"`
	// RestartPolicy contains the container's restart policy.
	RestartPolicy *InspectRestartPolicy `json:"RestartPolicy"`
	// AutoRemove is whether the container will be automatically removed on
	// exiting.
	// It is not handled directly within libpod and is stored in an
	// annotation.
	AutoRemove bool `json:"AutoRemove"`
	// VolumeDriver is presently unused and is retained for Docker
	// compatibility.
	VolumeDriver string `json:"VolumeDriver"`
	// VolumesFrom is a list of containers which this container uses volumes
	// from. This is not handled directly within libpod and is stored in an
	// annotation.
	// It is formatted as an array of container names and IDs.
	VolumesFrom []string `json:"VolumesFrom"`
	// CapAdd is a list of capabilities added to the container.
	// It is not directly stored by Libpod, and instead computed from the
	// capabilities listed in the container's spec, compared against a set
	// of default capabilities.
	CapAdd []string `json:"CapAdd"`
	// CapDrop is a list of capabilities removed from the container.
	// It is not directly stored by libpod, and instead computed from the
	// capabilities listed in the container's spec, compared against a set
	// of default capabilities.
	CapDrop []string `json:"CapDrop"`
	// Dns is a list of DNS nameservers that will be added to the
	// container's resolv.conf
	Dns []string `json:"Dns"`
	// DnsOptions is a list of DNS options that will be set in the
	// container's resolv.conf
	DnsOptions []string `json:"DnsOptions"`
	// DnsSearch is a list of DNS search domains that will be set in the
	// container's resolv.conf
	DnsSearch []string `json:"DnsSearch"`
	// ExtraHosts contains hosts that will be aded to the container's
	// /etc/hosts.
	ExtraHosts []string `json:"ExtraHosts"`
	// GroupAdd contains groups that the user inside the container will be
	// added to.
	GroupAdd []string `json:"GroupAdd"`
	// IpcMode represents the configuration of the container's IPC
	// namespace.
	// Populated as follows:
	// "" (empty string) - Default, an IPC namespace will be created
	// host - No IPC namespace created
	// container:<id> - Using another container's IPC namespace
	// ns:<path> - A path to an IPC namespace has been specified
	IpcMode string `json:"IpcMode"`
	// Cgroup contains the container's cgroup. It is presently not
	// populated.
	// TODO.
	Cgroup string `json:"Cgroup"`
	// Cgroups contains the container's CGroup mode.
	// Allowed values are "default" (container is creating CGroups) and
	// "disabled" (container is not creating CGroups).
	// This is Libpod-specific and not included in `docker inspect`.
	Cgroups string `json:"Cgroups"`
	// Links is unused, and provided purely for Docker compatibility.
	Links []string `json:"Links"`
	// OOMScoreAdj is an adjustment that will be made to the container's OOM
	// score.
	OomScoreAdj int `json:"OomScoreAdj"`
	// PidMode represents the configuration of the container's PID
	// namespace.
	// Populated as follows:
	// "" (empty string) - Default, a PID namespace will be created
	// host - No PID namespace created
	// container:<id> - Using another container's PID namespace
	// ns:<path> - A path to a PID namespace has been specified
	PidMode string `json:"PidMode"`
	// Privileged indicates whether the container is running with elevated
	// privileges.
	// This has a very specific meaning in the Docker sense, so it's very
	// difficult to decode from the spec and config, and so is stored as an
	// annotation.
	Privileged bool `json:"Privileged"`
	// PublishAllPorts indicates whether image ports are being published.
	// This is not directly stored in libpod and is saved as an annotation.
	PublishAllPorts bool `json:"PublishAllPorts"`
	// ReadonlyRootfs is whether the container will be mounted read-only.
	ReadonlyRootfs bool `json:"ReadonlyRootfs"`
	// SecurityOpt is a list of security-related options that are set in the
	// container.
	SecurityOpt []string `json:"SecurityOpt"`
	// Tmpfs is a list of tmpfs filesystems that will be mounted into the
	// container.
	// It is a map of destination path to options for the mount.
	Tmpfs map[string]string `json:"Tmpfs"`
	// UTSMode represents the configuration of the container's UID
	// namespace.
	// Populated as follows:
	// "" (empty string) - Default, a UTS namespace will be created
	// host - no UTS namespace created
	// container:<id> - Using another container's UTS namespace
	// ns:<path> - A path to a UTS namespace has been specified
	UTSMode string `json:"UTSMode"`
	// UsernsMode represents the configuration of the container's user
	// namespace.
	// When running rootless, a user namespace is created outside of libpod
	// to allow some privileged operations. This will not be reflected here.
	// Populated as follows:
	// "" (empty string) - No user namespace will be created
	// private - The container will be run in a user namespace
	// container:<id> - Using another container's user namespace
	// ns:<path> - A path to a user namespace has been specified
	// TODO Rootless has an additional 'keep-id' option, presently not
	// reflected here.
	UsernsMode string `json:"UsernsMode"`
	// ShmSize is the size of the container's SHM device.
	ShmSize int64 `json:"ShmSize"`
	// Runtime is provided purely for Docker compatibility.
	// It is set unconditionally to "oci" as Podman does not presently
	// support non-OCI runtimes.
	Runtime string `json:"Runtime"`
	// ConsoleSize is an array of 2 integers showing the size of the
	// container's console.
	// It is only set if the container is creating a terminal.
	// TODO.
	ConsoleSize []uint `json:"ConsoleSize"`
	// Isolation is presently unused and provided solely for Docker
	// compatibility.
	Isolation string `json:"Isolation"`
	// CpuShares indicates the CPU resources allocated to the container.
	// It is a relative weight in the scheduler for assigning CPU time
	// versus other CGroups.
	CpuShares uint64 `json:"CpuShares"`
	// Memory indicates the memory resources allocated to the container.
	// This is the limit (in bytes) of RAM the container may use.
	Memory int64 `json:"Memory"`
	// NanoCpus indicates number of CPUs allocated to the container.
	// It is an integer where one full CPU is indicated by 1000000000 (one
	// billion).
	// Thus, 2.5 CPUs (fractional portions of CPUs are allowed) would be
	// 2500000000 (2.5 billion).
	// In 'docker inspect' this is set exclusively of two further options in
	// the output (CpuPeriod and CpuQuota) which are both used to implement
	// this functionality.
	// We can't distinguish here, so if CpuQuota is set to the default of
	// 100000, we will set both CpuQuota, CpuPeriod, and NanoCpus. If
	// CpuQuota is not the default, we will not set NanoCpus.
	NanoCpus int64 `json:"NanoCpus"`
	// CgroupParent is the CGroup parent of the container.
	// Only set if not default.
	CgroupParent string `json:"CgroupParent"`
	// BlkioWeight indicates the I/O resources allocated to the container.
	// It is a relative weight in the scheduler for assigning I/O time
	// versus other CGroups.
	BlkioWeight uint16 `json:"BlkioWeight"`
	// BlkioWeightDevice is an array of I/O resource priorities for
	// individual device nodes.
	// Unfortunately, the spec only stores the device's Major/Minor numbers
	// and not the path, which is used here.
	// Fortunately, the kernel provides an interface for retrieving the path
	// of a given node by major:minor at /sys/dev/. However, the exact path
	// in use may not be what was used in the original CLI invocation -
	// though it is guaranteed that the device node will be the same, and
	// using the given path will be functionally identical.
	BlkioWeightDevice []InspectBlkioWeightDevice `json:"BlkioWeightDevice"`
	// BlkioDeviceReadBps is an array of I/O throttle parameters for
	// individual device nodes.
	// This specifically sets read rate cap in bytes per second for device
	// nodes.
	// As with BlkioWeightDevice, we pull the path from /sys/dev, and we
	// don't guarantee the path will be identical to the original (though
	// the node will be).
	BlkioDeviceReadBps []InspectBlkioThrottleDevice `json:"BlkioDeviceReadBps"`
	// BlkioDeviceWriteBps is an array of I/O throttle parameters for
	// individual device nodes.
	// this specifically sets write rate cap in bytes per second for device
	// nodes.
	// as with BlkioWeightDevice, we pull the path from /sys/dev, and we
	// don't guarantee the path will be identical to the original (though
	// the node will be).
	BlkioDeviceWriteBps []InspectBlkioThrottleDevice `json:"BlkioDeviceWriteBps"`
	// BlkioDeviceReadIOps is an array of I/O throttle parameters for
	// individual device nodes.
	// This specifically sets the read rate cap in iops per second for
	// device nodes.
	// As with BlkioWeightDevice, we pull the path from /sys/dev, and we
	// don't guarantee the path will be identical to the original (though
	// the node will be).
	BlkioDeviceReadIOps []InspectBlkioThrottleDevice `json:"BlkioDeviceReadIOps"`
	// BlkioDeviceWriteIOps is an array of I/O throttle parameters for
	// individual device nodes.
	// This specifically sets the write rate cap in iops per second for
	// device nodes.
	// As with BlkioWeightDevice, we pull the path from /sys/dev, and we
	// don't guarantee the path will be identical to the original (though
	// the node will be).
	BlkioDeviceWriteIOps []InspectBlkioThrottleDevice `json:"BlkioDeviceWriteIOps"`
	// CpuPeriod is the length of a CPU period in microseconds.
	// It relates directly to CpuQuota.
	CpuPeriod uint64 `json:"CpuPeriod"`
	// CpuPeriod is the amount of time (in microseconds) that a container
	// can use the CPU in every CpuPeriod.
	CpuQuota int64 `json:"CpuQuota"`
	// CpuRealtimePeriod is the length of time (in microseconds) of the CPU
	// realtime period. If set to 0, no time will be allocated to realtime
	// tasks.
	CpuRealtimePeriod uint64 `json:"CpuRealtimePeriod"`
	// CpuRealtimeRuntime is the length of time (in microseconds) allocated
	// for realtime tasks within every CpuRealtimePeriod.
	CpuRealtimeRuntime int64 `json:"CpuRealtimeRuntime"`
	// CpusetCpus is the is the set of CPUs that the container will execute
	// on. Formatted as `0-3` or `0,2`. Default (if unset) is all CPUs.
	CpusetCpus string `json:"CpusetCpus"`
	// CpusetMems is the set of memory nodes the container will use.
	// Formatted as `0-3` or `0,2`. Default (if unset) is all memory nodes.
	CpusetMems string `json:"CpusetMems"`
	// Devices is a list of device nodes that will be added to the
	// container.
	// These are stored in the OCI spec only as type, major, minor while we
	// display the host path. We convert this with /sys/dev, but we cannot
	// guarantee that the host path will be identical - only that the actual
	// device will be.
	Devices []InspectDevice `json:"Devices"`
	// DiskQuota is the maximum amount of disk space the container may use
	// (in bytes).
	// Presently not populated.
	// TODO.
	DiskQuota uint64 `json:"DiskQuota"`
	// KernelMemory is the maximum amount of memory the kernel will devote
	// to the container.
	KernelMemory int64 `json:"KernelMemory"`
	// MemoryReservation is the reservation (soft limit) of memory available
	// to the container. Soft limits are warnings only and can be exceeded.
	MemoryReservation int64 `json:"MemoryReservation"`
	// MemorySwap is the total limit for all memory available to the
	// container, including swap. 0 indicates that there is no limit to the
	// amount of memory available.
	MemorySwap int64 `json:"MemorySwap"`
	// MemorySwappiness is the willingness of the kernel to page container
	// memory to swap. It is an integer from 0 to 100, with low numbers
	// being more likely to be put into swap.
	// -1, the default, will not set swappiness and use the system defaults.
	MemorySwappiness int64 `json:"MemorySwappiness"`
	// OomKillDisable indicates whether the kernel OOM killer is disabled
	// for the container.
	OomKillDisable bool `json:"OomKillDisable"`
	// Init indicates whether the container has an init mounted into it.
	Init bool `json:"Init,omitempty"`
	// PidsLimit is the maximum number of PIDs what may be created within
	// the container. 0, the default, indicates no limit.
	PidsLimit int64 `json:"PidsLimit"`
	// Ulimits is a set of ulimits that will be set within the container.
	Ulimits []InspectUlimit `json:"Ulimits"`
	// CpuCount is Windows-only and not presently implemented.
	CpuCount uint64 `json:"CpuCount"`
	// CpuPercent is Windows-only and not presently implemented.
	CpuPercent uint64 `json:"CpuPercent"`
	// IOMaximumIOps is Windows-only and not presently implemented.
	IOMaximumIOps uint64 `json:"IOMaximumIOps"`
	// IOMaximumBandwidth is Windows-only and not presently implemented.
	IOMaximumBandwidth uint64 `json:"IOMaximumBandwidth"`
}

// InspectLogConfig holds information about a container's configured log driver
// and is presently unused. It is retained for Docker compatibility.
type InspectLogConfig struct {
	Type   string            `json:"Type"`
	Config map[string]string `json:"Config"` //idk type, TODO
}

// InspectRestartPolicy holds information about the container's restart policy.
type InspectRestartPolicy struct {
	// Name contains the container's restart policy.
	// Allowable values are "no" or "" (take no action),
	// "on-failure" (restart on non-zero exit code, with an optional max
	// retry count), and "always" (always restart on container stop, unless
	// explicitly requested by API).
	// Note that this is NOT actually a name of any sort - the poor naming
	// is for Docker compatibility.
	Name string `json:"Name"`
	// MaximumRetryCount is the maximum number of retries allowed if the
	// "on-failure" restart policy is in use. Not used if "on-failure" is
	// not set.
	MaximumRetryCount uint `json:"MaximumRetryCount"`
}

// InspectBlkioWeightDevice holds information about the relative weight
// of an individual device node. Weights are used in the I/O scheduler to give
// relative priority to some accesses.
type InspectBlkioWeightDevice struct {
	// Path is the path to the device this applies to.
	Path string `json:"Path"`
	// Weight is the relative weight the scheduler will use when scheduling
	// I/O.
	Weight uint16 `json:"Weight"`
}

// InspectBlkioThrottleDevice holds information about a speed cap for a device
// node. This cap applies to a specific operation (read, write, etc) on the given
// node.
type InspectBlkioThrottleDevice struct {
	// Path is the path to the device this applies to.
	Path string `json:"Path"`
	// Rate is the maximum rate. It is in either bytes per second or iops
	// per second, determined by where it is used - documentation will
	// indicate which is appropriate.
	Rate uint64 `json:"Rate"`
}

// InspectUlimit is a ulimit that will be applied to the container.
type InspectUlimit struct {
	// Name is the name (type) of the ulimit.
	Name string `json:"Name"`
	// Soft is the soft limit that will be applied.
	Soft uint64 `json:"Soft"`
	// Hard is the hard limit that will be applied.
	Hard uint64 `json:"Hard"`
}

// InspectMount provides a record of a single mount in a container. It contains
// fields for both named and normal volumes. Only user-specified volumes will be
// included, and tmpfs volumes are not included even if the user specified them.
type InspectMount struct {
	// Whether the mount is a volume or bind mount. Allowed values are
	// "volume" and "bind".
	Type string `json:"Type"`
	// The name of the volume. Empty for bind mounts.
	Name string `json:"Name,omptempty"`
	// The source directory for the volume.
	Source string `json:"Source"`
	// The destination directory for the volume. Specified as a path within
	// the container, as it would be passed into the OCI runtime.
	Destination string `json:"Destination"`
	// The driver used for the named volume. Empty for bind mounts.
	Driver string `json:"Driver"`
	// Contains SELinux :z/:Z mount options. Unclear what, if anything, else
	// goes in here.
	Mode string `json:"Mode"`
	// All remaining mount options. Additional data, not present in the
	// original output.
	Options []string `json:"Options"`
	// Whether the volume is read-write
	RW bool `json:"RW"`
	// Mount propagation for the mount. Can be empty if not specified, but
	// is always printed - no omitempty.
	Propagation string `json:"Propagation"`
}

// InspectDevice is a single device that will be mounted into the container.
type InspectDevice struct {
	// PathOnHost is the path of the device on the host.
	PathOnHost string `json:"PathOnHost"`
	// PathInContainer is the path of the device within the container.
	PathInContainer string `json:"PathInContainer"`
	// CgroupPermissions is the permissions of the mounted device.
	// Presently not populated.
	// TODO.
	CgroupPermissions string `json:"CgroupPermissions"`
}

// InspectHostPort provides information on a port on the host that a container's
// port is bound to.
type InspectHostPort struct {
	// IP on the host we are bound to. "" if not specified (binding to all
	// IPs).
	HostIP string `json:"HostIp"`
	// Port on the host we are bound to. No special formatting - just an
	// integer stuffed into a string.
	HostPort string `json:"HostPort"`
}

// InspectContainerState provides a detailed record of a container's current
// state. It is returned as part of InspectContainerData.
// As with InspectContainerData, many portions of this struct are matched to
// Docker, but here we see more fields that are unused (nonsensical in the
// context of Libpod).
type InspectContainerState struct {
	OciVersion  string             `json:"OciVersion"`
	Status      string             `json:"Status"`
	Running     bool               `json:"Running"`
	Paused      bool               `json:"Paused"`
	Restarting  bool               `json:"Restarting"` // TODO
	OOMKilled   bool               `json:"OOMKilled"`
	Dead        bool               `json:"Dead"`
	Pid         int                `json:"Pid"`
	ConmonPid   int                `json:"ConmonPid,omitempty"`
	ExitCode    int32              `json:"ExitCode"`
	Error       string             `json:"Error"` // TODO
	StartedAt   time.Time          `json:"StartedAt"`
	FinishedAt  time.Time          `json:"FinishedAt"`
	Healthcheck HealthCheckResults `json:"Healthcheck,omitempty"`
}

// InspectNetworkSettings holds information about the network settings of the
// container.
// Many fields are maintained only for compatibility with `docker inspect` and
// are unused within Libpod.
type InspectNetworkSettings struct {
	Bridge                 string               `json:"Bridge"`
	SandboxID              string               `json:"SandboxID"`
	HairpinMode            bool                 `json:"HairpinMode"`
	LinkLocalIPv6Address   string               `json:"LinkLocalIPv6Address"`
	LinkLocalIPv6PrefixLen int                  `json:"LinkLocalIPv6PrefixLen"`
	Ports                  []ocicni.PortMapping `json:"Ports"`
	SandboxKey             string               `json:"SandboxKey"`
	SecondaryIPAddresses   []string             `json:"SecondaryIPAddresses"`
	SecondaryIPv6Addresses []string             `json:"SecondaryIPv6Addresses"`
	EndpointID             string               `json:"EndpointID"`
	Gateway                string               `json:"Gateway"`
	GlobalIPv6Address      string               `json:"GlobalIPv6Address"`
	GlobalIPv6PrefixLen    int                  `json:"GlobalIPv6PrefixLen"`
	IPAddress              string               `json:"IPAddress"`
	IPPrefixLen            int                  `json:"IPPrefixLen"`
	IPv6Gateway            string               `json:"IPv6Gateway"`
	MacAddress             string               `json:"MacAddress"`
}

// Inspect a container for low-level information
func (c *Container) Inspect(size bool) (*InspectContainerData, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	storeCtr, err := c.runtime.store.Container(c.ID())
	if err != nil {
		return nil, errors.Wrapf(err, "error getting container from store %q", c.ID())
	}
	layer, err := c.runtime.store.Layer(storeCtr.LayerID)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading information about layer %q", storeCtr.LayerID)
	}
	driverData, err := driver.GetDriverData(c.runtime.store, layer.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting graph driver info %q", c.ID())
	}
	return c.getContainerInspectData(size, driverData)
}

func (c *Container) getContainerInspectData(size bool, driverData *driver.Data) (*InspectContainerData, error) {
	config := c.config
	runtimeInfo := c.state
	ctrSpec, err := c.specFromState()
	if err != nil {
		return nil, err
	}

	// Process is allowed to be nil in the stateSpec
	args := []string{}
	if config.Spec.Process != nil {
		args = config.Spec.Process.Args
	}
	var path string
	if len(args) > 0 {
		path = args[0]
	}
	if len(args) > 1 {
		args = args[1:]
	}

	execIDs := []string{}
	for id := range c.state.ExecSessions {
		execIDs = append(execIDs, id)
	}

	resolvPath := ""
	hostsPath := ""
	hostnamePath := ""
	if c.state.BindMounts != nil {
		if getPath, ok := c.state.BindMounts["/etc/resolv.conf"]; ok {
			resolvPath = getPath
		}
		if getPath, ok := c.state.BindMounts["/etc/hosts"]; ok {
			hostsPath = getPath
		}
		if getPath, ok := c.state.BindMounts["/etc/hostname"]; ok {
			hostnamePath = getPath
		}
	}

	namedVolumes, mounts := c.sortUserVolumes(ctrSpec)
	inspectMounts, err := c.getInspectMounts(ctrSpec, namedVolumes, mounts)
	if err != nil {
		return nil, err
	}

	data := &InspectContainerData{
		ID:      config.ID,
		Created: config.CreatedTime,
		Path:    path,
		Args:    args,
		State: &InspectContainerState{
			OciVersion: ctrSpec.Version,
			Status:     runtimeInfo.State.String(),
			Running:    runtimeInfo.State == define.ContainerStateRunning,
			Paused:     runtimeInfo.State == define.ContainerStatePaused,
			OOMKilled:  runtimeInfo.OOMKilled,
			Dead:       runtimeInfo.State.String() == "bad state",
			Pid:        runtimeInfo.PID,
			ConmonPid:  runtimeInfo.ConmonPID,
			ExitCode:   runtimeInfo.ExitCode,
			Error:      "", // can't get yet
			StartedAt:  runtimeInfo.StartedTime,
			FinishedAt: runtimeInfo.FinishedTime,
		},
		Image:           config.RootfsImageID,
		ImageName:       config.RootfsImageName,
		ExitCommand:     config.ExitCommand,
		Namespace:       config.Namespace,
		Rootfs:          config.Rootfs,
		Pod:             config.Pod,
		ResolvConfPath:  resolvPath,
		HostnamePath:    hostnamePath,
		HostsPath:       hostsPath,
		StaticDir:       config.StaticDir,
		LogPath:         config.LogPath,
		OCIRuntime:      config.OCIRuntime,
		ConmonPidFile:   config.ConmonPidFile,
		Name:            config.Name,
		RestartCount:    int32(runtimeInfo.RestartCount),
		Driver:          driverData.Name,
		MountLabel:      config.MountLabel,
		ProcessLabel:    config.ProcessLabel,
		EffectiveCaps:   ctrSpec.Process.Capabilities.Effective,
		BoundingCaps:    ctrSpec.Process.Capabilities.Bounding,
		AppArmorProfile: ctrSpec.Process.ApparmorProfile,
		ExecIDs:         execIDs,
		GraphDriver:     driverData,
		Mounts:          inspectMounts,
		Dependencies:    c.Dependencies(),
		NetworkSettings: &InspectNetworkSettings{
			Bridge:                 "",    // TODO
			SandboxID:              "",    // TODO - is this even relevant?
			HairpinMode:            false, // TODO
			LinkLocalIPv6Address:   "",    // TODO - do we even support IPv6?
			LinkLocalIPv6PrefixLen: 0,     // TODO - do we even support IPv6?

			Ports:                  []ocicni.PortMapping{}, // TODO - maybe worth it to put this in Docker format?
			SandboxKey:             "",                     // Network namespace path
			SecondaryIPAddresses:   nil,                    // TODO - do we support this?
			SecondaryIPv6Addresses: nil,                    // TODO - do we support this?
			EndpointID:             "",                     // TODO - is this even relevant?
			Gateway:                "",                     // TODO
			GlobalIPv6Address:      "",
			GlobalIPv6PrefixLen:    0,
			IPAddress:              "",
			IPPrefixLen:            0,
			IPv6Gateway:            "",
			MacAddress:             "", // TODO
		},
		IsInfra: c.IsInfra(),
	}

	if c.state.ConfigPath != "" {
		data.OCIConfigPath = c.state.ConfigPath
	}

	if c.config.HealthCheckConfig != nil {
		// This container has a healthcheck defined in it; we need to add it's state
		healthCheckState, err := c.GetHealthCheckLog()
		if err != nil {
			// An error here is not considered fatal; no health state will be displayed
			logrus.Error(err)
		} else {
			data.State.Healthcheck = healthCheckState
		}
	}

	// Copy port mappings into network settings
	if config.PortMappings != nil {
		data.NetworkSettings.Ports = config.PortMappings
	}

	// Get information on the container's network namespace (if present)
	data = c.getContainerNetworkInfo(data)

	inspectConfig, err := c.generateInspectContainerConfig(ctrSpec)
	if err != nil {
		return nil, err
	}
	data.Config = inspectConfig

	hostConfig, err := c.generateInspectContainerHostConfig(ctrSpec, namedVolumes, mounts)
	if err != nil {
		return nil, err
	}
	data.HostConfig = hostConfig

	if size {
		rootFsSize, err := c.rootFsSize()
		if err != nil {
			logrus.Errorf("error getting rootfs size %q: %v", config.ID, err)
		}
		rwSize, err := c.rwSize()
		if err != nil {
			logrus.Errorf("error getting rw size %q: %v", config.ID, err)
		}
		data.SizeRootFs = rootFsSize
		data.SizeRw = rwSize
	}
	return data, nil
}

// Get inspect-formatted mounts list.
// Only includes user-specified mounts. Only includes bind mounts and named
// volumes, not tmpfs volumes.
func (c *Container) getInspectMounts(ctrSpec *spec.Spec, namedVolumes []*ContainerNamedVolume, mounts []spec.Mount) ([]InspectMount, error) {
	inspectMounts := []InspectMount{}

	// No mounts, return early
	if len(c.config.UserVolumes) == 0 {
		return inspectMounts, nil
	}

	for _, volume := range namedVolumes {
		mountStruct := InspectMount{}
		mountStruct.Type = "volume"
		mountStruct.Destination = volume.Dest
		mountStruct.Name = volume.Name

		// For src and driver, we need to look up the named
		// volume.
		volFromDB, err := c.runtime.state.Volume(volume.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking up volume %s in container %s config", volume.Name, c.ID())
		}
		mountStruct.Driver = volFromDB.Driver()
		mountStruct.Source = volFromDB.MountPoint()

		parseMountOptionsForInspect(volume.Options, &mountStruct)

		inspectMounts = append(inspectMounts, mountStruct)
	}
	for _, mount := range mounts {
		// It's a mount.
		// Is it a tmpfs? If so, discard.
		if mount.Type == "tmpfs" {
			continue
		}

		mountStruct := InspectMount{}
		mountStruct.Type = "bind"
		mountStruct.Source = mount.Source
		mountStruct.Destination = mount.Destination

		parseMountOptionsForInspect(mount.Options, &mountStruct)

		inspectMounts = append(inspectMounts, mountStruct)
	}

	return inspectMounts, nil
}

// Parse mount options so we can populate them in the mount structure.
// The mount passed in will be modified.
func parseMountOptionsForInspect(options []string, mount *InspectMount) {
	isRW := true
	mountProp := ""
	zZ := ""
	otherOpts := []string{}

	// Some of these may be overwritten if the user passes us garbage opts
	// (for example, [ro,rw])
	// We catch these on the Podman side, so not a problem there, but other
	// users of libpod who do not properly validate mount options may see
	// this.
	// Not really worth dealing with on our end - garbage in, garbage out.
	for _, opt := range options {
		switch opt {
		case "ro":
			isRW = false
		case "rw":
			// Do nothing, silently discard
		case "shared", "slave", "private", "rshared", "rslave", "rprivate":
			mountProp = opt
		case "z", "Z":
			zZ = opt
		default:
			otherOpts = append(otherOpts, opt)
		}
	}

	mount.RW = isRW
	mount.Propagation = mountProp
	mount.Mode = zZ
	mount.Options = otherOpts
}

// Generate the InspectContainerConfig struct for the Config field of Inspect.
func (c *Container) generateInspectContainerConfig(spec *spec.Spec) (*InspectContainerConfig, error) {
	ctrConfig := new(InspectContainerConfig)

	ctrConfig.Hostname = c.Hostname()
	ctrConfig.User = c.config.User
	if spec.Process != nil {
		ctrConfig.Tty = spec.Process.Terminal
		ctrConfig.Env = []string{}
		ctrConfig.Env = append(ctrConfig.Env, spec.Process.Env...)
		ctrConfig.WorkingDir = spec.Process.Cwd
	}

	ctrConfig.OpenStdin = c.config.Stdin
	ctrConfig.Image = c.config.RootfsImageName

	// Leave empty is not explicitly overwritten by user
	if len(c.config.Command) != 0 {
		ctrConfig.Cmd = []string{}
		ctrConfig.Cmd = append(ctrConfig.Cmd, c.config.Command...)
	}

	// Leave empty if not explicitly overwritten by user
	if len(c.config.Entrypoint) != 0 {
		ctrConfig.Entrypoint = strings.Join(c.config.Entrypoint, " ")
	}

	if len(c.config.Labels) != 0 {
		ctrConfig.Labels = make(map[string]string)
		for k, v := range c.config.Labels {
			ctrConfig.Labels[k] = v
		}
	}

	if len(spec.Annotations) != 0 {
		ctrConfig.Annotations = make(map[string]string)
		for k, v := range spec.Annotations {
			ctrConfig.Annotations[k] = v
		}
	}

	ctrConfig.StopSignal = c.config.StopSignal
	// TODO: should JSON deep copy this to ensure internal pointers don't
	// leak.
	ctrConfig.Healthcheck = c.config.HealthCheckConfig

	return ctrConfig, nil
}

// Generate the InspectContainerHostConfig struct for the HostConfig field of
// Inspect.
func (c *Container) generateInspectContainerHostConfig(ctrSpec *spec.Spec, namedVolumes []*ContainerNamedVolume, mounts []spec.Mount) (*InspectContainerHostConfig, error) {
	hostConfig := new(InspectContainerHostConfig)

	logConfig := new(InspectLogConfig)
	logConfig.Type = c.config.LogDriver
	hostConfig.LogConfig = logConfig

	restartPolicy := new(InspectRestartPolicy)
	restartPolicy.Name = c.config.RestartPolicy
	restartPolicy.MaximumRetryCount = c.config.RestartRetries
	hostConfig.RestartPolicy = restartPolicy
	if c.config.NoCgroups {
		hostConfig.Cgroups = "disabled"
	} else {
		hostConfig.Cgroups = "default"
	}

	hostConfig.Dns = make([]string, 0, len(c.config.DNSServer))
	for _, dns := range c.config.DNSServer {
		hostConfig.Dns = append(hostConfig.Dns, dns.String())
	}

	hostConfig.DnsOptions = make([]string, 0, len(c.config.DNSOption))
	hostConfig.DnsOptions = append(hostConfig.DnsOptions, c.config.DNSOption...)

	hostConfig.DnsSearch = make([]string, 0, len(c.config.DNSSearch))
	hostConfig.DnsSearch = append(hostConfig.DnsSearch, c.config.DNSSearch...)

	hostConfig.ExtraHosts = make([]string, 0, len(c.config.HostAdd))
	hostConfig.ExtraHosts = append(hostConfig.ExtraHosts, c.config.HostAdd...)

	hostConfig.GroupAdd = make([]string, 0, len(c.config.Groups))
	hostConfig.GroupAdd = append(hostConfig.GroupAdd, c.config.Groups...)

	hostConfig.SecurityOpt = []string{}
	if ctrSpec.Process != nil {
		if ctrSpec.Process.OOMScoreAdj != nil {
			hostConfig.OomScoreAdj = *ctrSpec.Process.OOMScoreAdj
		}
		if ctrSpec.Process.NoNewPrivileges {
			hostConfig.SecurityOpt = append(hostConfig.SecurityOpt, "no-new-privileges")
		}
	}

	hostConfig.ReadonlyRootfs = ctrSpec.Root.Readonly
	hostConfig.ShmSize = c.config.ShmSize
	hostConfig.Runtime = "oci"

	// This is very expensive to initialize.
	// So we don't want to initialize it unless we absolutely have to - IE,
	// there are things that require a major:minor to path translation.
	var deviceNodes map[string]string

	// Annotations
	if ctrSpec.Annotations != nil {
		hostConfig.ContainerIDFile = ctrSpec.Annotations[InspectAnnotationCIDFile]
		if ctrSpec.Annotations[InspectAnnotationAutoremove] == InspectResponseTrue {
			hostConfig.AutoRemove = true
		}
		if ctrs, ok := ctrSpec.Annotations[InspectAnnotationVolumesFrom]; ok {
			hostConfig.VolumesFrom = strings.Split(ctrs, ",")
		}
		if ctrSpec.Annotations[InspectAnnotationPrivileged] == InspectResponseTrue {
			hostConfig.Privileged = true
		}
		if ctrSpec.Annotations[InspectAnnotationInit] == InspectResponseTrue {
			hostConfig.Init = true
		}
		if label, ok := ctrSpec.Annotations[InspectAnnotationLabel]; ok {
			hostConfig.SecurityOpt = append(hostConfig.SecurityOpt, fmt.Sprintf("label=%s", label))
		}
		if seccomp, ok := ctrSpec.Annotations[InspectAnnotationSeccomp]; ok {
			hostConfig.SecurityOpt = append(hostConfig.SecurityOpt, fmt.Sprintf("seccomp=%s", seccomp))
		}
		if apparmor, ok := ctrSpec.Annotations[InspectAnnotationApparmor]; ok {
			hostConfig.SecurityOpt = append(hostConfig.SecurityOpt, fmt.Sprintf("apparmor=%s", apparmor))
		}
	}

	// Resource limits
	if ctrSpec.Linux != nil {
		if ctrSpec.Linux.Resources != nil {
			if ctrSpec.Linux.Resources.CPU != nil {
				if ctrSpec.Linux.Resources.CPU.Shares != nil {
					hostConfig.CpuShares = *ctrSpec.Linux.Resources.CPU.Shares
				}
				if ctrSpec.Linux.Resources.CPU.Period != nil {
					hostConfig.CpuPeriod = *ctrSpec.Linux.Resources.CPU.Period
				}
				if ctrSpec.Linux.Resources.CPU.Quota != nil {
					hostConfig.CpuQuota = *ctrSpec.Linux.Resources.CPU.Quota
				}
				if ctrSpec.Linux.Resources.CPU.RealtimePeriod != nil {
					hostConfig.CpuRealtimePeriod = *ctrSpec.Linux.Resources.CPU.RealtimePeriod
				}
				if ctrSpec.Linux.Resources.CPU.RealtimeRuntime != nil {
					hostConfig.CpuRealtimeRuntime = *ctrSpec.Linux.Resources.CPU.RealtimeRuntime
				}
				hostConfig.CpusetCpus = ctrSpec.Linux.Resources.CPU.Cpus
				hostConfig.CpusetMems = ctrSpec.Linux.Resources.CPU.Mems
			}
			if ctrSpec.Linux.Resources.Memory != nil {
				if ctrSpec.Linux.Resources.Memory.Limit != nil {
					hostConfig.Memory = *ctrSpec.Linux.Resources.Memory.Limit
				}
				if ctrSpec.Linux.Resources.Memory.Kernel != nil {
					hostConfig.KernelMemory = *ctrSpec.Linux.Resources.Memory.Kernel
				}
				if ctrSpec.Linux.Resources.Memory.Reservation != nil {
					hostConfig.MemoryReservation = *ctrSpec.Linux.Resources.Memory.Reservation
				}
				if ctrSpec.Linux.Resources.Memory.Swap != nil {
					hostConfig.MemorySwap = *ctrSpec.Linux.Resources.Memory.Swap
				}
				if ctrSpec.Linux.Resources.Memory.Swappiness != nil {
					hostConfig.MemorySwappiness = int64(*ctrSpec.Linux.Resources.Memory.Swappiness)
				} else {
					// Swappiness has a default of -1
					hostConfig.MemorySwappiness = -1
				}
				if ctrSpec.Linux.Resources.Memory.DisableOOMKiller != nil {
					hostConfig.OomKillDisable = *ctrSpec.Linux.Resources.Memory.DisableOOMKiller
				}
			}
			if ctrSpec.Linux.Resources.Pids != nil {
				hostConfig.PidsLimit = ctrSpec.Linux.Resources.Pids.Limit
			}
			if ctrSpec.Linux.Resources.BlockIO != nil {
				if ctrSpec.Linux.Resources.BlockIO.Weight != nil {
					hostConfig.BlkioWeight = *ctrSpec.Linux.Resources.BlockIO.Weight
				}
				hostConfig.BlkioWeightDevice = []InspectBlkioWeightDevice{}
				for _, dev := range ctrSpec.Linux.Resources.BlockIO.WeightDevice {
					key := fmt.Sprintf("%d:%d", dev.Major, dev.Minor)
					// TODO: how do we handle LeafWeight vs
					// Weight? For now, ignore anything
					// without Weight set.
					if dev.Weight == nil {
						logrus.Warnf("Ignoring weight device %s as it lacks a weight", key)
						continue
					}
					if deviceNodes == nil {
						nodes, err := util.FindDeviceNodes()
						if err != nil {
							return nil, err
						}
						deviceNodes = nodes
					}
					path, ok := deviceNodes[key]
					if !ok {
						logrus.Warnf("Could not locate weight device %s in system devices", key)
						continue
					}
					weightDev := InspectBlkioWeightDevice{}
					weightDev.Path = path
					weightDev.Weight = *dev.Weight
					hostConfig.BlkioWeightDevice = append(hostConfig.BlkioWeightDevice, weightDev)
				}

				handleThrottleDevice := func(devs []spec.LinuxThrottleDevice) ([]InspectBlkioThrottleDevice, error) {
					out := []InspectBlkioThrottleDevice{}
					for _, dev := range devs {
						key := fmt.Sprintf("%d:%d", dev.Major, dev.Minor)
						if deviceNodes == nil {
							nodes, err := util.FindDeviceNodes()
							if err != nil {
								return nil, err
							}
							deviceNodes = nodes
						}
						path, ok := deviceNodes[key]
						if !ok {
							logrus.Warnf("Could not locate throttle device %s in system devices", key)
							continue
						}
						throttleDev := InspectBlkioThrottleDevice{}
						throttleDev.Path = path
						throttleDev.Rate = dev.Rate
						out = append(out, throttleDev)
					}
					return out, nil
				}

				readBps, err := handleThrottleDevice(ctrSpec.Linux.Resources.BlockIO.ThrottleReadBpsDevice)
				if err != nil {
					return nil, err
				}
				hostConfig.BlkioDeviceReadBps = readBps

				writeBps, err := handleThrottleDevice(ctrSpec.Linux.Resources.BlockIO.ThrottleWriteBpsDevice)
				if err != nil {
					return nil, err
				}
				hostConfig.BlkioDeviceWriteBps = writeBps

				readIops, err := handleThrottleDevice(ctrSpec.Linux.Resources.BlockIO.ThrottleReadIOPSDevice)
				if err != nil {
					return nil, err
				}
				hostConfig.BlkioDeviceReadIOps = readIops

				writeIops, err := handleThrottleDevice(ctrSpec.Linux.Resources.BlockIO.ThrottleWriteIOPSDevice)
				if err != nil {
					return nil, err
				}
				hostConfig.BlkioDeviceWriteIOps = writeIops
			}
		}
	}

	// NanoCPUs.
	// This is only calculated if CpuPeriod == 100000.
	// It is given in nanoseconds, versus the microseconds used elsewhere -
	// so multiply by 10000 (not sure why, but 1000 is off by 10).
	if hostConfig.CpuPeriod == 100000 {
		hostConfig.NanoCpus = 10000 * hostConfig.CpuQuota
	}

	// Bind mounts, formatted as src:dst.
	// We'll be appending some options that aren't necessarily in the
	// original command line... but no helping that from inside libpod.
	binds := []string{}
	tmpfs := make(map[string]string)
	for _, namedVol := range namedVolumes {
		if len(namedVol.Options) > 0 {
			binds = append(binds, fmt.Sprintf("%s:%s:%s", namedVol.Name, namedVol.Dest, strings.Join(namedVol.Options, ",")))
		} else {
			binds = append(binds, fmt.Sprintf("%s:%s", namedVol.Name, namedVol.Dest))
		}
	}
	for _, mount := range mounts {
		if mount.Type == "tmpfs" {
			tmpfs[mount.Destination] = strings.Join(mount.Options, ",")
		} else {
			// TODO - maybe we should parse for empty source/destination
			// here. Would be confusing if we print just a bare colon.
			if len(mount.Options) > 0 {
				binds = append(binds, fmt.Sprintf("%s:%s:%s", mount.Source, mount.Destination, strings.Join(mount.Options, ",")))
			} else {
				binds = append(binds, fmt.Sprintf("%s:%s", mount.Source, mount.Destination))
			}
		}
	}
	hostConfig.Binds = binds
	hostConfig.Tmpfs = tmpfs

	// Network mode parsing.
	networkMode := ""
	if c.config.CreateNetNS {
		networkMode = "default"
	} else if c.config.NetNsCtr != "" {
		networkMode = fmt.Sprintf("container:%s", c.config.NetNsCtr)
	} else {
		// Find the spec's network namespace.
		// If there is none, it's host networking.
		// If there is one and it has a path, it's "ns:".
		foundNetNS := false
		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == spec.NetworkNamespace {
				foundNetNS = true
				if ns.Path != "" {
					networkMode = fmt.Sprintf("ns:%s", ns.Path)
				} else {
					networkMode = "none"
				}
				break
			}
		}
		if !foundNetNS {
			networkMode = "host"
		}
	}
	hostConfig.NetworkMode = networkMode

	// Port bindings.
	// Only populate if we're using CNI to configure the network.
	portBindings := make(map[string][]InspectHostPort)
	if c.config.CreateNetNS {
		for _, port := range c.config.PortMappings {
			key := fmt.Sprintf("%d/%s", port.ContainerPort, port.Protocol)
			hostPorts := portBindings[key]
			if hostPorts == nil {
				hostPorts = []InspectHostPort{}
			}
			hostPorts = append(hostPorts, InspectHostPort{
				HostIP:   port.HostIP,
				HostPort: fmt.Sprintf("%d", port.HostPort),
			})
			portBindings[key] = hostPorts
		}
	}
	hostConfig.PortBindings = portBindings

	// Cap add and cap drop.
	// We need a default set of capabilities to compare against.
	// The OCI generate package has one, and is commonly used, so we'll
	// use it.
	// Problem: there are 5 sets of capabilities.
	// Use the bounding set for this computation, it's the most encompassing
	// (but still not perfect).
	capAdd := []string{}
	capDrop := []string{}
	// No point in continuing if we got a spec without a Process block...
	if ctrSpec.Process != nil {
		// Max an O(1) lookup table for default bounding caps.
		boundingCaps := make(map[string]bool)
		g, err := generate.New("linux")
		if err != nil {
			return nil, err
		}
		if !hostConfig.Privileged {
			for _, cap := range g.Config.Process.Capabilities.Bounding {
				boundingCaps[cap] = true
			}
		} else {
			// If we are privileged, use all caps.
			for _, cap := range capability.List() {
				if g.HostSpecific && cap > validate.LastCap() {
					continue
				}
				boundingCaps[fmt.Sprintf("CAP_%s", strings.ToUpper(cap.String()))] = true
			}
		}
		// Iterate through spec caps.
		// If it's not in default bounding caps, it was added.
		// If it is, delete from the default set. Whatever remains after
		// we finish are the dropped caps.
		for _, cap := range ctrSpec.Process.Capabilities.Bounding {
			if _, ok := boundingCaps[cap]; ok {
				delete(boundingCaps, cap)
			} else {
				capAdd = append(capAdd, cap)
			}
		}
		for cap := range boundingCaps {
			capDrop = append(capDrop, cap)
		}
	}
	hostConfig.CapAdd = capAdd
	hostConfig.CapDrop = capDrop

	// IPC Namespace mode
	ipcMode := ""
	if c.config.IPCNsCtr != "" {
		ipcMode = fmt.Sprintf("container:%s", c.config.IPCNsCtr)
	} else {
		// Locate the spec's IPC namespace.
		// If there is none, it's ipc=host.
		// If there is one and it has a path, it's "ns:".
		// If no path, it's default - the empty string.
		foundIPCNS := false
		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == spec.IPCNamespace {
				foundIPCNS = true
				if ns.Path != "" {
					ipcMode = fmt.Sprintf("ns:%s", ns.Path)
				}
				break
			}
		}
		if !foundIPCNS {
			ipcMode = "host"
		}
	}
	hostConfig.IpcMode = ipcMode

	// CGroup parent
	// Need to check if it's the default, and not print if so.
	defaultCgroupParent := ""
	switch c.runtime.config.CgroupManager {
	case define.CgroupfsCgroupsManager:
		defaultCgroupParent = CgroupfsDefaultCgroupParent
	case define.SystemdCgroupsManager:
		defaultCgroupParent = SystemdDefaultCgroupParent
	}
	if c.config.CgroupParent != defaultCgroupParent {
		hostConfig.CgroupParent = c.config.CgroupParent
	}

	// PID namespace mode
	pidMode := ""
	if c.config.PIDNsCtr != "" {
		pidMode = fmt.Sprintf("container:%s", c.config.PIDNsCtr)
	} else {
		// Locate the spec's PID namespace.
		// If there is none, it's pid=host.
		// If there is one and it has a path, it's "ns:".
		// If there is no path, it's default - the empty string.
		foundPIDNS := false
		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == spec.PIDNamespace {
				foundPIDNS = true
				if ns.Path != "" {
					pidMode = fmt.Sprintf("ns:%s", ns.Path)
				}
				break
			}
		}
		if !foundPIDNS {
			pidMode = "host"
		}
	}
	hostConfig.PidMode = pidMode

	// UTS namespace mode
	utsMode := ""
	if c.config.UTSNsCtr != "" {
		utsMode = fmt.Sprintf("container:%s", c.config.UTSNsCtr)
	} else {
		// Locate the spec's UTS namespace.
		// If there is none, it's uts=host.
		// If there is one and it has a path, it's "ns:".
		// If there is no path, it's default - the empty string.
		foundUTSNS := false
		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == spec.UTSNamespace {
				foundUTSNS = true
				if ns.Path != "" {
					utsMode = fmt.Sprintf("ns:%s", ns.Path)
				}
				break
			}
		}
		if !foundUTSNS {
			utsMode = "host"
		}
	}
	hostConfig.UTSMode = utsMode

	// User namespace mode
	usernsMode := ""
	if c.config.UserNsCtr != "" {
		usernsMode = fmt.Sprintf("container:%s", c.config.UserNsCtr)
	} else {
		// Locate the spec's user namespace.
		// If there is none, it's default - the empty string.
		// If there is one, it's "private" if no path, or "ns:" if
		// there's a path.
		for _, ns := range ctrSpec.Linux.Namespaces {
			if ns.Type == spec.UserNamespace {
				if ns.Path != "" {
					usernsMode = fmt.Sprintf("ns:%s", ns.Path)
				} else {
					usernsMode = "private"
				}
			}
		}
	}
	hostConfig.UsernsMode = usernsMode

	// Devices
	// Do not include if privileged - assumed that all devices will be
	// included.
	hostConfig.Devices = []InspectDevice{}
	if ctrSpec.Linux != nil && !hostConfig.Privileged {
		for _, dev := range ctrSpec.Linux.Devices {
			key := fmt.Sprintf("%d:%d", dev.Major, dev.Minor)
			if deviceNodes == nil {
				nodes, err := util.FindDeviceNodes()
				if err != nil {
					return nil, err
				}
				deviceNodes = nodes
			}
			path, ok := deviceNodes[key]
			if !ok {
				logrus.Warnf("Could not locate device %s on host", key)
				continue
			}
			newDev := InspectDevice{}
			newDev.PathOnHost = path
			newDev.PathInContainer = dev.Path
			hostConfig.Devices = append(hostConfig.Devices, newDev)
		}
	}

	// Ulimits
	hostConfig.Ulimits = []InspectUlimit{}
	if ctrSpec.Process != nil {
		for _, limit := range ctrSpec.Process.Rlimits {
			newLimit := InspectUlimit{}
			newLimit.Name = limit.Type
			newLimit.Soft = limit.Soft
			newLimit.Hard = limit.Hard
			hostConfig.Ulimits = append(hostConfig.Ulimits, newLimit)
		}
	}

	// Terminal size
	// We can't actually get this for now...
	// So default to something sane.
	// TODO: Populate this.
	hostConfig.ConsoleSize = []uint{0, 0}

	return hostConfig, nil
}
