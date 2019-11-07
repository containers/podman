package libpod

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/storage/pkg/stringid"
	"github.com/docker/docker/oci/caps"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
)

// Init creates a container in the OCI runtime
func (c *Container) Init(ctx context.Context) (err error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "containerInit")
	span.SetTag("struct", "container")
	defer span.Finish()

	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if !c.ensureState(define.ContainerStateConfigured, define.ContainerStateStopped, define.ContainerStateExited) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s has already been created in runtime", c.ID())
	}

	// don't recursively start
	if err := c.checkDependenciesAndHandleError(ctx); err != nil {
		return err
	}

	if err := c.prepare(); err != nil {
		if err2 := c.cleanup(ctx); err2 != nil {
			logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
		}
		return err
	}

	if c.state.State == define.ContainerStateStopped {
		// Reinitialize the container
		return c.reinit(ctx, false)
	}

	// Initialize the container for the first time
	return c.init(ctx, false)
}

// Start starts a container.
// Start can start configured, created or stopped containers.
// For configured containers, the container will be initialized first, then
// started.
// Stopped containers will be deleted and re-created in runc, undergoing a fresh
// Init().
// If recursive is set, Start will also start all containers this container depends on.
func (c *Container) Start(ctx context.Context, recursive bool) (err error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "containerStart")
	span.SetTag("struct", "container")
	defer span.Finish()

	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}
	if err := c.prepareToStart(ctx, recursive); err != nil {
		return err
	}

	// Start the container
	return c.start()
}

// StartAndAttach starts a container and attaches to it.
// StartAndAttach can start configured, created or stopped containers.
// For configured containers, the container will be initialized first, then
// started.
// Stopped containers will be deleted and re-created in runc, undergoing a fresh
// Init().
// If successful, an error channel will be returned containing the result of the
// attach call.
// The channel will be closed automatically after the result of attach has been
// sent.
// If recursive is set, StartAndAttach will also start all containers this container depends on.
func (c *Container) StartAndAttach(ctx context.Context, streams *AttachStreams, keys string, resize <-chan remotecommand.TerminalSize, recursive bool) (attachResChan <-chan error, err error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	if err := c.prepareToStart(ctx, recursive); err != nil {
		return nil, err
	}
	attachChan := make(chan error)

	// We need to ensure that we don't return until start() fired in attach.
	// Use a channel to sync
	startedChan := make(chan bool)

	// Attach to the container before starting it
	go func() {
		if err := c.attach(streams, keys, resize, true, startedChan); err != nil {
			attachChan <- err
		}
		close(attachChan)
	}()

	select {
	case err := <-attachChan:
		return nil, err
	case <-startedChan:
		c.newContainerEvent(events.Attach)
	}

	return attachChan, nil
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

	if err = c.checkDependenciesAndHandleError(ctx); err != nil {
		return err
	}

	return c.restartWithTimeout(ctx, timeout)
}

// Stop uses the container's stop signal (or SIGTERM if no signal was specified)
// to stop the container, and if it has not stopped after container's stop
// timeout, SIGKILL is used to attempt to forcibly stop the container
// Default stop timeout is 10 seconds, but can be overridden when the container
// is created
func (c *Container) Stop() error {
	// Stop with the container's given timeout
	return c.StopWithTimeout(c.config.StopTimeout)
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

	if c.ensureState(define.ContainerStateStopped, define.ContainerStateExited) {
		return define.ErrCtrStopped
	}

	if !c.ensureState(define.ContainerStateCreated, define.ContainerStateRunning) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "can only stop created or running containers. %s is in state %s", c.ID(), c.state.State.String())
	}

	return c.stop(timeout, false)
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

	// TODO: Is killing a paused container OK?
	if c.state.State != define.ContainerStateRunning {
		return errors.Wrapf(define.ErrCtrStateInvalid, "can only kill running containers. %s is in state %s", c.ID(), c.state.State.String())
	}

	// Hardcode all = false, we only use all when removing.
	if err := c.ociRuntime.KillContainer(c, signal, false); err != nil {
		return err
	}

	c.state.StoppedByUser = true

	c.newContainerEvent(events.Kill)

	return c.save()
}

// Exec starts a new process inside the container
// Returns an exit code and an error. If Exec was not able to exec in the container before a failure, an exit code of define.ExecErrorCodeCannotInvoke is returned.
// If another generic error happens, an exit code of define.ExecErrorCodeGeneric is returned.
// Sometimes, the $RUNTIME exec call errors, and if that is the case, the exit code is the exit code of the call.
// Otherwise, the exit code will be the exit code of the executed call inside of the container.
// TODO investigate allowing exec without attaching
func (c *Container) Exec(tty, privileged bool, env map[string]string, cmd []string, user, workDir string, streams *AttachStreams, preserveFDs uint, resize chan remotecommand.TerminalSize, detachKeys string) (int, error) {
	var capList []string
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return define.ExecErrorCodeCannotInvoke, err
		}
	}

	if c.state.State != define.ContainerStateRunning {
		return define.ExecErrorCodeCannotInvoke, errors.Wrapf(define.ErrCtrStateInvalid, "cannot exec into container that is not running")
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
		for id := range c.state.ExecSessions {
			if id == sessionID {
				found = true
				break
			}
		}
		if found {
			sessionID = stringid.GenerateNonCryptoID()
		}
	}

	logrus.Debugf("Creating new exec session in container %s with session id %s", c.ID(), sessionID)
	if err := c.createExecBundle(sessionID); err != nil {
		return define.ExecErrorCodeCannotInvoke, err
	}

	defer func() {
		// cleanup exec bundle
		if err := c.cleanupExecBundle(sessionID); err != nil {
			logrus.Errorf("Error removing exec session %s bundle path for container %s: %v", sessionID, c.ID(), err)
		}
	}()

	// if the user is empty, we should inherit the user that the container is currently running with
	if user == "" {
		user = c.config.User
	}

	opts := new(ExecOptions)
	opts.Cmd = cmd
	opts.CapAdd = capList
	opts.Env = env
	opts.Terminal = tty
	opts.Cwd = workDir
	opts.User = user
	opts.Streams = streams
	opts.PreserveFDs = preserveFDs
	opts.Resize = resize
	opts.DetachKeys = detachKeys

	pid, attachChan, err := c.ociRuntime.ExecContainer(c, sessionID, opts)
	if err != nil {
		ec := define.ExecErrorCodeGeneric
		// Conmon will pass a non-zero exit code from the runtime as a pid here.
		// we differentiate a pid with an exit code by sending it as negative, so reverse
		// that change and return the exit code the runtime failed with.
		if pid < 0 {
			ec = -1 * pid
		}
		return ec, err
	}

	// We have the PID, add it to state
	if c.state.ExecSessions == nil {
		c.state.ExecSessions = make(map[string]*ExecSession)
	}
	session := new(ExecSession)
	session.ID = sessionID
	session.Command = cmd
	session.PID = pid
	c.state.ExecSessions[sessionID] = session
	if err := c.save(); err != nil {
		// Now we have a PID but we can't save it in the DB
		// TODO handle this better
		return define.ExecErrorCodeGeneric, errors.Wrapf(err, "error saving exec sessions %s for container %s", sessionID, c.ID())
	}
	c.newContainerEvent(events.Exec)
	logrus.Debugf("Successfully started exec session %s in container %s", sessionID, c.ID())

	// Unlock so other processes can use the container
	if !c.batched {
		c.lock.Unlock()
	}

	lastErr := <-attachChan

	exitCode, err := c.readExecExitCode(sessionID)
	if err != nil {
		if lastErr != nil {
			logrus.Errorf(lastErr.Error())
		}
		lastErr = err
	}
	if exitCode != 0 {
		if lastErr != nil {
			logrus.Errorf(lastErr.Error())
		}
		lastErr = errors.Wrapf(define.ErrOCIRuntime, "non zero exit code: %d", exitCode)
	}

	// Lock again
	if !c.batched {
		c.lock.Lock()
	}

	// Sync the container again to pick up changes in state
	if err := c.syncContainer(); err != nil {
		logrus.Errorf("error syncing container %s state to remove exec session %s", c.ID(), sessionID)
		return exitCode, lastErr
	}

	// Remove the exec session from state
	delete(c.state.ExecSessions, sessionID)
	if err := c.save(); err != nil {
		logrus.Errorf("Error removing exec session %s from container %s state: %v", sessionID, c.ID(), err)
	}
	return exitCode, lastErr
}

// AttachStreams contains streams that will be attached to the container
type AttachStreams struct {
	// OutputStream will be attached to container's STDOUT
	OutputStream io.WriteCloser
	// ErrorStream will be attached to container's STDERR
	ErrorStream io.WriteCloser
	// InputStream will be attached to container's STDIN
	InputStream *bufio.Reader
	// AttachOutput is whether to attach to STDOUT
	// If false, stdout will not be attached
	AttachOutput bool
	// AttachError is whether to attach to STDERR
	// If false, stdout will not be attached
	AttachError bool
	// AttachInput is whether to attach to STDIN
	// If false, stdout will not be attached
	AttachInput bool
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

	if !c.ensureState(define.ContainerStateCreated, define.ContainerStateRunning) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "can only attach to created or running containers")
	}

	defer c.newContainerEvent(events.Attach)
	return c.attach(streams, keys, resize, false, nil)
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
	defer c.newContainerEvent(events.Mount)
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

	if c.state.Mounted {
		mounted, err := c.runtime.storageService.MountedContainerImage(c.ID())
		if err != nil {
			return errors.Wrapf(err, "can't determine how many times %s is mounted, refusing to unmount", c.ID())
		}
		if mounted == 1 {
			if c.ensureState(define.ContainerStateRunning, define.ContainerStatePaused) {
				return errors.Wrapf(define.ErrCtrStateInvalid, "cannot unmount storage for container %s as it is running or paused", c.ID())
			}
			if len(c.state.ExecSessions) != 0 {
				return errors.Wrapf(define.ErrCtrStateInvalid, "container %s has active exec sessions, refusing to unmount", c.ID())
			}
			return errors.Wrapf(define.ErrInternal, "can't unmount %s last mount, it is still in use", c.ID())
		}
	}
	defer c.newContainerEvent(events.Unmount)
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

	if c.state.State == define.ContainerStatePaused {
		return errors.Wrapf(define.ErrCtrStateInvalid, "%q is already paused", c.ID())
	}
	if c.state.State != define.ContainerStateRunning {
		return errors.Wrapf(define.ErrCtrStateInvalid, "%q is not running, can't pause", c.state.State)
	}
	defer c.newContainerEvent(events.Pause)
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

	if c.state.State != define.ContainerStatePaused {
		return errors.Wrapf(define.ErrCtrStateInvalid, "%q is not paused, can't unpause", c.ID())
	}
	defer c.newContainerEvent(events.Unpause)
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
	defer c.newContainerEvent(events.Export)
	return c.export(path)
}

// AddArtifact creates and writes to an artifact file for the container
func (c *Container) AddArtifact(name string, data []byte) error {
	if !c.valid {
		return define.ErrCtrRemoved
	}

	return ioutil.WriteFile(c.getArtifactPath(name), data, 0740)
}

// GetArtifact reads the specified artifact file from the container
func (c *Container) GetArtifact(name string) ([]byte, error) {
	if !c.valid {
		return nil, define.ErrCtrRemoved
	}

	return ioutil.ReadFile(c.getArtifactPath(name))
}

// RemoveArtifact deletes the specified artifacts file
func (c *Container) RemoveArtifact(name string) error {
	if !c.valid {
		return define.ErrCtrRemoved
	}

	return os.Remove(c.getArtifactPath(name))
}

// Wait blocks until the container exits and returns its exit code.
func (c *Container) Wait() (int32, error) {
	return c.WaitWithInterval(DefaultWaitInterval)
}

// WaitWithInterval blocks until the container to exit and returns its exit
// code. The argument is the interval at which checks the container's status.
func (c *Container) WaitWithInterval(waitTimeout time.Duration) (int32, error) {
	if !c.valid {
		return -1, define.ErrCtrRemoved
	}

	exitFile, err := c.exitFilePath()
	if err != nil {
		return -1, err
	}
	chWait := make(chan error, 1)

	defer close(chWait)

	for {
		// ignore errors here, it is only used to avoid waiting
		// too long.
		_, _ = WaitForFile(exitFile, chWait, waitTimeout)

		stopped, err := c.isStopped()
		if err != nil {
			return -1, err
		}
		if stopped {
			return c.state.ExitCode, nil
		}
	}
}

// Cleanup unmounts all mount points in container and cleans up container storage
// It also cleans up the network stack
func (c *Container) Cleanup(ctx context.Context) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	// Check if state is good
	if !c.ensureState(define.ContainerStateConfigured, define.ContainerStateCreated, define.ContainerStateStopped, define.ContainerStateExited) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s is running or paused, refusing to clean up", c.ID())
	}

	// Handle restart policy.
	// Returns a bool indicating whether we actually restarted.
	// If we did, don't proceed to cleanup - just exit.
	didRestart, err := c.handleRestartPolicy(ctx)
	if err != nil {
		return err
	}
	if didRestart {
		return nil
	}

	// If we didn't restart, we perform a normal cleanup

	// Check if we have active exec sessions
	if len(c.state.ExecSessions) != 0 {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s has active exec sessions, refusing to clean up", c.ID())
	}
	defer c.newContainerEvent(events.Cleanup)
	return c.cleanup(ctx)
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
	newCtr.ociRuntime = c.ociRuntime
	newCtr.lock = c.lock
	newCtr.valid = true

	newCtr.batched = true
	err := batchFunc(newCtr)
	newCtr.batched = false

	return err
}

// Sync updates the status of a container by querying the OCI runtime.
// If the container has not been created inside the OCI runtime, nothing will be
// done.
// Most of the time, Podman does not explicitly query the OCI runtime for
// container status, and instead relies upon exit files created by conmon.
// This can cause a disconnect between running state and what Podman sees in
// cases where Conmon was killed unexpected, or runc was upgraded.
// Running a manual Sync() ensures that container state will be correct in
// such situations.
func (c *Container) Sync() error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()
	}

	// If runtime knows about the container, update its status in runtime
	// And then save back to disk
	if c.ensureState(define.ContainerStateCreated, define.ContainerStateRunning, define.ContainerStatePaused, define.ContainerStateStopped) {
		oldState := c.state.State
		if err := c.ociRuntime.UpdateContainerStatus(c); err != nil {
			return err
		}
		// Only save back to DB if state changed
		if c.state.State != oldState {
			if err := c.save(); err != nil {
				return err
			}
		}
	}

	defer c.newContainerEvent(events.Sync)
	return nil
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
	if c.state.State == define.ContainerStateCreated {
		wasCreated = true
	}
	wasRunning := false
	if c.state.State == define.ContainerStateRunning {
		wasRunning = true
	}
	wasPaused := false
	if c.state.State == define.ContainerStatePaused {
		wasPaused = true
	}

	// First, unpause the container if it's paused
	if c.state.State == define.ContainerStatePaused {
		if err := c.unpause(); err != nil {
			return err
		}
	}

	// Next, if the container is running, stop it
	if c.state.State == define.ContainerStateRunning {
		if err := c.stop(c.config.StopTimeout, false); err != nil {
			return err
		}
	}

	// If there are active exec sessions, we need to kill them
	if len(c.state.ExecSessions) > 0 {
		logrus.Infof("Killing %d exec sessions in container %s. They will not be restored after refresh.",
			len(c.state.ExecSessions), c.ID())
	}
	for _, session := range c.state.ExecSessions {
		if err := c.ociRuntime.ExecStopContainer(c, session.ID, c.StopTimeout()); err != nil {
			return errors.Wrapf(err, "error stopping exec session %s of container %s", session.ID, c.ID())
		}
	}

	// If the container is in ContainerStateStopped, we need to delete it
	// from the runtime and clear conmon state
	if c.state.State == define.ContainerStateStopped {
		if err := c.delete(ctx); err != nil {
			return err
		}
		if err := c.removeConmonFiles(); err != nil {
			return err
		}
	}

	// Fire cleanup code one more time unconditionally to ensure we are good
	// to refresh
	if err := c.cleanup(ctx); err != nil {
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

	logrus.Debugf("Successfully refresh container %s state", c.ID())

	// Initialize the container if it was created in runc
	if wasCreated || wasRunning || wasPaused {
		if err := c.prepare(); err != nil {
			return err
		}
		if err := c.init(ctx, false); err != nil {
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

// ContainerCheckpointOptions is a struct used to pass the parameters
// for checkpointing (and restoring) to the corresponding functions
type ContainerCheckpointOptions struct {
	// Keep tells the API to not delete checkpoint artifacts
	Keep bool
	// KeepRunning tells the API to keep the container running
	// after writing the checkpoint to disk
	KeepRunning bool
	// TCPEstablished tells the API to checkpoint a container
	// even if it contains established TCP connections
	TCPEstablished bool
	// TargetFile tells the API to read (or write) the checkpoint image
	// from (or to) the filename set in TargetFile
	TargetFile string
	// Name tells the API that during restore from an exported
	// checkpoint archive a new name should be used for the
	// restored container
	Name string
	// IgnoreRootfs tells the API to not export changes to
	// the container's root file-system (or to not import)
	IgnoreRootfs bool
	// IgnoreStaticIP tells the API to ignore the IP set
	// during 'podman run' with '--ip'. This is especially
	// important to be able to restore a container multiple
	// times with '--import --name'.
	IgnoreStaticIP bool
	// IgnoreStaticMAC tells the API to ignore the MAC set
	// during 'podman run' with '--mac-address'. This is especially
	// important to be able to restore a container multiple
	// times with '--import --name'.
	IgnoreStaticMAC bool
}

// Checkpoint checkpoints a container
func (c *Container) Checkpoint(ctx context.Context, options ContainerCheckpointOptions) error {
	logrus.Debugf("Trying to checkpoint container %s", c.ID())

	if options.TargetFile != "" {
		if err := c.prepareCheckpointExport(); err != nil {
			return err
		}
	}

	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}
	defer c.newContainerEvent(events.Checkpoint)
	return c.checkpoint(ctx, options)
}

// Restore restores a container
func (c *Container) Restore(ctx context.Context, options ContainerCheckpointOptions) (err error) {
	logrus.Debugf("Trying to restore container %s", c.ID())
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}
	defer c.newContainerEvent(events.Restore)
	return c.restore(ctx, options)
}
