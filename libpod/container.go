package libpod

import (
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/containerd/cgroups"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/ulule/deepcopier"
)

// ContainerStatus represents the current state of a container
type ContainerStatus int

const (
	// ContainerStateUnknown indicates that the container is in an error
	// state where information about it cannot be retrieved
	ContainerStateUnknown ContainerStatus = iota
	// ContainerStateConfigured indicates that the container has had its
	// storage configured but it has not been created in the OCI runtime
	ContainerStateConfigured ContainerStatus = iota
	// ContainerStateCreated indicates the container has been created in
	// the OCI runtime but not started
	ContainerStateCreated ContainerStatus = iota
	// ContainerStateRunning indicates the container is currently executing
	ContainerStateRunning ContainerStatus = iota
	// ContainerStateStopped indicates that the container was running but has
	// exited
	ContainerStateStopped ContainerStatus = iota
	// ContainerStatePaused indicates that the container has been paused
	ContainerStatePaused ContainerStatus = iota
)

// DefaultCgroupParent is the default prefix to a cgroup path in libpod
var DefaultCgroupParent = "/libpod_parent"

// LinuxNS represents a Linux namespace
type LinuxNS int

const (
	// InvalidNS is an invalid namespace
	InvalidNS LinuxNS = iota
	// IPCNS is the IPC namespace
	IPCNS LinuxNS = iota
	// MountNS is the mount namespace
	MountNS LinuxNS = iota
	// NetNS is the network namespace
	NetNS LinuxNS = iota
	// PIDNS is the PID namespace
	PIDNS LinuxNS = iota
	// UserNS is the user namespace
	UserNS LinuxNS = iota
	// UTSNS is the UTS namespace
	UTSNS LinuxNS = iota
	// CgroupNS is the CGroup namespace
	CgroupNS LinuxNS = iota
)

// String returns a string representation of a Linux namespace
// It is guaranteed to be the name of the namespace in /proc for valid ns types
func (ns LinuxNS) String() string {
	switch ns {
	case InvalidNS:
		return "invalid"
	case IPCNS:
		return "ipc"
	case MountNS:
		return "mnt"
	case NetNS:
		return "net"
	case PIDNS:
		return "pid"
	case UserNS:
		return "user"
	case UTSNS:
		return "uts"
	case CgroupNS:
		return "cgroup"
	default:
		return "unknown"
	}
}

// Container is a single OCI container
// ffjson: skip
type Container struct {
	config *ContainerConfig

	runningSpec *spec.Spec

	state *containerState

	// Locked indicates that a container has been locked as part of a
	// Batch() operation
	// Functions called on a locked container will not lock or sync
	locked bool

	valid   bool
	lock    storage.Locker
	runtime *Runtime
}

// TODO fetch IP and Subnet Mask from networks once we have updated OCICNI

// containerState contains the current state of the container
// It is stored on disk in a tmpfs and recreated on reboot
type containerState struct {
	// The current state of the running container
	State ContainerStatus `json:"state"`
	// The path to the JSON OCI runtime spec for this container
	ConfigPath string `json:"configPath,omitempty"`
	// RunDir is a per-boot directory for container content
	RunDir string `json:"runDir,omitempty"`
	// Mounted indicates whether the container's storage has been mounted
	// for use
	Mounted bool `json:"mounted,omitempty"`
	// MountPoint contains the path to the container's mounted storage
	Mountpoint string `json:"mountPoint,omitempty"`
	// StartedTime is the time the container was started
	StartedTime time.Time `json:"startedTime,omitempty"`
	// FinishedTime is the time the container finished executing
	FinishedTime time.Time `json:"finishedTime,omitempty"`
	// ExitCode is the exit code returned when the container stopped
	ExitCode int32 `json:"exitCode,omitempty"`
	// OOMKilled indicates that the container was killed as it ran out of
	// memory
	OOMKilled bool `json:"oomKilled,omitempty"`
	// PID is the PID of a running container
	PID int `json:"pid,omitempty"`
	// NetNSPath is the path of the container's network namespace
	// Will only be set if config.CreateNetNS is true, or the container was
	// told to join another container's network namespace
	NetNS ns.NetNS `json:"-"`
	// IP address of container (if network namespace was created)
	IPAddress string `json:"ipAddress"`
	// Subnet mask of container (if network namespace was created)
	SubnetMask string `json:"subnetMask"`
}

// ContainerConfig contains all information that was used to create the
// container. It may not be changed once created.
// It is stored, read-only, on disk
type ContainerConfig struct {
	Spec *spec.Spec `json:"spec"`
	ID   string     `json:"id"`
	Name string     `json:"name"`
	// Full ID of the pood the container belongs to
	Pod string `json:"pod,omitempty"`

	// TODO consider breaking these subsections up into smaller structs

	// Storage Config
	// Information on the image used for the root filesystem
	RootfsImageID   string `json:"rootfsImageID,omitempty"`
	RootfsImageName string `json:"rootfsImageName,omitempty"`
	// Whether to mount volumes specified in the image
	ImageVolumes bool `json:"imageVolumes"`
	// Src path to be mounted on /dev/shm in container
	ShmDir string `json:"ShmDir,omitempty"`
	// Size of the container's SHM
	ShmSize int64 `json:"shmSize"`
	// Static directory for container content that will persist across
	// reboot
	StaticDir string `json:"staticDir"`
	// Mounts list contains all additional mounts into the container rootfs
	// These include the SHM mount
	// These must be unmounted before the container's rootfs is unmounted
	Mounts []string `json:"mounts,omitempty"`

	// Security Config
	// Whether the container is privileged
	Privileged bool `json:"privileged"`
	// Whether to set the No New Privileges flag
	NoNewPrivs bool `json:"noNewPrivs"`
	// SELinux process label for container
	ProcessLabel string `json:"ProcessLabel,omitempty"`
	// SELinux mount label for root filesystem
	MountLabel string `json:"MountLabel,omitempty"`
	// User and group to use in the container
	// Can be specified by name or UID/GID
	User string `json:"user,omitempty"`

	// Namespace Config
	// IDs of container to share namespaces with
	// NetNsCtr conflicts with the CreateNetNS bool
	IPCNsCtr    string `json:"ipcNsCtr,omitempty"`
	MountNsCtr  string `json:"mountNsCtr,omitempty"`
	NetNsCtr    string `json:"netNsCtr,omitempty"`
	PIDNsCtr    string `json:"pidNsCtr,omitempty"`
	UserNsCtr   string `json:"userNsCtr,omitempty"`
	UTSNsCtr    string `json:"utsNsCtr,omitempty"`
	CgroupNsCtr string `json:"cgroupNsCtr,omitempty"`

	// Network Config
	// CreateNetNS indicates that libpod should create and configure a new
	// network namespace for the container
	// This cannot be set if NetNsCtr is also set
	CreateNetNS bool `json:"createNetNS"`
	// PortMappings are the ports forwarded to the container's network
	// namespace
	// These are not used unless CreateNetNS is true
	PortMappings []ocicni.PortMapping `json:"portMappings,omitempty"`
	// DNS servers to use in container resolv.conf
	// Will override servers in host resolv if set
	DNSServer []net.IP `json:"dnsServer,omitempty"`
	// DNS Search domains to use in container resolv.conf
	// Will override search domains in host resolv if set
	DNSSearch []string `json:"dnsSearch,omitempty"`
	// DNS options to be set in container resolv.conf
	// With override options in host resolv if set
	DNSOption []string `json:"dnsOption,omitempty"`
	// Hosts to add in container
	// Will be appended to host's host file
	HostAdd []string `json:"hostsAdd,omitempty"`

	// Misc Options
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
	// Cgroup parent of the container
	CgroupParent string `json:"cgroupParent"`
	// LogPath log location
	LogPath string `json:"logPath"`
	// TODO log options for log drivers
}

// ContainerStatus returns a string representation for users
// of a container state
func (t ContainerStatus) String() string {
	switch t {
	case ContainerStateUnknown:
		return "unknown"
	case ContainerStateConfigured:
		return "configured"
	case ContainerStateCreated:
		return "created"
	case ContainerStateRunning:
		return "running"
	case ContainerStateStopped:
		return "exited"
	case ContainerStatePaused:
		return "paused"
	}
	return "bad state"
}

// Config accessors
// Unlocked

// Config returns the configuration used to create the container
func (c *Container) Config() *ContainerConfig {
	returnConfig := new(ContainerConfig)
	deepcopier.Copy(c.config).To(returnConfig)

	return returnConfig
}

// Spec returns the container's OCI runtime spec
// The spec returned is the one used to create the container. The running
// spec may differ slightly as mounts are added based on the image
func (c *Container) Spec() *spec.Spec {
	returnSpec := new(spec.Spec)
	deepcopier.Copy(c.config.Spec).To(returnSpec)

	return returnSpec
}

// ID returns the container's ID
func (c *Container) ID() string {
	return c.config.ID
}

// Name returns the container's name
func (c *Container) Name() string {
	return c.config.Name
}

// PodID returns the full ID of the pod the container belongs to, or "" if it
// does not belong to a pod
func (c *Container) PodID() string {
	return c.config.Pod
}

// Image returns the ID and name of the image used as the container's rootfs
func (c *Container) Image() (string, string) {
	return c.config.RootfsImageID, c.config.RootfsImageName
}

// ImageVolumes returns whether the container is configured to create
// persistent volumes requested by the image
func (c *Container) ImageVolumes() bool {
	return c.config.ImageVolumes
}

// ShmDir returns the sources path to be mounted on /dev/shm in container
func (c *Container) ShmDir() string {
	return c.config.ShmDir
}

// ShmSize returns the size of SHM device to be mounted into the container
func (c *Container) ShmSize() int64 {
	return c.config.ShmSize
}

// StaticDir returns the directory used to store persistent container files
func (c *Container) StaticDir() string {
	return c.config.StaticDir
}

// Privileged returns whether the container is privileged
func (c *Container) Privileged() bool {
	return c.config.Privileged
}

// ProcessLabel returns the selinux ProcessLabel of the container
func (c *Container) ProcessLabel() string {
	return c.config.ProcessLabel
}

// MountLabel returns the SELinux mount label of the container
func (c *Container) MountLabel() string {
	return c.config.MountLabel
}

// User returns the user who the container is run as
func (c *Container) User() string {
	return c.config.User
}

// Dependencies gets the containers this container depends upon
func (c *Container) Dependencies() []string {
	// Collect in a map first to remove dupes
	dependsCtrs := map[string]bool{}
	if c.config.IPCNsCtr != "" {
		dependsCtrs[c.config.IPCNsCtr] = true
	}
	if c.config.MountNsCtr != "" {
		dependsCtrs[c.config.MountNsCtr] = true
	}
	if c.config.NetNsCtr != "" {
		dependsCtrs[c.config.NetNsCtr] = true
	}
	if c.config.PIDNsCtr != "" {
		dependsCtrs[c.config.NetNsCtr] = true
	}
	if c.config.UserNsCtr != "" {
		dependsCtrs[c.config.UserNsCtr] = true
	}
	if c.config.UTSNsCtr != "" {
		dependsCtrs[c.config.UTSNsCtr] = true
	}
	if c.config.CgroupNsCtr != "" {
		dependsCtrs[c.config.CgroupNsCtr] = true
	}

	if len(dependsCtrs) == 0 {
		return []string{}
	}

	depends := make([]string, 0, len(dependsCtrs))
	for ctr := range dependsCtrs {
		depends = append(depends, ctr)
	}

	return depends
}

// NewNetNS returns whether the container will create a new network namespace
func (c *Container) NewNetNS() bool {
	return c.config.CreateNetNS
}

// PortMappings returns the ports that will be mapped into a container if
// a new network namespace is created
// If NewNetNS() is false, this value is unused
func (c *Container) PortMappings() []ocicni.PortMapping {
	return c.config.PortMappings
}

// DNSServers returns DNS servers that will be used in the container's
// resolv.conf
// If empty, DNS server from the host's resolv.conf will be used instead
func (c *Container) DNSServers() []net.IP {
	return c.config.DNSServer
}

// DNSSearch returns the DNS search domains that will be used in the container's
// resolv.conf
// If empty, DNS Search domains from the host's resolv.conf will be used instead
func (c *Container) DNSSearch() []string {
	return c.config.DNSSearch
}

// DNSOption returns the DNS options that will be used in the container's
// resolv.conf
// If empty, options from the host's resolv.conf will be used instead
func (c *Container) DNSOption() []string {
	return c.config.DNSOption
}

// HostsAdd returns hosts that will be added to the container's hosts file
// The host system's hosts file is used as a base, and these are appended to it
func (c *Container) HostsAdd() []string {
	return c.config.HostAdd
}

// Stdin returns whether STDIN on the container will be kept open
func (c *Container) Stdin() bool {
	return c.config.Stdin
}

// Labels returns the container's labels
func (c *Container) Labels() map[string]string {
	labels := make(map[string]string)
	for key, value := range c.config.Labels {
		labels[key] = value
	}
	return labels
}

// StopSignal is the signal that will be used to stop the container
// If it fails to stop the container, SIGKILL will be used after a timeout
// If StopSignal is 0, the default signal of SIGTERM will be used
func (c *Container) StopSignal() uint {
	return c.config.StopSignal
}

// StopTimeout returns the container's stop timeout
// If the container's default stop signal fails to kill the container, SIGKILL
// will be used after this timeout
func (c *Container) StopTimeout() uint {
	return c.config.StopTimeout
}

// CreatedTime gets the time when the container was created
func (c *Container) CreatedTime() time.Time {
	return c.config.CreatedTime
}

// CgroupParent gets the container's CGroup parent
func (c *Container) CgroupParent() string {
	return c.config.CgroupParent
}

// LogPath returns the path to the container's log file
// This file will only be present after Init() is called to create the container
// in runc
func (c *Container) LogPath() string {
	return c.config.LogPath
}

// RuntimeName returns the name of the runtime
func (c *Container) RuntimeName() string {
	return c.runtime.ociRuntime.name
}

// State Accessors
// Require locking

// State returns the current state of the container
func (c *Container) State() (ContainerStatus, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return ContainerStateUnknown, err
		}
	}
	return c.state.State, nil
}

// Mounted returns a bool as to if the container's storage
// is mounted
func (c *Container) Mounted() (bool, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return false, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.state.Mounted, nil
}

// Mountpoint returns the path to the container's mounted storage as a string
// If the container is not mounted, no error is returned, but the mountpoint
// will be ""
func (c *Container) Mountpoint() (string, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return "", errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.state.Mountpoint, nil
}

// StartedTime is the time the container was started
func (c *Container) StartedTime() (time.Time, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return time.Time{}, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.state.StartedTime, nil
}

// FinishedTime is the time the container was stopped
func (c *Container) FinishedTime() (time.Time, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return time.Time{}, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.state.FinishedTime, nil
}

// ExitCode returns the exit code of the container as
// an int32
func (c *Container) ExitCode() (int32, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return 0, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.state.ExitCode, nil
}

// OOMKilled returns whether the container was killed by an OOM condition
func (c *Container) OOMKilled() (bool, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return false, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.state.OOMKilled, nil
}

// PID returns the PID of the container
// An error is returned if the container is not running
func (c *Container) PID() (int, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return -1, err
		}
	}

	return c.state.PID, nil
}

// Misc Accessors
// Most will require locking

// IPAddress returns the IP address of the container
// If the container does not have a network namespace, an error will be returned
func (c *Container) IPAddress() (net.IP, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}

	if !c.config.CreateNetNS || c.state.NetNS == nil {
		return nil, errors.Wrapf(ErrInvalidArg, "container %s does not have a network namespace", c.ID())
	}

	return c.runtime.getContainerIP(c)
}

// NamespacePath returns the path of one of the container's namespaces
// If the container is not running, an error will be returned
func (c *Container) NamespacePath(ns LinuxNS) (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return "", errors.Wrapf(err, "error updating container %s state", c.ID())
	}

	if c.state.State != ContainerStateRunning && c.state.State != ContainerStatePaused {
		return "", errors.Wrapf(ErrCtrStopped, "cannot get namespace path unless container %s is running", c.ID())
	}

	if ns == InvalidNS {
		return "", errors.Wrapf(ErrInvalidArg, "invalid namespace requested from container %s", c.ID())
	}

	return fmt.Sprintf("/proc/%d/ns/%s", c.state.PID, ns.String()), nil
}

// CGroupPath returns a cgroups "path" for a given container.
func (c *Container) CGroupPath() cgroups.Path {
	return cgroups.StaticPath(filepath.Join(c.config.CgroupParent, fmt.Sprintf("libpod-conmon-%s", c.ID())))
}

// RootFsSize returns the root FS size of the container
func (c *Container) RootFsSize() (int64, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return -1, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.rootFsSize()
}

// RWSize returns the rw size of the container
func (c *Container) RWSize() (int64, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return -1, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.rwSize()
}

// Hostname gets the container's hostname
func (c *Container) Hostname() string {
	if c.config.Spec.Hostname != "" {
		return c.config.Spec.Hostname
	}

	if len(c.ID()) < 11 {
		return c.ID()
	}
	return c.ID()[:12]
}
