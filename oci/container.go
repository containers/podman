package oci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/docker/docker/pkg/signal"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"k8s.io/apimachinery/pkg/fields"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

const (
	defaultStopSignal = "TERM"
)

// Container represents a runtime container.
type Container struct {
	id              string
	name            string
	logPath         string
	labels          fields.Set
	annotations     fields.Set
	crioAnnotations fields.Set
	image           string
	sandbox         string
	netns           ns.NetNS
	terminal        bool
	stdin           bool
	stdinOnce       bool
	privileged      bool
	trusted         bool
	state           *ContainerState
	metadata        *pb.ContainerMetadata
	opLock          sync.Locker
	// this is the /var/run/storage/... directory, erased on reboot
	bundlePath string
	// this is the /var/lib/storage/... directory
	dir        string
	stopSignal string
	imageName  string
	imageRef   string
	volumes    []ContainerVolume
	mountPoint string
	spec       *specs.Spec
}

// ContainerVolume is a bind mount for the container.
type ContainerVolume struct {
	ContainerPath string `json:"container_path"`
	HostPath      string `json:"host_path"`
	Readonly      bool   `json:"readonly"`
}

// ContainerState represents the status of a container.
type ContainerState struct {
	specs.State
	Created   time.Time `json:"created"`
	Started   time.Time `json:"started,omitempty"`
	Finished  time.Time `json:"finished,omitempty"`
	ExitCode  int32     `json:"exitCode,omitempty"`
	OOMKilled bool      `json:"oomKilled,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// NewContainer creates a container object.
func NewContainer(id string, name string, bundlePath string, logPath string, netns ns.NetNS, labels map[string]string, crioAnnotations map[string]string, annotations map[string]string, image string, imageName string, imageRef string, metadata *pb.ContainerMetadata, sandbox string, terminal bool, stdin bool, stdinOnce bool, privileged bool, trusted bool, dir string, created time.Time, stopSignal string) (*Container, error) {
	state := &ContainerState{}
	state.Created = created
	c := &Container{
		id:              id,
		name:            name,
		bundlePath:      bundlePath,
		logPath:         logPath,
		labels:          labels,
		sandbox:         sandbox,
		netns:           netns,
		terminal:        terminal,
		stdin:           stdin,
		stdinOnce:       stdinOnce,
		privileged:      privileged,
		trusted:         trusted,
		metadata:        metadata,
		annotations:     annotations,
		crioAnnotations: crioAnnotations,
		image:           image,
		imageName:       imageName,
		imageRef:        imageRef,
		dir:             dir,
		state:           state,
		stopSignal:      stopSignal,
		opLock:          new(sync.Mutex),
	}
	return c, nil
}

// SetSpec loads the OCI spec in the container struct
func (c *Container) SetSpec(s *specs.Spec) {
	c.spec = s
}

// Spec returns a copy of the spec for the container
func (c *Container) Spec() specs.Spec {
	return *c.spec
}

// GetStopSignal returns the container's own stop signal configured from the
// image configuration or the default one.
func (c *Container) GetStopSignal() string {
	if c.stopSignal == "" {
		return defaultStopSignal
	}
	cleanSignal := strings.TrimPrefix(strings.ToUpper(c.stopSignal), "SIG")
	_, ok := signal.SignalMap[cleanSignal]
	if !ok {
		return defaultStopSignal
	}
	return cleanSignal
}

// FromDisk restores container's state from disk
func (c *Container) FromDisk() error {
	jsonSource, err := os.Open(c.StatePath())
	if err != nil {
		return err
	}
	defer jsonSource.Close()

	dec := json.NewDecoder(jsonSource)
	return dec.Decode(c.state)
}

// StatePath returns the containers state.json path
func (c *Container) StatePath() string {
	return filepath.Join(c.dir, "state.json")
}

// CreatedAt returns the container creation time
func (c *Container) CreatedAt() time.Time {
	return c.state.Created
}

// Name returns the name of the container.
func (c *Container) Name() string {
	return c.name
}

// ID returns the id of the container.
func (c *Container) ID() string {
	return c.id
}

// BundlePath returns the bundlePath of the container.
func (c *Container) BundlePath() string {
	return c.bundlePath
}

// LogPath returns the log path of the container.
func (c *Container) LogPath() string {
	return c.logPath
}

// Labels returns the labels of the container.
func (c *Container) Labels() map[string]string {
	return c.labels
}

// Annotations returns the annotations of the container.
func (c *Container) Annotations() map[string]string {
	return c.annotations
}

// CrioAnnotations returns the crio annotations of the container.
func (c *Container) CrioAnnotations() map[string]string {
	return c.crioAnnotations
}

// Image returns the image of the container.
func (c *Container) Image() string {
	return c.image
}

// ImageName returns the image name of the container.
func (c *Container) ImageName() string {
	return c.imageName
}

// ImageRef returns the image ref of the container.
func (c *Container) ImageRef() string {
	return c.imageRef
}

// Sandbox returns the sandbox name of the container.
func (c *Container) Sandbox() string {
	return c.sandbox
}

// Dir returns the the dir of the container
func (c *Container) Dir() string {
	return c.dir
}

// NetNsPath returns the path to the network namespace of the container.
func (c *Container) NetNsPath() (string, error) {
	if c.state == nil {
		return "", fmt.Errorf("container state is not populated")
	}

	if c.netns == nil {
		return fmt.Sprintf("/proc/%d/ns/net", c.state.Pid), nil
	}

	return c.netns.Path(), nil
}

// Metadata returns the metadata of the container.
func (c *Container) Metadata() *pb.ContainerMetadata {
	return c.metadata
}

// State returns the state of the running container
func (c *Container) State() *ContainerState {
	c.opLock.Lock()
	defer c.opLock.Unlock()
	return c.state
}

// AddVolume adds a volume to list of container volumes.
func (c *Container) AddVolume(v ContainerVolume) {
	c.volumes = append(c.volumes, v)
}

// Volumes returns the list of container volumes.
func (c *Container) Volumes() []ContainerVolume {
	return c.volumes

}

// SetMountPoint sets the container mount point
func (c *Container) SetMountPoint(mp string) {
	c.mountPoint = mp
}

// MountPoint returns the container mount point
func (c *Container) MountPoint() string {
	return c.mountPoint
}

// SetState sets the conainer state
//
// XXX: DO NOT EVER USE THIS, THIS IS JUST USEFUL FOR MOCKING!!!
func (c *Container) SetState(state *ContainerState) {
	c.state = state
}
