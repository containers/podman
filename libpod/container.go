package libpod

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/term"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	crioAnnotations "github.com/projectatomic/libpod/pkg/annotations"
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
)

// Container is a single OCI container
type Container struct {
	config *ContainerConfig

	pod         *Pod
	runningSpec *spec.Spec

	state *containerRuntimeInfo

	// TODO move to storage.Locker from sync.Mutex
	valid   bool
	lock    sync.Mutex
	runtime *Runtime
}

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
	// TODO: Save information about image used in container if one is used
}

// ContainerConfig contains all information that was used to create the
// container. It may not be changed once created.
// It is stored, read-only, on disk
type ContainerConfig struct {
	Spec *spec.Spec `json:"spec"`
	ID   string     `json:"id"`
	Name string     `json:"name"`
	// Information on the image used for the root filesystem
	RootfsImageID   string `json:"rootfsImageID,omitempty"`
	RootfsImageName string `json:"rootfsImageName,omitempty"`
	UseImageConfig  bool   `json:"useImageConfig"`
	// SELinux process label for container
	ProcessLabel string `json:"ProcessLabel,omitempty"`
	// SELinux mount label for root filesystem
	MountLabel string `json:"MountLabel,omitempty"`
	// Src path to be mounted on /dev/shm in container
	ShmDir string `json:"ShmDir,omitempty"`
	// Static directory for container content that will persist across
	// reboot
	StaticDir string `json:"staticDir"`
	// Whether to keep container STDIN open
	Stdin bool `json:"stdin,omitempty"`
	// Pod the container belongs to
	Pod string `json:"pod,omitempty"`
	// Labels is a set of key-value pairs providing additional information
	// about a container
	Labels map[string]string `json:"labels,omitempty"`
	// Mounts list contains all additional mounts by the container runtime.
	Mounts []string
	// StopSignal is the signal that will be used to stop the container
	StopSignal uint `json:"stopSignal,omitempty"`
	// Shared namespaces with container
	SharedNamespaceCtr *string           `json:"shareNamespacesWith,omitempty"`
	SharedNamespaceMap map[string]string `json:"sharedNamespaces"`
	// Time container was created
	CreatedTime time.Time `json:"createdTime"`
	// TODO save log location here and pass into OCI code
	// TODO allow overriding of log path
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

// ShmDir returns the sources path to be mounted on /dev/shm in container
func (c *Container) ShmDir() string {
	return c.config.ShmDir
}

// ProcessLabel returns the selinux ProcessLabel of the container
func (c *Container) ProcessLabel() string {
	return c.config.ProcessLabel
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

// LogPath returns the path to the container's log file
// This file will only be present after Init() is called to create the container
// in runc
func (c *Container) LogPath() string {
	// TODO store this in state and allow overriding
	return c.logPath()
}

// ExitCode returns the exit code of the container as
// an int32
func (c *Container) ExitCode() (int32, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return 0, errors.Wrapf(err, "error updating container %s state", c.ID())
	}
	return c.state.ExitCode, nil
}

// Mounted returns a bool as to if the container's storage
// is mounted
func (c *Container) Mounted() (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return false, errors.Wrapf(err, "error updating container %s state", c.ID())
	}
	return c.state.Mounted, nil
}

// Mountpoint returns the path to the container's mounted
// storage as a string
func (c *Container) Mountpoint() (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return "", errors.Wrapf(err, "error updating container %s state", c.ID())
	}
	return c.state.Mountpoint, nil
}

// StartedTime is the time the container was started
func (c *Container) StartedTime() (time.Time, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return time.Time{}, errors.Wrapf(err, "error updating container %s state", c.ID())
	}
	return c.state.StartedTime, nil
}

// FinishedTime is the time the container was stopped
func (c *Container) FinishedTime() (time.Time, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return time.Time{}, errors.Wrapf(err, "error updating container %s state", c.ID())
	}
	return c.state.FinishedTime, nil
}

// State returns the current state of the container
func (c *Container) State() (ContainerState, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return ContainerStateUnknown, err
	}
	return c.state.State, nil
}

// PID returns the PID of the container
// An error is returned if the container is not running
func (c *Container) PID() (int, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return -1, err
	}

	return c.state.PID, nil
}

// MountPoint returns the mount point of the continer
func (c *Container) MountPoint() (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return "", errors.Wrapf(err, "error updating container %s state", c.ID())
	}
	return c.state.Mountpoint, nil
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
func (c *Container) syncContainer() error {
	if err := c.runtime.state.UpdateContainer(c); err != nil {
		return err
	}
	// If runc knows about the container, update its status in runc
	// And then save back to disk
	if (c.state.State != ContainerStateUnknown) &&
		(c.state.State != ContainerStateConfigured) {
		if err := c.runtime.ociRuntime.updateContainerStatus(c); err != nil {
			return err
		}
		if err := c.save(); err != nil {
			return err
		}
	}

	if !c.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid", c.ID())
	}

	return nil
}

// Make a new container
func newContainer(rspec *spec.Spec) (*Container, error) {
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
		return errors.Wrapf(ErrCtrRemoved, "container %s has been removed from the state", c.ID())
	}

	// We need to get the container's temporary directory from c/storage
	// It was lost in the reboot and must be recreated
	dir, err := c.runtime.storageService.GetRunDir(c.ID())
	if err != nil {
		return errors.Wrapf(err, "error retrieving temporary directory for container %s", c.ID())
	}
	c.state.RunDir = dir

	// The container is no longer mounted
	c.state.Mounted = false
	c.state.Mountpoint = ""

	// The container is no longe running
	c.state.PID = 0

	// Check the container's state. If it's not created in runc yet, we're
	// done
	if c.state.State == ContainerStateConfigured {
		if err := c.runtime.state.SaveContainer(c); err != nil {
			return errors.Wrapf(err, "error refreshing state for container %s", c.ID())
		}

		return nil
	}

	// The container must be recreated in runc
	if err := c.init(); err != nil {
		return err
	}

	if err := c.runtime.state.SaveContainer(c); err != nil {
		return errors.Wrapf(err, "error refreshing state for container %s", c.ID())
	}

	return nil
}

// Init creates a container in the OCI runtime
func (c *Container) Init() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return err
	}

	if c.state.State != ContainerStateConfigured {
		return errors.Wrapf(ErrCtrExists, "container %s has already been created in runtime", c.ID())
	}

	return c.init()
}

// Creates container in OCI runtime
// Internal only - does not lock or check state
func (c *Container) init() (err error) {
	if err := c.mountStorage(); err != nil {
		return err
	}

	// Make the OCI runtime spec we will use
	g := generate.NewFromSpec(c.config.Spec)
	// Mount ShmDir from host into container
	g.AddBindMount(c.config.ShmDir, "/dev/shm", []string{"rw"})
	c.runningSpec = g.Spec()
	c.runningSpec.Root.Path = c.state.Mountpoint
	c.runningSpec.Annotations[crioAnnotations.Created] = c.config.CreatedTime.Format(time.RFC3339Nano)
	c.runningSpec.Annotations["org.opencontainers.image.stopSignal"] = fmt.Sprintf("%d", c.config.StopSignal)

	// Save the OCI spec to disk
	jsonPath := filepath.Join(c.bundlePath(), "config.json")
	// If the OCI spec already exists, replace it
	if _, err := os.Stat(jsonPath); err != nil {
		if !os.IsNotExist(err) {
			return errors.Wrapf(err, "error doing stat on container %s spec", c.ID())
		}
	} else {
		// No error, the spec exists. Remove so we can replace.
		if err := os.Remove(jsonPath); err != nil {
			return errors.Wrapf(err, "error replacing spec of container %s", c.ID())
		}
	}
	fileJSON, err := json.Marshal(c.runningSpec)
	if err != nil {
		return errors.Wrapf(err, "error exporting runtime spec for container %s to JSON", c.ID())
	}
	if err := ioutil.WriteFile(jsonPath, fileJSON, 0644); err != nil {
		return errors.Wrapf(err, "error writing runtime spec JSON to file for container %s", c.ID())
	}
	c.state.ConfigPath = jsonPath

	logrus.Debugf("Created OCI spec for container %s at %s", c.ID(), jsonPath)

	// With the spec complete, do an OCI create
	// TODO set cgroup parent in a sane fashion
	if err := c.runtime.ociRuntime.createContainer(c, "/libpod_parent"); err != nil {
		return err
	}

	logrus.Debugf("Created container %s in runc", c.ID())

	c.state.State = ContainerStateCreated

	return c.save()
}

// Start starts a container
func (c *Container) Start() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return err
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

	// Update container's state as it should be ContainerStateRunning now
	if err := c.runtime.ociRuntime.updateContainerStatus(c); err != nil {
		return err
	}

	return c.save()
}

// Stop uses the container's stop signal (or SIGTERM if no signal was specified)
// to stop the container, and if it has not stopped after the given timeout (in
// seconds), uses SIGKILL to attempt to forcibly stop the container.
// If timeout is 0, SIGKILL will be used immediately
func (c *Container) Stop(timeout int64) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return err
	}

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
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return err
	}

	if c.state.State != ContainerStateRunning {
		return errors.Wrapf(ErrCtrStateInvalid, "can only kill running containers")
	}

	return c.runtime.ociRuntime.killContainer(c, signal)
}

// Exec starts a new process inside the container
// Returns fully qualified URL of streaming server for executed process
func (c *Container) Exec(cmd []string, tty bool, stdin bool) (string, error) {
	return "", ErrNotImplemented
}

// Attach attaches to a container
// Returns fully qualified URL of streaming server for the container
func (c *Container) Attach(noStdin bool, keys string, attached chan<- bool) error {
	if err := c.syncContainer(); err != nil {
		return err
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
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return "", err
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
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return err
	}

	if c.state.State == ContainerStateRunning || c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "cannot remove storage for container %s as it is running or paused", c.ID())
	}

	return c.cleanupStorage()
}

// Pause pauses a container
func (c *Container) Pause() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return err
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

	// Update container's state as it should be ContainerStatePaused now
	if err := c.runtime.ociRuntime.updateContainerStatus(c); err != nil {
		return err
	}

	return c.save()
}

// Unpause unpauses a container
func (c *Container) Unpause() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return err
	}

	if c.state.State != ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "%q is not paused, can't unpause", c.ID())
	}
	if err := c.runtime.ociRuntime.unpauseContainer(c); err != nil {
		return err
	}

	logrus.Debugf("Unpaused container %s", c.ID())

	// Update container's state as it should be ContainerStateRunning now
	if err := c.runtime.ociRuntime.updateContainerStatus(c); err != nil {
		return err
	}

	return c.save()
}

// Export exports a container's root filesystem as a tar archive
// The archive will be saved as a file at the given path
func (c *Container) Export(path string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if err := c.syncContainer(); err != nil {
		return err
	}

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

// Commit commits the changes between a container and its image, creating a new
// image
func (c *Container) Commit() (*storage.Image, error) {
	return nil, ErrNotImplemented
}

// Wait blocks on a container to exit and returns its exit code
func (c *Container) Wait() (int32, error) {
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
	c.lock.Lock()
	defer c.lock.Unlock()
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
		shmOptions := "mode=1777,size=" + strconv.Itoa(DefaultShmSize)
		if err := unix.Mount("shm", c.config.ShmDir, "tmpfs", unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV,
			label.FormatMountLabel(shmOptions, c.config.MountLabel)); err != nil {
			return errors.Wrapf(err, "failed to mount shm tmpfs %q", c.config.ShmDir)
		}
	}

	mountPoint, err := c.runtime.storageService.StartContainer(c.ID())
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

	if err := c.runtime.state.SaveContainer(c); err != nil {
		return errors.Wrapf(err, "error saving container %s state", c.ID())
	}

	return nil
}

// CleanupStorage unmounts all mount points in container and cleans up container storage
func (c *Container) CleanupStorage() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if err := c.syncContainer(); err != nil {
		return err
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
	if err := c.runtime.storageService.StopContainer(c.ID()); err != nil {
		return errors.Wrapf(err, "error unmounting container %s root filesystem", c.ID())
	}

	c.state.Mountpoint = ""
	c.state.Mounted = false

	if err := c.runtime.state.SaveContainer(c); err != nil {
		return errors.Wrapf(err, "error saving container %s state", c.ID())
	}

	return nil
}
