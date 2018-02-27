package libpod

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/docker/docker/daemon/caps"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod/driver"
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

	runDirHostname, err := c.generateEtcHostname(c.Hostname())
	if err != nil {
		return errors.Wrapf(err, "unable to generate hostname file for container")
	}

	// Generate the OCI spec
	spec, err := c.generateSpec(runDirResolv, runDirHosts, runDirHostname)
	if err != nil {
		return err
	}
	c.runningSpec = spec

	// Save the OCI spec to disk
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
// TODO allow specifying streams to attach to
// TODO investigate allowing exec without attaching
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

	// TODO can probably relax this once we track exec sessions
	if conState != ContainerStateRunning {
		return errors.Errorf("cannot exec into container that is not running")
	}
	if privileged || c.config.Privileged {
		capList = caps.GetAllCapabilities()
	}

	// Generate exec session ID
	// Ensure we don't conflict with an existing session ID
	sessionID := stringid.GenerateNonCryptoID()
	found := true
	// This really ought to be a do-while, but Go doesn't have those...
	for found {
		found = false
		for id, _ := range c.state.ExecSessions {
			if id == sessionID {
				found = true
				break
			}
		}
		if found == true {
			sessionID = stringid.GenerateNonCryptoID()
		}
	}

	execCmd, err := c.runtime.ociRuntime.execContainer(c, cmd, capList, env, tty, user, sessionID)
	if err != nil {
		return errors.Wrapf(err, "error creating exec command for container %s", c.ID())
	}

	if err := execCmd.Start(); err != nil {
		return errors.Wrapf(err, "error starting exec command for container %s", c.ID())
	}

	pidFile := c.execPidPath(sessionID)
	const pidWaitTimeout = 250

	// Wait until runc makes the pidfile
	// TODO: If runc errors before the PID file is created, we have to wait for timeout here
	if err := WaitForFile(pidFile, pidWaitTimeout * time.Millisecond); err != nil {
		logrus.Debugf("Timed out waiting for pidfile from runc for container %s exec", c.ID())

		// Check if an error occurred in the process before we made a pidfile
		// TODO: Wait() here is a poor choice - is there a way to see if
		// a process has finished, instead of waiting for it to finish?
		if err := execCmd.Wait(); err != nil {
			return err
		}

		return errors.Wrapf(err, "timed out waiting for runc to create pidfile for exec session in container %s", c.ID())
	}

	// Pidfile exists, read it
	contents, err := ioutil.ReadFile(pidFile)
	if err != nil {
		// We don't know the PID of the exec session
		// However, it may still be alive
		// TODO handle this better
		return errors.Wrapf(err, "could not read pidfile for exec session %s in container %s", sessionID, c.ID())
	}
	pid, err := strconv.ParseInt(string(contents), 10, 32)
	if err != nil {
		// As above, we don't have a valid PID, but the exec session is likely still alive
		// TODO handle this better
		return errors.Wrapf(err, "error parsing PID of exec session %s in container %s", sessionID, c.ID())
	}

	// We have the PID, add it to state
	if c.state.ExecSessions == nil {
		c.state.ExecSessions = make(map[string]int)
	}
	c.state.ExecSessions[sessionID] = int(pid)
	if err := c.save(); err != nil {
		// Now we have a PID but we can't save it in the DB
		// TODO handle this better
		return errors.Wrapf(err, "error saving exec sessions %s for container %s", sessionID, c.ID())
	}

	waitErr := execCmd.Wait()

	// Remove the exec session from state
	delete(c.state.ExecSessions, sessionID)
	if err := c.save(); err != nil {
		logrus.Errorf("Error removing exec session %s from container %s state: %v", sessionID, c.ID(), err)
	}

	return waitErr
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
