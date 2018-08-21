package libpod

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containers/libpod/libpod/driver"
	"github.com/containers/libpod/pkg/chrootuser"
	"github.com/containers/libpod/pkg/inspect"
	"github.com/containers/storage/pkg/stringid"
	"github.com/docker/docker/daemon/caps"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/remotecommand"
)

// Init creates a container in the OCI runtime
func (c *Container) Init(ctx context.Context) (err error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if !(c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateStopped) {
		return errors.Wrapf(ErrCtrExists, "container %s has already been created in runtime", c.ID())
	}

	notRunning, err := c.checkDependenciesRunning()
	if err != nil {
		return errors.Wrapf(err, "error checking dependencies for container %s")
	}
	if len(notRunning) > 0 {
		depString := strings.Join(notRunning, ",")
		return errors.Wrapf(ErrCtrStateInvalid, "some dependencies of container %s are not started: %s", c.ID(), depString)
	}

	if err := c.prepare(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if err2 := c.cleanup(); err2 != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
			}
		}
	}()

	if c.state.State == ContainerStateStopped {
		// Reinitialize the container
		return c.reinit(ctx)
	}

	// Initialize the container for the first time
	return c.init(ctx)
}

// Start starts a container
// Start can start configured, created or stopped containers
// For configured containers, the container will be initialized first, then
// started
// Stopped containers will be deleted and re-created in runc, undergoing a fresh
// Init()
func (c *Container) Start(ctx context.Context) (err error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	// Container must be created or stopped to be started
	if !(c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateCreated ||
		c.state.State == ContainerStateStopped) {
		return errors.Wrapf(ErrCtrStateInvalid, "container %s must be in Created or Stopped state to be started", c.ID())
	}

	notRunning, err := c.checkDependenciesRunning()
	if err != nil {
		return errors.Wrapf(err, "error checking dependencies for container %s")
	}
	if len(notRunning) > 0 {
		depString := strings.Join(notRunning, ",")
		return errors.Wrapf(ErrCtrStateInvalid, "some dependencies of container %s are not started: %s", c.ID(), depString)
	}

	if err := c.prepare(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if err2 := c.cleanup(); err2 != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
			}
		}
	}()

	if c.state.State == ContainerStateStopped {
		// Reinitialize the container if we need to
		if err := c.reinit(ctx); err != nil {
			return err
		}
	} else if c.state.State == ContainerStateConfigured {
		// Or initialize it for the first time if necessary
		if err := c.init(ctx); err != nil {
			return err
		}
	}

	// Start the container
	return c.start()
}

// StartAndAttach starts a container and attaches to it
// StartAndAttach can start configured, created or stopped containers
// For configured containers, the container will be initialized first, then
// started
// Stopped containers will be deleted and re-created in runc, undergoing a fresh
// Init()
// If successful, an error channel will be returned containing the result of the
// attach call.
// The channel will be closed automatically after the result of attach has been
// sent
func (c *Container) StartAndAttach(ctx context.Context, streams *AttachStreams, keys string, resize <-chan remotecommand.TerminalSize) (attachResChan <-chan error, err error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	// Container must be created or stopped to be started
	if !(c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateCreated ||
		c.state.State == ContainerStateStopped) {
		return nil, errors.Wrapf(ErrCtrStateInvalid, "container %s must be in Created or Stopped state to be started", c.ID())
	}

	notRunning, err := c.checkDependenciesRunning()
	if err != nil {
		return nil, errors.Wrapf(err, "error checking dependencies for container %s")
	}
	if len(notRunning) > 0 {
		depString := strings.Join(notRunning, ",")
		return nil, errors.Wrapf(ErrCtrStateInvalid, "some dependencies of container %s are not started: %s", c.ID(), depString)
	}

	if err := c.prepare(); err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if err2 := c.cleanup(); err2 != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
			}
		}
	}()

	if c.state.State == ContainerStateStopped {
		// Reinitialize the container if we need to
		if err := c.reinit(ctx); err != nil {
			return nil, err
		}
	} else if c.state.State == ContainerStateConfigured {
		// Or initialize it for the first time if necessary
		if err := c.init(ctx); err != nil {
			return nil, err
		}
	}

	attachChan := make(chan error)

	// Attach to the container before starting it
	go func() {
		if err := c.attach(streams, keys, resize, true); err != nil {
			attachChan <- err
		}
		close(attachChan)
	}()

	return attachChan, nil
}

// Stop uses the container's stop signal (or SIGTERM if no signal was specified)
// to stop the container, and if it has not stopped after container's stop
// timeout, SIGKILL is used to attempt to forcibly stop the container
// Default stop timeout is 10 seconds, but can be overridden when the container
// is created
func (c *Container) Stop() error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateUnknown ||
		c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "can only stop created, running, or stopped containers")
	}

	if c.state.State == ContainerStateStopped {
		return ErrCtrStopped
	}

	return c.stop(c.config.StopTimeout)
}

// StopWithTimeout is a version of Stop that allows a timeout to be specified
// manually. If timeout is 0, SIGKILL will be used immediately to kill the
// container.
func (c *Container) StopWithTimeout(timeout uint) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if c.state.State == ContainerStateConfigured ||
		c.state.State == ContainerStateUnknown ||
		c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "can only stop created, running, or stopped containers")
	}

	if c.state.State == ContainerStateStopped {
		return ErrCtrStopped
	}

	return c.stop(timeout)
}

// Kill sends a signal to a container
func (c *Container) Kill(signal uint) error {
	if !c.batched {
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

	locked := false
	if !c.batched {
		locked = true

		c.lock.Lock()
		defer func() {
			if locked {
				c.lock.Unlock()
			}
		}()

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

	// If user was set, look it up in the container to get a UID to use on
	// the host
	hostUser := ""
	if user != "" {
		uid, gid, err := chrootuser.GetUser(c.state.Mountpoint, user)
		if err != nil {
			return errors.Wrapf(err, "error getting user to launch exec session as")
		}

		// runc expects user formatted as uid:gid
		hostUser = fmt.Sprintf("%d:%d", uid, gid)
	}

	// Generate exec session ID
	// Ensure we don't conflict with an existing session ID
	sessionID := stringid.GenerateNonCryptoID()
	found := true
	// This really ought to be a do-while, but Go doesn't have those...
	for found {
		found = false
		for id := range c.state.ExecSessions {
			if id == sessionID {
				found = true
				break
			}
		}
		if found == true {
			sessionID = stringid.GenerateNonCryptoID()
		}
	}

	logrus.Debugf("Creating new exec session in container %s with session id %s", c.ID(), sessionID)

	execCmd, err := c.runtime.ociRuntime.execContainer(c, cmd, capList, env, tty, hostUser, sessionID)
	if err != nil {
		return errors.Wrapf(err, "error creating exec command for container %s", c.ID())
	}

	if err := execCmd.Start(); err != nil {
		return errors.Wrapf(err, "error starting exec command for container %s", c.ID())
	}

	pidFile := c.execPidPath(sessionID)
	const pidWaitTimeout = 250

	// Wait until the runtime makes the pidfile
	// TODO: If runtime errors before the PID file is created, we have to
	// wait for timeout here
	if err := WaitForFile(pidFile, pidWaitTimeout*time.Millisecond); err != nil {
		logrus.Debugf("Timed out waiting for pidfile from runtime for container %s exec", c.ID())

		// Check if an error occurred in the process before we made a pidfile
		// TODO: Wait() here is a poor choice - is there a way to see if
		// a process has finished, instead of waiting for it to finish?
		if err := execCmd.Wait(); err != nil {
			return err
		}

		return errors.Wrapf(err, "timed out waiting for runtime to create pidfile for exec session in container %s", c.ID())
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
		c.state.ExecSessions = make(map[string]*ExecSession)
	}
	session := new(ExecSession)
	session.ID = sessionID
	session.Command = cmd
	session.PID = int(pid)
	c.state.ExecSessions[sessionID] = session
	if err := c.save(); err != nil {
		// Now we have a PID but we can't save it in the DB
		// TODO handle this better
		return errors.Wrapf(err, "error saving exec sessions %s for container %s", sessionID, c.ID())
	}

	logrus.Debugf("Successfully started exec session %s in container %s", sessionID, c.ID())

	// Unlock so other processes can use the container
	if !c.batched {
		c.lock.Unlock()
		locked = false
	}

	waitErr := execCmd.Wait()

	// Lock again
	if !c.batched {
		locked = true
		c.lock.Lock()
	}

	// Sync the container again to pick up changes in state
	if err := c.syncContainer(); err != nil {
		return errors.Wrapf(err, "error syncing container %s state to remove exec session %s", c.ID(), sessionID)
	}

	// Remove the exec session from state
	delete(c.state.ExecSessions, sessionID)
	if err := c.save(); err != nil {
		logrus.Errorf("Error removing exec session %s from container %s state: %v", sessionID, c.ID(), err)
	}

	return waitErr
}

// Attach attaches to a container
func (c *Container) Attach(streams *AttachStreams, keys string, resize <-chan remotecommand.TerminalSize) error {
	if !c.batched {
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

	return c.attach(streams, keys, resize, false)
}

// Mount mounts a container's filesystem on the host
// The path where the container has been mounted is returned
func (c *Container) Mount() (string, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return "", err
		}
	}

	return c.mount()
}

// Unmount unmounts a container's filesystem on the host
func (c *Container) Unmount(force bool) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if c.state.State == ContainerStateRunning || c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "cannot unmount storage for container %s as it is running or paused", c.ID())
	}

	// Check if we have active exec sessions
	if len(c.state.ExecSessions) != 0 {
		return errors.Wrapf(ErrCtrStateInvalid, "container %s has active exec sessions, refusing to unmount", c.ID())
	}

	if c.state.Mounted {
		mounted, err := c.runtime.storageService.MountedContainerImage(c.ID())
		if err != nil {
			return errors.Wrapf(err, "can't determine how many times %s is mounted, refusing to unmount", c.ID())
		}
		if mounted == 1 {
			return errors.Wrapf(err, "can't unmount %s last mount, it is still in use", c.ID())
		}
	}
	return c.unmount(force)
}

// Pause pauses a container
func (c *Container) Pause() error {
	if !c.batched {
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

	return c.pause()
}

// Unpause unpauses a container
func (c *Container) Unpause() error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if c.state.State != ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "%q is not paused, can't unpause", c.ID())
	}

	return c.unpause()
}

// Export exports a container's root filesystem as a tar archive
// The archive will be saved as a file at the given path
func (c *Container) Export(path string) error {
	if !c.batched {
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
			}
			return true, nil
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
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()
		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	// Check if state is good
	if c.state.State == ContainerStateRunning || c.state.State == ContainerStatePaused {
		return errors.Wrapf(ErrCtrStateInvalid, "container %s is running or paused, refusing to clean up", c.ID())
	}

	// Check if we have active exec sessions
	if len(c.state.ExecSessions) != 0 {
		return errors.Wrapf(ErrCtrStateInvalid, "container %s has active exec sessions, refusing to clean up", c.ID())
	}

	return c.cleanup()
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

	newCtr.batched = true

	if err := batchFunc(newCtr); err != nil {
		return err
	}

	newCtr.batched = false

	return c.save()
}

// Sync updates the current state of the container, checking whether its state
// has changed
// Sync can only be used inside Batch() - otherwise, it will be done
// automatically.
// When called outside Batch(), Sync() is a no-op
func (c *Container) Sync() error {
	if !c.batched {
		return nil
	}

	// If runtime knows about the container, update its status in runtime
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

// RestartWithTimeout restarts a running container and takes a given timeout in uint
func (c *Container) RestartWithTimeout(ctx context.Context, timeout uint) (err error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	notRunning, err := c.checkDependenciesRunning()
	if err != nil {
		return errors.Wrapf(err, "error checking dependencies for container %s")
	}
	if len(notRunning) > 0 {
		depString := strings.Join(notRunning, ",")
		return errors.Wrapf(ErrCtrStateInvalid, "some dependencies of container %s are not started: %s", c.ID(), depString)
	}
	return c.restartWithTimeout(ctx, timeout)
}

// Refresh refreshes a container's state in the database, restarting the
// container if it is running
func (c *Container) Refresh(ctx context.Context) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	wasCreated := false
	if c.state.State == ContainerStateCreated {
		wasCreated = true
	}
	wasRunning := false
	if c.state.State == ContainerStateRunning {
		wasRunning = true
	}
	wasPaused := false
	if c.state.State == ContainerStatePaused {
		wasPaused = true
	}

	// First, unpause the container if it's paused
	if c.state.State == ContainerStatePaused {
		if err := c.unpause(); err != nil {
			return err
		}
	}

	// Next, if the container is running, stop it
	if c.state.State == ContainerStateRunning {
		if err := c.stop(c.config.StopTimeout); err != nil {
			return err
		}
	}

	// If there are active exec sessions, we need to kill them
	if len(c.state.ExecSessions) > 0 {
		logrus.Infof("Killing %d exec sessions in container %s. They will not be restored after refresh.",
			len(c.state.ExecSessions), c.ID())
		if err := c.runtime.ociRuntime.execStopContainer(c, c.config.StopTimeout); err != nil {
			return err
		}
	}

	// If the container is in ContainerStateStopped, we need to delete it
	// from the runtime and clear conmon state
	if c.state.State == ContainerStateStopped {
		if err := c.delete(ctx); err != nil {
			return err
		}
		if err := c.removeConmonFiles(); err != nil {
			return err
		}
	}

	// Fire cleanup code one more time unconditionally to ensure we are good
	// to refresh
	if err := c.cleanup(); err != nil {
		return err
	}

	logrus.Debugf("Resetting state of container %s", c.ID())

	// We've finished unwinding the container back to its initial state
	// Now safe to refresh container state
	if err := resetState(c.state); err != nil {
		return errors.Wrapf(err, "error resetting state of container %s", c.ID())
	}
	if err := c.refresh(); err != nil {
		return err
	}

	logrus.Debugf("Successfully refresh container %s state")

	// Initialize the container if it was created in runc
	if wasCreated || wasRunning || wasPaused {
		if err := c.prepare(); err != nil {
			return err
		}
		if err := c.init(ctx); err != nil {
			return err
		}
	}

	// If the container was running before, start it
	if wasRunning || wasPaused {
		if err := c.start(); err != nil {
			return err
		}
	}

	// If the container was paused before, re-pause it
	if wasPaused {
		if err := c.pause(); err != nil {
			return err
		}
	}

	return nil
}
