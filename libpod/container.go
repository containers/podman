package libpod

import (
	"fmt"
	"net"
	"path/filepath"
	"time"

	"github.com/containernetworking/cni/pkg/types"
	cnitypes "github.com/containernetworking/cni/pkg/types/current"
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

// CgroupfsDefaultCgroupParent is the cgroup parent for CGroupFS in libpod
const CgroupfsDefaultCgroupParent = "/libpod_parent"

// SystemdDefaultCgroupParent is the cgroup parent for the systemd cgroup
// manager in libpod
const SystemdDefaultCgroupParent = "system.slice"

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

	state *containerState

	// Batched indicates that a container has been locked as part of a
	// Batch() operation
	// Functions called on a batched container will not lock or sync
	batched bool

	valid   bool
	lock    storage.Locker
	runtime *Runtime
}

// containerState contains the current state of the container
// It is stored on disk in a tmpfs and recreated on reboot
type containerState struct {
	// The current state of the running container
	State ContainerStatus `json:"state"`
	// The path to the JSON OCI runtime spec for this container
	ConfigPath string `json:"configPath,omitempty"`
	// RunDir is a per-boot directory for container content
	RunDir string `json:"runDir,omitempty"`
	// DestinationRunDir is where the files in RunDir will be accessible for the container.
	// It is different than RunDir when using userNS
	DestinationRunDir string `json:"destinationRunDir,omitempty"`
	// Mounted indicates whether the container's storage has been mounted
	// for use
	Mounted bool `json:"mounted,omitempty"`
	// Mountpoint contains the path to the container's mounted storage as given
	// by containers/storage.  It can be different than RealMountpoint when
	// usernamespaces are used
	Mountpoint string `json:"mountPoint,omitempty"`
	// RealMountpoint contains the path to the container's mounted storage
	RealMountpoint string `json:"realMountPoint,omitempty"`
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
	// ExecSessions contains active exec sessions for container
	// Exec session ID is mapped to PID of exec process
	ExecSessions map[string]*ExecSession `json:"execSessions,omitempty"`
	// IPs contains IP addresses assigned to the container
	// Only populated if we created a network namespace for the container,
	// and the network namespace is currently active
	IPs []*cnitypes.IPConfig `json:"ipAddresses,omitempty"`
	// Routes contains network routes present in the container
	// Only populated if we created a network namespace for the container,
	// and the network namespace is currently active
	Routes []*types.Route `json:"routes,omitempty"`
	// BindMounts contains files that will be bind-mounted into the
	// container when it is mounted.
	// These include /etc/hosts and /etc/resolv.conf
	// This maps the path the file will be mounted to in the container to
	// the path of the file on disk outside the container
	BindMounts map[string]string `json:"bindMounts,omitempty"`

	// UserNSRoot is the directory used as root for the container when using
	// user namespaces.
	UserNSRoot string `json:"userNSRoot,omitempty"`
}

// ExecSession contains information on an active exec session
type ExecSession struct {
	ID      string   `json:"id"`
	Command []string `json:"command"`
	PID     int      `json:"pid"`
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

	// UID/GID mappings used by the storage
	IDMappings storage.IDMappingOptions `json:"idMappingsOptions,omitempty"`

	// Information on the image used for the root filesystem/
	RootfsImageID   string `json:"rootfsImageID,omitempty"`
	RootfsImageName string `json:"rootfsImageName,omitempty"`
	// Whether to mount volumes specified in the image.
	ImageVolumes bool `json:"imageVolumes"`
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

	// Security Config

	// Whether the container is privileged
	Privileged bool `json:"privileged"`
	// SELinux process label for container
	ProcessLabel string `json:"ProcessLabel,omitempty"`
	// SELinux mount label for root filesystem
	MountLabel string `json:"MountLabel,omitempty"`
	// User and group to use in the container
	// Can be specified by name or UID/GID
	User string `json:"user,omitempty"`
	// Additional groups to add
	Groups []string `json:"groups,omitempty"`

	// Namespace Config
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

	// IDs of dependency containers.
	// These containers must be started before this container is started.
	Dependencies []string

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

	// Image Config

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
	// File containing the conmon PID
	ConmonPidFile string `json:"conmonPidFile,omitempty"`
	// TODO log options for log drivers

	PostConfigureNetNS bool `json:"postConfigureNetNS"`
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

	// First add all namespace containers
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

	// Add all generic dependencies
	for _, id := range c.config.Dependencies {
		dependsCtrs[id] = true
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

// UserVolumes returns user-added volume mounts in the container.
// These are not added to the spec, but are used during image commit and to
// trigger some OCI hooks.
func (c *Container) UserVolumes() []string {
	volumes := make([]string, 0, len(c.config.UserVolumes))
	for _, vol := range c.config.UserVolumes {
		volumes = append(volumes, vol)
	}

	return volumes
}

// Entrypoint is the container's entrypoint.
// This is not added to the spec, but is instead used during image commit.
func (c *Container) Entrypoint() []string {
	entrypoint := make([]string, 0, len(c.config.Entrypoint))
	for _, str := range c.config.Entrypoint {
		entrypoint = append(entrypoint, str)
	}

	return entrypoint
}

// Command is the container's command
// This is not added to the spec, but is instead used during image commit
func (c *Container) Command() []string {
	command := make([]string, 0, len(c.config.Command))
	for _, str := range c.config.Command {
		command = append(command, str)
	}

	return command
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
// in the runtime
func (c *Container) LogPath() string {
	return c.config.LogPath
}

// RuntimeName returns the name of the runtime
func (c *Container) RuntimeName() string {
	return c.runtime.ociRuntime.name
}

// Runtime spec accessors
// Unlocked

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

// State Accessors
// Require locking

// State returns the current state of the container
func (c *Container) State() (ContainerStatus, error) {
	if !c.batched {
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
	if !c.batched {
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
	if !c.batched {
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
	if !c.batched {
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
	if !c.batched {
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
	if !c.batched {
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
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return false, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.state.OOMKilled, nil
}

// PID returns the PID of the container
// If the container is not running, a pid of 0 will be returned. No error will
// occur.
func (c *Container) PID() (int, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return -1, err
		}
	}

	return c.state.PID, nil
}

// ExecSessions retrieves active exec sessions running in the container
func (c *Container) ExecSessions() ([]string, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	ids := make([]string, 0, len(c.state.ExecSessions))
	for id := range c.state.ExecSessions {
		ids = append(ids, id)
	}

	return ids, nil
}

// ExecSession retrieves detailed information on a single active exec session in
// a container
func (c *Container) ExecSession(id string) (*ExecSession, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	session, ok := c.state.ExecSessions[id]
	if !ok {
		return nil, errors.Wrapf(ErrNoSuchCtr, "no exec session with ID %s found in container %s", id, c.ID())
	}

	returnSession := new(ExecSession)
	returnSession.ID = session.ID
	returnSession.Command = session.Command
	returnSession.PID = session.PID

	return returnSession, nil
}

// IPs retrieves a container's IP address(es)
// This will only be populated if the container is configured to created a new
// network namespace, and that namespace is presently active
func (c *Container) IPs() ([]net.IPNet, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	if !c.config.CreateNetNS {
		return nil, errors.Wrapf(ErrInvalidArg, "container %s network namespace is not managed by libpod")
	}

	ips := make([]net.IPNet, 0, len(c.state.IPs))

	for _, ip := range c.state.IPs {
		ips = append(ips, ip.Address)
	}

	return ips, nil
}

// Routes retrieves a container's routes
// This will only be populated if the container is configured to created a new
// network namespace, and that namespace is presently active
func (c *Container) Routes() ([]types.Route, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	if !c.config.CreateNetNS {
		return nil, errors.Wrapf(ErrInvalidArg, "container %s network namespace is not managed by libpod")
	}

	routes := make([]types.Route, 0, len(c.state.Routes))

	for _, route := range c.state.Routes {
		newRoute := types.Route{
			Dst: route.Dst,
			GW:  route.GW,
		}

		routes = append(routes, newRoute)
	}

	return routes, nil
}

// BindMounts retrieves bind mounts that were created by libpod and will be
// added to the container
// All these mounts except /dev/shm are ignored if a mount in the given spec has
// the same destination
// These mounts include /etc/resolv.conf, /etc/hosts, and /etc/hostname
// The return is formatted as a map from destination (mountpoint in the
// container) to source (path of the file that will be mounted into the
// container)
// If the container has not been started yet, an empty map will be returned, as
// the files in question are only created when the container is started.
func (c *Container) BindMounts() (map[string]string, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	newMap := make(map[string]string, len(c.state.BindMounts))

	for key, val := range c.state.BindMounts {
		newMap[key] = val
	}

	return newMap, nil
}

// Misc Accessors
// Most will require locking

// NamespacePath returns the path of one of the container's namespaces
// If the container is not running, an error will be returned
func (c *Container) NamespacePath(ns LinuxNS) (string, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return "", errors.Wrapf(err, "error updating container %s state", c.ID())
		}
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
func (c *Container) CGroupPath() (string, error) {
	switch c.runtime.config.CgroupManager {
	case CgroupfsCgroupsManager:
		return filepath.Join(c.config.CgroupParent, fmt.Sprintf("libpod-%s", c.ID()), "ctr"), nil
	case SystemdCgroupsManager:
		return filepath.Join(c.config.CgroupParent, createUnitName("libpod", c.ID())), nil
	default:
		return "", errors.Wrapf(ErrInvalidArg, "unsupported CGroup manager %s in use", c.runtime.config.CgroupManager)
	}
}

// RootFsSize returns the root FS size of the container
func (c *Container) RootFsSize() (int64, error) {
	if !c.batched {
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
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return -1, errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.rwSize()
}

// IDMappings returns the UID/GID mapping used for the container
func (c *Container) IDMappings() (storage.IDMappingOptions, error) {
	return c.config.IDMappings, nil
}

// RootUID returns the root user mapping from container
func (c *Container) RootUID() int {
	for _, uidmap := range c.config.IDMappings.UIDMap {
		if uidmap.ContainerID == 0 {
			return uidmap.HostID
		}
	}
	return 0
}

// RootGID returns the root user mapping from container
func (c *Container) RootGID() int {
	for _, gidmap := range c.config.IDMappings.GIDMap {
		if gidmap.ContainerID == 0 {
			return gidmap.HostID
		}
	}
	return 0
}
