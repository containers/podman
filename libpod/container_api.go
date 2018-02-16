package libpod

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/daemon/caps"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/term"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod/driver"
	crioAnnotations "github.com/projectatomic/libpod/pkg/annotations"
	"github.com/projectatomic/libpod/pkg/chrootuser"
	"github.com/projectatomic/libpod/pkg/inspect"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/remotecommand"
)

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
	runDirResolv, err := c.generateResolvConf()
	if err != nil {
		return err
	}

	// Copy /etc/hosts to the container's rundir
	runDirHosts, err := c.generateHosts()
	if err != nil {
		return errors.Wrapf(err, "unable to copy /etc/hosts to container space")
	}

	if c.Spec().Hostname == "" {
		id := c.ID()
		if len(c.ID()) > 11 {
			id = c.ID()[:12]
		}
		c.config.Spec.Hostname = id
	}
	runDirHostname, err := c.generateEtcHostname(c.config.Spec.Hostname)
	if err != nil {
		return errors.Wrapf(err, "unable to generate hostname file for container")
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
	// Bind hostname
	hostnameMnt := spec.Mount{
		Type:        "bind",
		Source:      runDirHostname,
		Destination: "/etc/hostname",
		Options:     []string{"rw", "bind"},
	}
	g.AddMount(hostnameMnt)

	// Bind builtin image volumes
	if c.config.ImageVolumes {
		if err = c.addImageVolumes(&g); err != nil {
			return errors.Wrapf(err, "error mounting image volumes")
		}
	}

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

	// Set the hostname in the env variables
	c.runningSpec.Process.Env = append(c.runningSpec.Process.Env, fmt.Sprintf("HOSTNAME=%s", c.config.Spec.Hostname))

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
	if err := c.runtime.ociRuntime.createContainer(c, c.config.CgroupParent); err != nil {
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

	// TODO remove this when we patch conmon to support restarting containers
	if c.state.State == ContainerStateStopped {
		return errors.Wrapf(ErrNotImplemented, "restarting a stopped container is not yet supported")
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
// to stop the container, and if it has not stopped after container's stop
// timeout, SIGKILL is used to attempt to forcibly stop the container
// Default stop timeout is 10 seconds, but can be overridden when the container
// is created
func (c *Container) Stop() error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	return c.stop(c.config.StopTimeout)
}

// StopWithTimeout is a version of Stop that allows a timeout to be specified
// manually. If timeout is 0, SIGKILL will be used immediately to kill the
// container.
func (c *Container) StopWithTimeout(timeout uint) error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	return c.stop(timeout)
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
	if c.state.State != ContainerStateRunning {
		return errors.Wrapf(ErrCtrStateInvalid, "%q is not running, can't pause", c.state.State)
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

// Inspect a container for low-level information
func (c *Container) Inspect(size bool) (*inspect.ContainerInspectData, error) {
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

// Cleanup unmounts all mount points in container and cleans up container storage
// It also cleans up the network stack
func (c *Container) Cleanup() error {
	if !c.locked {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	// Stop the container's network namespace (if it has one)
	if err := c.cleanupNetwork(); err != nil {
		logrus.Errorf("unable cleanup network for container %s: %q", c.ID(), err)
	}

	return c.cleanupStorage()
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
