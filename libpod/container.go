package libpod

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"

	"github.com/containers/storage"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/term"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	crioAnnotations "github.com/projectatomic/libpod/pkg/annotations"
	"github.com/sirupsen/logrus"
	"github.com/ulule/deepcopier"
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
	config *containerConfig

	pod         *Pod
	runningSpec *spec.Spec

	state *containerRuntimeInfo

	// TODO move to storage.Locker from sync.Mutex
	valid   bool
	lock    sync.Mutex
	runtime *Runtime
}

// containerState contains the current state of the container
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
	Mounted bool `json:"-"`
	// MountPoint contains the path to the container's mounted storage
	Mountpoint string `json:"mountPoint,omitempty"`
	// StartedTime is the time the container was started
	StartedTime time.Time `json:"startedTime,omitempty"`
	// FinishedTime is the time the container finished executing
	FinishedTime time.Time `json:"finishedTime,omitempty"`
	// ExitCode is the exit code returned when the container stopped
	ExitCode int32 `json:"exitCode,omitempty"`

	// TODO: Save information about image used in container if one is used
}

// containerConfig contains all information that was used to create the
// container. It may not be changed once created.
// It is stored, read-only, on disk
type containerConfig struct {
	Spec *spec.Spec `json:"spec"`
	ID   string     `json:"id"`
	Name string     `json:"name"`
	// Information on the image used for the root filesystem
	RootfsImageID   string `json:"rootfsImageID,omitempty"`
	RootfsImageName string `json:"rootfsImageName,omitempty"`
	UseImageConfig  bool   `json:"useImageConfig"`
	// SELinux mount label for root filesystem
	MountLabel      string `json:"MountLabel,omitempty"`
	// Static directory for container content that will persist across
	// reboot
	StaticDir string `json:"staticDir"`
	// Whether to keep container STDIN open
	Stdin bool
	// Pod the container belongs to
	Pod string `json:"pod,omitempty"`
	// Labels is a set of key-value pairs providing additional information
	// about a container
	Labels map[string]string `json:"labels,omitempty"`
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

// ID returns the container's ID
func (c *Container) ID() string {
	return c.config.ID
}

// Name returns the container's name
func (c *Container) Name() string {
	return c.config.Name
}

// Spec returns the container's OCI runtime spec
// The spec returned is the one used to create the container. The running
// spec may differ slightly as mounts are added based on the image
func (c *Container) Spec() *spec.Spec {
	spec := new(spec.Spec)
	deepcopier.Copy(c.config.Spec).To(spec)

	return spec
}

// Labels returns the container's labels
func (c *Container) Labels() map[string]string {
	labels := make(map[string]string)
	for key, value := range c.config.Labels {
		labels[key] = value
	}

	return labels
}

// State returns the current state of the container
func (c *Container) State() (ContainerState, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// TODO uncomment when working
	// if err := c.runtime.ociRuntime.updateContainerStatus(c); err != nil {
	// 	return ContainerStateUnknown, err
	// }

	return c.state.State, nil
}

// The path to the container's root filesystem - where the OCI spec will be
// placed, amongst other things
func (c *Container) bundlePath() string {
	return c.state.RunDir
}

// Retrieves the path of the container's attach socket
func (c *Container) attachSocketPath() string {
	return filepath.Join(c.runtime.ociRuntime.socketsDir, c.ID(), "attach")
}

// Make a new container
func newContainer(rspec *spec.Spec) (*Container, error) {
	if rspec == nil {
		return nil, errors.Wrapf(ErrInvalidArg, "must provide a valid runtime spec to create container")
	}

	ctr := new(Container)
	ctr.config = new(containerConfig)
	ctr.state = new(containerRuntimeInfo)

	ctr.config.ID = stringid.GenerateNonCryptoID()
	ctr.config.Name = ctr.config.ID // TODO generate unique human-readable names

	ctr.config.Spec = new(spec.Spec)
	deepcopier.Copy(rspec).To(ctr.config.Spec)

	ctr.config.CreatedTime = time.Now()

	return ctr, nil
}

// Create container root filesystem for use
func (c *Container) setupStorage() error {
	c.lock.Lock()
	defer c.lock.Unlock()

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
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid", c.ID())
	}

	if c.state.State == ContainerStateRunning || c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "cannot remove storage for container %s as it is running or paused", c.ID())
	}

	if c.state.Mounted {
		if err := c.runtime.storageService.StopContainer(c.ID()); err != nil {
			return errors.Wrapf(err, "error unmounting container %s root filesystem", c.ID())
		}

		c.state.Mounted = false
	}

	if err := c.runtime.storageService.DeleteContainer(c.ID()); err != nil {
		return errors.Wrapf(err, "error removing container %s root filesystem", c.ID())
	}

	return nil
}

// Create creates a container in the OCI runtime
func (c *Container) Create() (err error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.valid {
		return errors.Wrapf(ErrCtrRemoved, "container %s is not valid", c.ID())
	}

	if c.state.State != ContainerStateConfigured {
		return errors.Wrapf(ErrCtrExists, "container %s has already been created in runtime", c.ID())
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
			if err2 := c.runtime.storageService.StopContainer(c.ID()); err2 != nil {
				logrus.Errorf("Error unmounting storage for container %s: %v", c.ID(), err2)
			}

			c.state.Mounted = false
			c.state.Mountpoint = ""
		}
	}()

	// Make the OCI runtime spec we will use
	c.runningSpec = new(spec.Spec)
	deepcopier.Copy(c.config.Spec).To(c.runningSpec)
	c.runningSpec.Root.Path = c.state.Mountpoint
	c.runningSpec.Annotations[crioAnnotations.Created] = c.config.CreatedTime.Format(time.RFC3339Nano)
	c.runningSpec.Annotations["org.opencontainers.image.stopSignal"] = fmt.Sprintf("%d", c.config.StopSignal)

	// Save the OCI spec to disk
	jsonPath := filepath.Join(c.bundlePath(), "config.json")
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

	// TODO should flush this state to disk here
	c.state.State = ContainerStateCreated

	return nil
}

// Start starts a container
func (c *Container) Start() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.valid {
		return ErrCtrRemoved
	}

	// Container must be created or stopped to be started
	if !(c.state.State == ContainerStateCreated || c.state.State == ContainerStateStopped) {
		return errors.Wrapf(ErrCtrStateInvalid, "container %s must be in Created or Stopped state to be started", c.ID())
	}

	if err := c.runtime.ociRuntime.startContainer(c); err != nil {
		return err
	}

	logrus.Debugf("Started container %s", c.ID())

	// TODO should flush state to disk here
	c.state.StartedTime = time.Now()
	c.state.State = ContainerStateRunning

	return nil
}

// Stop stops a container
func (c *Container) Stop() error {
	return ErrNotImplemented
}

// Kill sends a signal to a container
func (c *Container) Kill(signal uint) error {
	return ErrNotImplemented
}

// Exec starts a new process inside the container
// Returns fully qualified URL of streaming server for executed process
func (c *Container) Exec(cmd []string, tty bool, stdin bool) (string, error) {
	return "", ErrNotImplemented
}

// Attach attaches to a container
// Returns fully qualified URL of streaming server for the container
func (c *Container) Attach(noStdin bool, keys string, attached chan<- bool) error {
	// Check the validity of the provided keys first
	var err error
	detachKeys := []byte{}
	if len(keys) > 0 {
		detachKeys, err = term.ToBytes(keys)
		if err != nil {
			return errors.Wrapf(err, "invalid detach keys")
		}
	}
	cStatus := c.state.State

	if !(cStatus == ContainerStateRunning || cStatus == ContainerStateCreated) {
		return errors.Errorf("%s is not created or running", c.Name())
	}
	resize := make(chan remotecommand.TerminalSize)
	defer close(resize)
	err = c.attachContainerSocket(resize, noStdin, detachKeys, attached)

	return err

	// TODO
	// Re-enable this when mheon is done wth it
	//if err != nil {
	//	return err
	//}
	//c.ContainerStateToDisk(c)

	//return err
}

// Mount mounts a container's filesystem on the host
// The path where the container has been mounted is returned
func (c *Container) Mount() (string, error) {
	return "", ErrNotImplemented
}

// Pause pauses a container
func (c *Container) Pause() error {
	return ErrNotImplemented
}

// Unpause unpauses a container
func (c *Container) Unpause() error {
	return ErrNotImplemented
}

// Export exports a container's root filesystem as a tar archive
// The archive will be saved as a file at the given path
func (c *Container) Export(path string) error {
	return ErrNotImplemented
}

// Commit commits the changes between a container and its image, creating a new
// image
// If the container was not created from an image (for example,
// WithRootFSFromPath will create a container from a directory on the system),
// a new base image will be created from the contents of the container's
// filesystem
func (c *Container) Commit() (*storage.Image, error) {
	return nil, ErrNotImplemented
}
