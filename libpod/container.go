package libpod

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/cgroups"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/docker/docker/daemon/caps"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/term"
	"github.com/mrunalp/fileutils"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod/driver"
	crioAnnotations "github.com/projectatomic/libpod/pkg/annotations"
	"github.com/projectatomic/libpod/pkg/chrootuser"
	"github.com/sirupsen/logrus"
	"github.com/ulule/deepcopier"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/remotecommand"
)

// ContainerState represents the current state of a container
type ContainerState int

const (
	// ContainerStateUnknown indicates that the container is in an error
	// state where information about it cannot be retrieved
	ContainerStateUnknown ContainerState = iota
	// ContainerStateConfigured indicates that the container has had its
	// storage configured but it has not been created in the OCI runtime
	ContainerStateConfigured ContainerState = iota
	// ContainerStateCreated indicates the container has been created in
	// the OCI runtime but not started
	ContainerStateCreated ContainerState = iota
	// ContainerStateRunning indicates the container is currently executing
	ContainerStateRunning ContainerState = iota
	// ContainerStateStopped indicates that the container was running but has
	// exited
	ContainerStateStopped ContainerState = iota
	// ContainerStatePaused indicates that the container has been paused
	ContainerStatePaused ContainerState = iota
	// name of the directory holding the artifacts
	artifactsDir = "artifacts"
)

// CgroupParent is the default prefix to a cgroup path in libpod
var CgroupParent = "/libpod_parent"

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
type Container struct {
	config *ContainerConfig

	runningSpec *spec.Spec

	state *containerRuntimeInfo

	// Locked indicates that a container has been locked as part of a
	// Batch() operation
	// Functions called on a locked container will not lock or sync
	locked bool

	valid   bool
	lock    storage.Locker
	runtime *Runtime
}

// TODO fetch IP and Subnet Mask from networks once we have updated OCICNI
// TODO enable pod support
// TODO Add readonly support
// TODO add SHM size support

// containerRuntimeInfo contains the current state of the container
// It is stored on disk in a tmpfs and recreated on reboot
type containerRuntimeInfo struct {
	// The current state of the running container
	State ContainerState `json:"state"`
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
	NetNS ns.NetNS
	// IP address of container (if network namespace was created)
	IPAddress string
	// Subnet mask of container (if network namespace was created)
	SubnetMask string
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
	// Whether to make the container read only
	ReadOnly bool `json:"readOnly"`
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

	// TODO log options - logpath for plaintext, others for log drivers
}

// ContainerStater returns a string representation for users
// of a container state
func (t ContainerState) String() string {
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

// ShmDir returns the sources path to be mounted on /dev/shm in container
func (c *Container) ShmDir() string {
	return c.config.ShmDir
}

// ProcessLabel returns the selinux ProcessLabel of the container
func (c *Container) ProcessLabel() string {
	return c.config.ProcessLabel
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

	depends := make([]string, len(dependsCtrs), 0)
	for ctr, _ := range dependsCtrs {
		depends = append(depends, ctr)
	}

	return depends
}

// Spec returns the container's OCI runtime spec
// The spec returned is the one used to create the container. The running
// spec may differ slightly as mounts are added based on the image
func (c *Container) Spec() *spec.Spec {
	returnSpec := new(spec.Spec)
	deepcopier.Copy(c.config.Spec).To(returnSpec)

	return returnSpec
}

// Labels returns the container's labels
func (c *Container) Labels() map[string]string {
	labels := make(map[string]string)
	for key, value := range c.config.Labels {
		labels[key] = value
	}
	return labels
}

// Config returns the configuration used to create the container
func (c *Container) Config() *ContainerConfig {
	returnConfig := new(ContainerConfig)
	deepcopier.Copy(c.config).To(returnConfig)

	return returnConfig
}

// RuntimeName returns the name of the runtime
func (c *Container) RuntimeName() string {
	return c.runtime.ociRuntime.name
}

// rootFsSize gets the size of the container's root filesystem
// A container FS is split into two parts.  The first is the top layer, a
// mutable layer, and the rest is the RootFS: the set of immutable layers
// that make up the image on which the container is based.
func (c *Container) rootFsSize() (int64, error) {
	container, err := c.runtime.store.Container(c.ID())
	if err != nil {
		return 0, err
	}

	// Ignore the size of the top layer.   The top layer is a mutable RW layer
	// and is not considered a part of the rootfs
	rwLayer, err := c.runtime.store.Layer(container.LayerID)
	if err != nil {
		return 0, err
	}
	layer, err := c.runtime.store.Layer(rwLayer.Parent)
	if err != nil {
		return 0, err
	}

	size := int64(0)
	for layer.Parent != "" {
		layerSize, err := c.runtime.store.DiffSize(layer.Parent, layer.ID)
		if err != nil {
			return size, errors.Wrapf(err, "getting diffsize of layer %q and its parent %q", layer.ID, layer.Parent)
		}
		size += layerSize
		layer, err = c.runtime.store.Layer(layer.Parent)
		if err != nil {
			return 0, err
		}
	}
	// Get the size of the last layer.  Has to be outside of the loop
	// because the parent of the last layer is "", andlstore.Get("")
	// will return an error.
	layerSize, err := c.runtime.store.DiffSize(layer.Parent, layer.ID)
	return size + layerSize, err
}

// rwSize Gets the size of the mutable top layer of the container.
func (c *Container) rwSize() (int64, error) {
	container, err := c.runtime.store.Container(c.ID())
	if err != nil {
		return 0, err
	}

	// Get the size of the top layer by calculating the size of the diff
	// between the layer and its parent.  The top layer of a container is
	// the only RW layer, all others are immutable
	layer, err := c.runtime.store.Layer(container.LayerID)
	if err != nil {
		return 0, err
	}
	return c.runtime.store.DiffSize(layer.Parent, layer.ID)
}

// LogPath returns the path to the container's log file
// This file will only be present after Init() is called to create the container
// in runc
func (c *Container) LogPath() string {
	// TODO store this in state and allow overriding
	return c.logPath()
}

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

// Mountpoint returns the path to the container's mounted
// storage as a string
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

// State returns the current state of the container
func (c *Container) State() (ContainerState, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return ContainerStateUnknown, err
		}
	}
	return c.state.State, nil
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

// MountPoint returns the mount point of the continer
func (c *Container) MountPoint() (string, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return "", errors.Wrapf(err, "error updating container %s state", c.ID())
		}
	}
	return c.state.Mountpoint, nil
}

// NamespacePath returns the path of one of the container's namespaces
// If the container is not running, an error will be returned
func (c *Container) NamespacePath(ns LinuxNS) (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return "", errors.Wrapf(err, "error updating container %s state", c.ID())
	}

	if c.state.State != ContainerStateRunning {
		return "", errors.Wrapf(ErrCtrStopped, "cannot get namespace path unless container %s is running", c.ID())
	}

	if ns == InvalidNS {
		return "", errors.Wrapf(ErrInvalidArg, "invalid namespace requested from container %s", c.ID())
	}

	return fmt.Sprintf("/proc/%d/ns/%s", c.state.PID, ns.String()), nil
}

// The path to the container's root filesystem - where the OCI spec will be
// placed, amongst other things
func (c *Container) bundlePath() string {
	return c.config.StaticDir
}

// The path to the container's logs file
func (c *Container) logPath() string {
	return filepath.Join(c.config.StaticDir, "ctr.log")
}

// Retrieves the path of the container's attach socket
func (c *Container) attachSocketPath() string {
	return filepath.Join(c.runtime.ociRuntime.socketsDir, c.ID(), "attach")
}

// Sync this container with on-disk state and runc status
// Should only be called with container lock held
// This function should suffice to ensure a container's state is accurate and
// it is valid for use.
func (c *Container) syncContainer() error {
	if err := c.runtime.state.UpdateContainer(c); err != nil {
		return err
	}
	// If runc knows about the container, update its status in runc
	// And then save back to disk
	if (c.state.State != ContainerStateUnknown) &&
		(c.state.State != ContainerStateConfigured) {
		oldState := c.state.State
		// TODO: optionally replace this with a stat for the exit file
		if err := c.runtime.ociRuntime.updateContainerStatus(c); err != nil {
			return err
		}
		// Only save back to DB if state changed
		if c.state.State != oldState {
			if err := c.save(); err != nil {
				return err
			}
		}
	}

	if !c.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid", c.ID())
	}

	return nil
}

// Make a new container
func newContainer(rspec *spec.Spec, lockDir string) (*Container, error) {
	if rspec == nil {
		return nil, errors.Wrapf(ErrInvalidArg, "must provide a valid runtime spec to create container")
	}

	ctr := new(Container)
	ctr.config = new(ContainerConfig)
	ctr.state = new(containerRuntimeInfo)

	ctr.config.ID = stringid.GenerateNonCryptoID()
	ctr.config.Name = namesgenerator.GetRandomName(0)

	ctr.config.Spec = new(spec.Spec)
	deepcopier.Copy(rspec).To(ctr.config.Spec)
	ctr.config.CreatedTime = time.Now()

	ctr.config.ShmSize = DefaultShmSize
	ctr.config.CgroupParent = CgroupParent

	// Path our lock file will reside at
	lockPath := filepath.Join(lockDir, ctr.config.ID)
	// Grab a lockfile at the given path
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating lockfile for new container")
	}
	ctr.lock = lock

	return ctr, nil
}

// Create container root filesystem for use
func (c *Container) setupStorage() error {
	if !c.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid", c.ID())
	}

	if c.state.State != ContainerStateConfigured {
		return errors.Wrapf(ErrCtrStateInvalid, "container %s must be in Configured state to have storage set up", c.ID())
	}

	// Need both an image ID and image name, plus a bool telling us whether to use the image configuration
	if c.config.RootfsImageID == "" || c.config.RootfsImageName == "" {
		return errors.Wrapf(ErrInvalidArg, "must provide image ID and image name to use an image")
	}

	containerInfo, err := c.runtime.storageService.CreateContainerStorage(c.runtime.imageContext, c.config.RootfsImageName, c.config.RootfsImageID, c.config.Name, c.config.ID, c.config.MountLabel)
	if err != nil {
		return errors.Wrapf(err, "error creating container storage")
	}

	c.config.StaticDir = containerInfo.Dir
	c.state.RunDir = containerInfo.RunDir

	artifacts := filepath.Join(c.config.StaticDir, artifactsDir)
	if err := os.MkdirAll(artifacts, 0755); err != nil {
		return errors.Wrapf(err, "error creating artifacts directory %q", artifacts)
	}

	return nil
}

// Tear down a container's storage prior to removal
func (c *Container) teardownStorage() error {
	if !c.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid", c.ID())
	}

	if c.state.State == ContainerStateRunning || c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "cannot remove storage for container %s as it is running or paused", c.ID())
	}

	artifacts := filepath.Join(c.config.StaticDir, artifactsDir)
	if err := os.RemoveAll(artifacts); err != nil {
		return errors.Wrapf(err, "error removing artifacts %q", artifacts)
	}

	if err := c.cleanupStorage(); err != nil {
		return errors.Wrapf(err, "failed to cleanup container %s storage", c.ID())
	}

	if err := c.runtime.storageService.DeleteContainer(c.ID()); err != nil {
		return errors.Wrapf(err, "error removing container %s root filesystem", c.ID())
	}

	return nil
}

// Refresh refreshes the container's state after a restart
func (c *Container) refresh() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid - may have been removed", c.ID())
	}

	// We need to get the container's temporary directory from c/storage
	// It was lost in the reboot and must be recreated
	dir, err := c.runtime.storageService.GetRunDir(c.ID())
	if err != nil {
		return errors.Wrapf(err, "error retrieving temporary directory for container %s", c.ID())
	}
	c.state.RunDir = dir

	if err := c.runtime.state.SaveContainer(c); err != nil {
		return errors.Wrapf(err, "error refreshing state for container %s", c.ID())
	}

	return nil
}

// Init creates a container in the OCI runtime
func (c *Container) Init() (err error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if c.state.State != ContainerStateConfigured {
		return errors.Wrapf(ErrCtrExists, "container %s has already been created in runtime", c.ID())
	}

	if err := c.mountStorage(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if err2 := c.cleanupStorage(); err2 != nil {
				logrus.Errorf("Error cleaning up storage for container %s: %v", c.ID(), err2)
			}
		}
	}()

	// Make a network namespace for the container
	if c.config.CreateNetNS && c.state.NetNS == nil {
		if err := c.runtime.createNetNS(c); err != nil {
			return err
		}
	}
	defer func() {
		if err != nil {
			if err2 := c.runtime.teardownNetNS(c); err2 != nil {
				logrus.Errorf("Error tearing down network namespace for container %s: %v", c.ID(), err2)
			}
		}
	}()

	// If the OCI spec already exists, we need to replace it
	// Cannot guarantee some things, e.g. network namespaces, have the same
	// paths
	jsonPath := filepath.Join(c.bundlePath(), "config.json")
	if _, err := os.Stat(jsonPath); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "error doing stat on container %s spec", c.ID())
		}
		// The spec does not exist, we're fine
	} else {
		// The spec exists, need to remove it
		if err := os.Remove(jsonPath); err != nil {
			return errors.Wrapf(err, "error replacing runtime spec for container %s", c.ID())
		}
	}

	// Copy /etc/resolv.conf to the container's rundir
	resolvPath := "/etc/resolv.conf"

	// Check if the host system is using system resolve and if so
	// copy its resolv.conf
	_, err = os.Stat("/run/systemd/resolve/resolv.conf")
	if err == nil {
		resolvPath = "/run/systemd/resolve/resolv.conf"
	}
	runDirResolv, err := c.copyHostFileToRundir(resolvPath)
	if err != nil {
		return errors.Wrapf(err, "unable to copy resolv.conf to ", runDirResolv)
	}
	// Copy /etc/hosts to the container's rundir
	runDirHosts, err := c.copyHostFileToRundir("/etc/hosts")
	if err != nil {
		return errors.Wrapf(err, "unable to copy /etc/hosts to ", runDirHosts)
	}

	// Save OCI spec to disk
	g := generate.NewFromSpec(c.config.Spec)
	// If network namespace was requested, add it now
	if c.config.CreateNetNS {
		g.AddOrReplaceLinuxNamespace(spec.NetworkNamespace, c.state.NetNS.Path())
	}
	// Remove default /etc/shm mount
	g.RemoveMount("/dev/shm")
	// Mount ShmDir from host into container
	shmMnt := spec.Mount{
		Type:        "bind",
		Source:      c.config.ShmDir,
		Destination: "/dev/shm",
		Options:     []string{"rw", "bind"},
	}
	g.AddMount(shmMnt)
	// Bind mount resolv.conf
	resolvMnt := spec.Mount{
		Type:        "bind",
		Source:      runDirResolv,
		Destination: "/etc/resolv.conf",
		Options:     []string{"rw", "bind"},
	}
	g.AddMount(resolvMnt)
	// Bind mount hosts
	hostsMnt := spec.Mount{
		Type:        "bind",
		Source:      runDirHosts,
		Destination: "/etc/hosts",
		Options:     []string{"rw", "bind"},
	}
	g.AddMount(hostsMnt)

	if c.config.User != "" {
		if !c.state.Mounted {
			return errors.Wrapf(ErrCtrStateInvalid, "container %s must be mounted in order to translate User field", c.ID())
		}
		uid, gid, err := chrootuser.GetUser(c.state.Mountpoint, c.config.User)
		if err != nil {
			return err
		}
		// User and Group must go together
		g.SetProcessUID(uid)
		g.SetProcessGID(gid)
	}

	// Add shared namespaces from other containers
	if c.config.IPCNsCtr != "" {
		ipcCtr, err := c.runtime.state.Container(c.config.IPCNsCtr)
		if err != nil {
			return err
		}

		nsPath, err := ipcCtr.NamespacePath(IPCNS)
		if err != nil {
			return err
		}

		if err := g.AddOrReplaceLinuxNamespace(spec.IPCNamespace, nsPath); err != nil {
			return err
		}
	}
	if c.config.MountNsCtr != "" {
		mountCtr, err := c.runtime.state.Container(c.config.MountNsCtr)
		if err != nil {
			return err
		}

		nsPath, err := mountCtr.NamespacePath(MountNS)
		if err != nil {
			return err
		}

		if err := g.AddOrReplaceLinuxNamespace(spec.MountNamespace, nsPath); err != nil {
			return err
		}
	}
	if c.config.NetNsCtr != "" {
		netCtr, err := c.runtime.state.Container(c.config.NetNsCtr)
		if err != nil {
			return err
		}

		nsPath, err := netCtr.NamespacePath(NetNS)
		if err != nil {
			return err
		}

		if err := g.AddOrReplaceLinuxNamespace(spec.NetworkNamespace, nsPath); err != nil {
			return err
		}
	}
	if c.config.PIDNsCtr != "" {
		pidCtr, err := c.runtime.state.Container(c.config.PIDNsCtr)
		if err != nil {
			return err
		}

		nsPath, err := pidCtr.NamespacePath(PIDNS)
		if err != nil {
			return err
		}

		if err := g.AddOrReplaceLinuxNamespace(string(spec.PIDNamespace), nsPath); err != nil {
			return err
		}
	}
	if c.config.UserNsCtr != "" {
		userCtr, err := c.runtime.state.Container(c.config.UserNsCtr)
		if err != nil {
			return err
		}

		nsPath, err := userCtr.NamespacePath(UserNS)
		if err != nil {
			return err
		}

		if err := g.AddOrReplaceLinuxNamespace(spec.UserNamespace, nsPath); err != nil {
			return err
		}
	}
	if c.config.UTSNsCtr != "" {
		utsCtr, err := c.runtime.state.Container(c.config.UTSNsCtr)
		if err != nil {
			return err
		}

		nsPath, err := utsCtr.NamespacePath(UTSNS)
		if err != nil {
			return err
		}

		if err := g.AddOrReplaceLinuxNamespace(spec.UTSNamespace, nsPath); err != nil {
			return err
		}
	}
	if c.config.CgroupNsCtr != "" {
		cgroupCtr, err := c.runtime.state.Container(c.config.CgroupNsCtr)
		if err != nil {
			return err
		}

		nsPath, err := cgroupCtr.NamespacePath(CgroupNS)
		if err != nil {
			return err
		}

		if err := g.AddOrReplaceLinuxNamespace(spec.CgroupNamespace, nsPath); err != nil {
			return err
		}
	}

	c.runningSpec = g.Spec()
	c.runningSpec.Root.Path = c.state.Mountpoint
	c.runningSpec.Annotations[crioAnnotations.Created] = c.config.CreatedTime.Format(time.RFC3339Nano)
	c.runningSpec.Annotations["org.opencontainers.image.stopSignal"] = fmt.Sprintf("%d", c.config.StopSignal)

	fileJSON, err := json.Marshal(c.runningSpec)
	if err != nil {
		return errors.Wrapf(err, "error exporting runtime spec for container %s to JSON", c.ID())
	}
	if err := ioutil.WriteFile(jsonPath, fileJSON, 0644); err != nil {
		return errors.Wrapf(err, "error writing runtime spec JSON to file for container %s", c.ID())
	}

	logrus.Debugf("Created OCI spec for container %s at %s", c.ID(), jsonPath)

	c.state.ConfigPath = jsonPath

	// With the spec complete, do an OCI create
	// TODO set cgroup parent in a sane fashion
	if err := c.runtime.ociRuntime.createContainer(c, CgroupParent); err != nil {
		return err
	}

	logrus.Debugf("Created container %s in runc", c.ID())

	c.state.State = ContainerStateCreated

	return c.save()
}

// Start starts a container
func (c *Container) Start() error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	// Container must be created or stopped to be started
	if !(c.state.State == ContainerStateCreated || c.state.State == ContainerStateStopped) {
		return errors.Wrapf(ErrCtrStateInvalid, "container %s must be in Created or Stopped state to be started", c.ID())
	}

	// Mount storage for the container
	if err := c.mountStorage(); err != nil {
		return err
	}

	if err := c.runtime.ociRuntime.startContainer(c); err != nil {
		return err
	}

	logrus.Debugf("Started container %s", c.ID())

	c.state.State = ContainerStateRunning

	return c.save()
}

// Stop uses the container's stop signal (or SIGTERM if no signal was specified)
// to stop the container, and if it has not stopped after the given timeout (in
// seconds), uses SIGKILL to attempt to forcibly stop the container.
// If timeout is 0, SIGKILL will be used immediately
func (c *Container) Stop(timeout uint) error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	logrus.Debugf("Stopping ctr %s with timeout %d", c.ID(), timeout)

	if c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateUnknown ||
		c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "can only stop created, running, or stopped containers")
	}

	if err := c.runtime.ociRuntime.stopContainer(c, timeout); err != nil {
		return err
	}

	// Sync the container's state to pick up return code
	if err := c.runtime.ociRuntime.updateContainerStatus(c); err != nil {
		return err
	}

	return c.cleanupStorage()
}

// Kill sends a signal to a container
func (c *Container) Kill(signal uint) error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if c.state.State != ContainerStateRunning {
		return errors.Wrapf(ErrCtrStateInvalid, "can only kill running containers")
	}

	return c.runtime.ociRuntime.killContainer(c, signal)
}

// Exec starts a new process inside the container
func (c *Container) Exec(tty, privileged bool, env, cmd []string, user string) error {
	var capList []string

	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	conState := c.state.State

	if conState != ContainerStateRunning {
		return errors.Errorf("cannot attach to container that is not running")
	}
	if privileged {
		capList = caps.GetAllCapabilities()
	}
	globalOpts := runcGlobalOptions{
		log: c.LogPath(),
	}
	execOpts := runcExecOptions{
		capAdd:  capList,
		pidFile: filepath.Join(c.state.RunDir, fmt.Sprintf("%s-execpid", stringid.GenerateNonCryptoID()[:12])),
		env:     env,
		user:    user,
		cwd:     c.config.Spec.Process.Cwd,
		tty:     tty,
	}

	return c.runtime.ociRuntime.execContainer(c, cmd, globalOpts, execOpts)
}

// Attach attaches to a container
// Returns fully qualified URL of streaming server for the container
func (c *Container) Attach(noStdin bool, keys string, attached chan<- bool) error {
	if !c.locked {
		c.lock.Lock()
		if err := c.syncContainer(); err != nil {
			c.lock.Unlock()
			return err
		}
		c.lock.Unlock()
	}

	if c.state.State != ContainerStateCreated &&
		c.state.State != ContainerStateRunning {
		return errors.Wrapf(ErrCtrStateInvalid, "can only attach to created or running containers")
	}

	// Check the validity of the provided keys first
	var err error
	detachKeys := []byte{}
	if len(keys) > 0 {
		detachKeys, err = term.ToBytes(keys)
		if err != nil {
			return errors.Wrapf(err, "invalid detach keys")
		}
	}

	resize := make(chan remotecommand.TerminalSize)
	defer close(resize)

	err = c.attachContainerSocket(resize, noStdin, detachKeys, attached)
	return err
}

// Mount mounts a container's filesystem on the host
// The path where the container has been mounted is returned
func (c *Container) Mount(label string) (string, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return "", err
		}
	}

	// return mountpoint if container already mounted
	if c.state.Mounted {
		return c.state.Mountpoint, nil
	}

	mountLabel := label
	if label == "" {
		mountLabel = c.config.MountLabel
	}
	mountPoint, err := c.runtime.store.Mount(c.ID(), mountLabel)
	if err != nil {
		return "", err
	}
	c.state.Mountpoint = mountPoint
	c.state.Mounted = true
	c.config.MountLabel = mountLabel

	if err := c.save(); err != nil {
		return "", err
	}

	return mountPoint, nil
}

// Unmount unmounts a container's filesystem on the host
func (c *Container) Unmount() error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if c.state.State == ContainerStateRunning || c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "cannot remove storage for container %s as it is running or paused", c.ID())
	}

	return c.cleanupStorage()
}

// Pause pauses a container
func (c *Container) Pause() error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "%q is already paused", c.ID())
	}
	if c.state.State != ContainerStateRunning && c.state.State != ContainerStateCreated {
		return errors.Wrapf(ErrCtrStateInvalid, "%q is not running/created, can't pause", c.state.State)
	}
	if err := c.runtime.ociRuntime.pauseContainer(c); err != nil {
		return err
	}

	logrus.Debugf("Paused container %s", c.ID())

	c.state.State = ContainerStatePaused

	return c.save()
}

// Unpause unpauses a container
func (c *Container) Unpause() error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if c.state.State != ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "%q is not paused, can't unpause", c.ID())
	}
	if err := c.runtime.ociRuntime.unpauseContainer(c); err != nil {
		return err
	}

	logrus.Debugf("Unpaused container %s", c.ID())

	c.state.State = ContainerStateRunning

	return c.save()
}

// Export exports a container's root filesystem as a tar archive
// The archive will be saved as a file at the given path
func (c *Container) Export(path string) error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	return c.export(path)
}

func (c *Container) export(path string) error {
	mountPoint := c.state.Mountpoint
	if !c.state.Mounted {
		mount, err := c.runtime.store.Mount(c.ID(), c.config.MountLabel)
		if err != nil {
			return errors.Wrapf(err, "error mounting container %q", c.ID())
		}
		mountPoint = mount
		defer func() {
			if err := c.runtime.store.Unmount(c.ID()); err != nil {
				logrus.Errorf("error unmounting container %q: %v", c.ID(), err)
			}
		}()
	}

	input, err := archive.Tar(mountPoint, archive.Uncompressed)
	if err != nil {
		return errors.Wrapf(err, "error reading container directory %q", c.ID())
	}

	outFile, err := os.Create(path)
	if err != nil {
		return errors.Wrapf(err, "error creating file %q", path)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, input)
	return err
}

// AddArtifact creates and writes to an artifact file for the container
func (c *Container) AddArtifact(name string, data []byte) error {
	if !c.valid {
		return ErrCtrRemoved
	}

	return ioutil.WriteFile(c.getArtifactPath(name), data, 0740)
}

// GetArtifact reads the specified artifact file from the container
func (c *Container) GetArtifact(name string) ([]byte, error) {
	if !c.valid {
		return nil, ErrCtrRemoved
	}

	return ioutil.ReadFile(c.getArtifactPath(name))
}

// RemoveArtifact deletes the specified artifacts file
func (c *Container) RemoveArtifact(name string) error {
	if !c.valid {
		return ErrCtrRemoved
	}

	return os.Remove(c.getArtifactPath(name))
}

func (c *Container) getArtifactPath(name string) string {
	return filepath.Join(c.config.StaticDir, artifactsDir, name)
}

// Inspect a container for low-level information
func (c *Container) Inspect(size bool) (*ContainerInspectData, error) {
	if !c.locked {
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

// Commit commits the changes between a container and its image, creating a new
// image
func (c *Container) Commit(pause bool, options CopyOptions) error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if c.state.State == ContainerStateRunning && pause {
		if err := c.runtime.ociRuntime.pauseContainer(c); err != nil {
			return errors.Wrapf(err, "error pausing container %q", c.ID())
		}
		defer func() {
			if err := c.runtime.ociRuntime.unpauseContainer(c); err != nil {
				logrus.Errorf("error unpausing container %q: %v", c.ID(), err)
			}
		}()
	}

	tempFile, err := ioutil.TempFile(c.runtime.config.TmpDir, "podman-commit")
	if err != nil {
		return errors.Wrapf(err, "error creating temp file")
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if err := c.export(tempFile.Name()); err != nil {
		return err
	}
	return c.runtime.ImportImage(tempFile.Name(), options)
}

// Wait blocks on a container to exit and returns its exit code
func (c *Container) Wait() (int32, error) {
	if !c.valid {
		return -1, ErrCtrRemoved
	}

	err := wait.PollImmediateInfinite(1,
		func() (bool, error) {
			stopped, err := c.isStopped()
			if err != nil {
				return false, err
			}
			if !stopped {
				return false, nil
			} else { // nolint
				return true, nil // nolint
			} // nolint
		},
	)
	if err != nil {
		return 0, err
	}
	exitCode := c.state.ExitCode
	return exitCode, nil
}

func (c *Container) isStopped() (bool, error) {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
	}
	err := c.syncContainer()
	if err != nil {
		return true, err
	}
	return c.state.State == ContainerStateStopped, nil
}

// save container state to the database
func (c *Container) save() error {
	if err := c.runtime.state.SaveContainer(c); err != nil {
		return errors.Wrapf(err, "error saving container %s state", c.ID())
	}
	return nil
}

// mountStorage sets up the container's root filesystem
// It mounts the image and any other requested mounts
// TODO: Add ability to override mount label so we can use this for Mount() too
// TODO: Can we use this for export? Copying SHM into the export might not be
// good
func (c *Container) mountStorage() (err error) {
	// Container already mounted, nothing to do
	if c.state.Mounted {
		return nil
	}

	// TODO: generalize this mount code so it will mount every mount in ctr.config.Mounts

	mounted, err := mount.Mounted(c.config.ShmDir)
	if err != nil {
		return errors.Wrapf(err, "unable to determine if %q is mounted", c.config.ShmDir)
	}

	if !mounted {
		shmOptions := fmt.Sprintf("mode=1777,size=%d", c.config.ShmSize)
		if err := unix.Mount("shm", c.config.ShmDir, "tmpfs", unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV,
			label.FormatMountLabel(shmOptions, c.config.MountLabel)); err != nil {
			return errors.Wrapf(err, "failed to mount shm tmpfs %q", c.config.ShmDir)
		}
	}

	mountPoint, err := c.runtime.storageService.MountContainerImage(c.ID())
	if err != nil {
		return errors.Wrapf(err, "error mounting storage for container %s", c.ID())
	}
	c.state.Mounted = true
	c.state.Mountpoint = mountPoint

	logrus.Debugf("Created root filesystem for container %s at %s", c.ID(), c.state.Mountpoint)

	defer func() {
		if err != nil {
			if err2 := c.cleanupStorage(); err2 != nil {
				logrus.Errorf("Error unmounting storage for container %s: %v", c.ID(), err)
			}
		}
	}()

	return c.save()
}

// CleanupStorage unmounts all mount points in container and cleans up container storage
func (c *Container) CleanupStorage() error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return err
		}
	}
	return c.cleanupStorage()
}

// cleanupStorage unmounts and cleans up the container's root filesystem
func (c *Container) cleanupStorage() error {
	if !c.state.Mounted {
		// Already unmounted, do nothing
		return nil
	}

	for _, mount := range c.config.Mounts {
		if err := unix.Unmount(mount, unix.MNT_DETACH); err != nil {
			if err != syscall.EINVAL {
				logrus.Warnf("container %s failed to unmount %s : %v", c.ID(), mount, err)
			}
		}
	}

	// Also unmount storage
	if err := c.runtime.storageService.UnmountContainerImage(c.ID()); err != nil {
		return errors.Wrapf(err, "error unmounting container %s root filesystem", c.ID())
	}

	c.state.Mountpoint = ""
	c.state.Mounted = false

	return c.save()
}

// CGroupPath returns a cgroups "path" for a given container.
func (c *Container) CGroupPath() cgroups.Path {
	return cgroups.StaticPath(filepath.Join(c.config.CgroupParent, fmt.Sprintf("libpod-conmon-%s", c.ID())))
}

// copyHostFileToRundir copies the provided file to the runtimedir
func (c *Container) copyHostFileToRundir(sourcePath string) (string, error) {
	destFileName := filepath.Join(c.state.RunDir, filepath.Base(sourcePath))
	if err := fileutils.CopyFile(sourcePath, destFileName); err != nil {
		return "", err
	}
	// Relabel runDirResolv for the container
	if err := label.Relabel(destFileName, c.config.MountLabel, false); err != nil {
		return "", err
	}
	return destFileName, nil
}

// StopTimeout returns a stop timeout field for this container
func (c *Container) StopTimeout() uint {
	return c.config.StopTimeout
}

// Batch starts a batch operation on the given container
// All commands in the passed function will execute under the same lock and
// without syncronyzing state after each operation
// This will result in substantial performance benefits when running numerous
// commands on the same container
// Note that the container passed into the Batch function cannot be removed
// during batched operations. runtime.RemoveContainer can only be called outside
// of Batch
// Any error returned by the given batch function will be returned unmodified by
// Batch
// As Batch normally disables updating the current state of the container, the
// Sync() function is provided to enable container state to be updated and
// checked within Batch.
func (c *Container) Batch(batchFunc func(*Container) error) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return err
	}

	newCtr := new(Container)
	newCtr.config = c.config
	newCtr.state = c.state
	newCtr.runtime = c.runtime
	newCtr.lock = c.lock
	newCtr.valid = true

	newCtr.locked = true

	if err := batchFunc(newCtr); err != nil {
		return err
	}

	newCtr.locked = false

	return c.save()
}

// Sync updates the current state of the container, checking whether its state
// has changed
// Sync can only be used inside Batch() - otherwise, it will be done
// automatically.
// When called outside Batch(), Sync() is a no-op
func (c *Container) Sync() error {
	if !c.locked {
		return nil
	}

	// If runc knows about the container, update its status in runc
	// And then save back to disk
	if (c.state.State != ContainerStateUnknown) &&
		(c.state.State != ContainerStateConfigured) {
		oldState := c.state.State
		// TODO: optionally replace this with a stat for the exit file
		if err := c.runtime.ociRuntime.updateContainerStatus(c); err != nil {
			return err
		}
		// Only save back to DB if state changed
		if c.state.State != oldState {
			if err := c.save(); err != nil {
				return err
			}
		}
	}

	return nil
}
