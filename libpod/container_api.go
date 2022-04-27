package libpod

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/signal"
	"github.com/containers/storage/pkg/archive"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Init creates a container in the OCI runtime, moving a container from
// ContainerStateConfigured, ContainerStateStopped, or ContainerStateExited to
// ContainerStateCreated. Once in Created state, Conmon will be running, which
// allows the container to be attached to. The container can subsequently
// transition to ContainerStateRunning via Start(), or be transitioned back to
// ContainerStateConfigured by Cleanup() (which will stop conmon and unmount the
// container).
// Init requires that all dependency containers be started (e.g. pod infra
// containers). The `recursive` parameter will, if set to true, start these
// dependency containers before initializing this container.
func (c *Container) Init(ctx context.Context, recursive bool) error {
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

	if !recursive {
		if err := c.checkDependenciesAndHandleError(); err != nil {
			return err
		}
	} else {
		if err := c.startDependencies(ctx); err != nil {
			return err
		}
	}

	if err := c.prepare(); err != nil {
		if err2 := c.cleanup(ctx); err2 != nil {
			logrus.Errorf("Cleaning up container %s: %v", c.ID(), err2)
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

// Start starts the given container.
// Start will accept container in ContainerStateConfigured,
// ContainerStateCreated, ContainerStateStopped, and ContainerStateExited, and
// transition them to ContainerStateRunning (all containers not in
// ContainerStateCreated will make an intermediate stop there via the Init API).
// Once in ContainerStateRunning, the container can be transitioned to
// ContainerStatePaused via Pause(), or to ContainerStateStopped by the process
// stopping (either due to exit, or being forced to stop by the Kill or Stop API
// calls).
// Start requites that all dependency containers (e.g. pod infra containers) be
// running before being run. The recursive parameter, if set, will start all
// dependencies before starting this container.
func (c *Container) Start(ctx context.Context, recursive bool) error {
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
// This acts as a combination of the Start and Attach APIs, ensuring proper
// ordering of the two such that no output from the container is lost (e.g. the
// Attach call occurs before Start).
// In overall functionality, it is identical to the Start call, with the added
// side effect that an attach session will also be started.
func (c *Container) StartAndAttach(ctx context.Context, streams *define.AttachStreams, keys string, resize <-chan define.TerminalSize, recursive bool) (<-chan error, error) {
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
		if err := c.attach(streams, keys, resize, true, startedChan, nil); err != nil {
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
func (c *Container) RestartWithTimeout(ctx context.Context, timeout uint) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if err := c.checkDependenciesAndHandleError(); err != nil {
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

	if !c.ensureState(define.ContainerStateCreated, define.ContainerStateRunning, define.ContainerStateStopping) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "can only stop created or running containers. %s is in state %s", c.ID(), c.state.State.String())
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

	// TODO: Is killing a paused container OK?
	switch c.state.State {
	case define.ContainerStateRunning, define.ContainerStateStopping:
		// Note that killing containers in "stopping" state is okay.
		// In that state, the Podman is waiting for the runtime to
		// stop the container and if that is taking too long, a user
		// may have decided to kill the container after all.
	default:
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

// Attach attaches to a container.
// This function returns when the attach finishes. It does not hold the lock for
// the duration of its runtime, only using it at the beginning to verify state.
func (c *Container) Attach(streams *define.AttachStreams, keys string, resize <-chan define.TerminalSize) error {
	if c.LogDriver() == define.PassthroughLogging {
		return errors.Wrapf(define.ErrNoLogs, "this container is using the 'passthrough' log driver, cannot attach")
	}
	if !c.batched {
		c.lock.Lock()
		if err := c.syncContainer(); err != nil {
			c.lock.Unlock()
			return err
		}
		// We are NOT holding the lock for the duration of the function.
		c.lock.Unlock()
	}

	if !c.ensureState(define.ContainerStateCreated, define.ContainerStateRunning) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "can only attach to created or running containers")
	}

	// HACK: This is really gross, but there isn't a better way without
	// splitting attach into separate versions for StartAndAttach and normal
	// attaching, and I really do not want to do that right now.
	// Send a SIGWINCH after attach succeeds so that most programs will
	// redraw the screen for the new attach session.
	attachRdy := make(chan bool, 1)
	if c.config.Spec.Process != nil && c.config.Spec.Process.Terminal {
		go func() {
			<-attachRdy
			if err := c.ociRuntime.KillContainer(c, uint(signal.SIGWINCH), false); err != nil {
				logrus.Warnf("Unable to send SIGWINCH to container %s after attach: %v", c.ID(), err)
			}
		}()
	}

	c.newContainerEvent(events.Attach)
	return c.attach(streams, keys, resize, false, nil, attachRdy)
}

// HTTPAttach forwards an attach session over a hijacked HTTP session.
// HTTPAttach will consume and close the included httpCon, which is expected to
// be sourced from a hijacked HTTP connection.
// The cancel channel is optional, and can be used to asynchronously cancel the
// attach session.
// The streams variable is only supported if the container was not a terminal,
// and allows specifying which of the container's standard streams will be
// forwarded to the client.
// This function returns when the attach finishes. It does not hold the lock for
// the duration of its runtime, only using it at the beginning to verify state.
// The streamLogs parameter indicates that all the container's logs until present
// will be streamed at the beginning of the attach.
// The streamAttach parameter indicates that the attach itself will be streamed
// over the socket; if this is not set, but streamLogs is, only the logs will be
// sent.
// At least one of streamAttach and streamLogs must be set.
func (c *Container) HTTPAttach(r *http.Request, w http.ResponseWriter, streams *HTTPAttachStreams, detachKeys *string, cancel <-chan bool, streamAttach, streamLogs bool, hijackDone chan<- bool) error {
	// Ensure we don't leak a goroutine if we exit before hijack completes.
	defer func() {
		close(hijackDone)
	}()

	if !c.batched {
		c.lock.Lock()
		if err := c.syncContainer(); err != nil {
			c.lock.Unlock()

			return err
		}
		// We are NOT holding the lock for the duration of the function.
		c.lock.Unlock()
	}

	if !c.ensureState(define.ContainerStateCreated, define.ContainerStateRunning) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "can only attach to created or running containers")
	}

	if !streamAttach && !streamLogs {
		return errors.Wrapf(define.ErrInvalidArg, "must specify at least one of stream or logs")
	}

	logrus.Infof("Performing HTTP Hijack attach to container %s", c.ID())

	c.newContainerEvent(events.Attach)
	return c.ociRuntime.HTTPAttach(c, r, w, streams, detachKeys, cancel, hijackDone, streamAttach, streamLogs)
}

// AttachResize resizes the container's terminal, which is displayed by Attach
// and HTTPAttach.
func (c *Container) AttachResize(newSize define.TerminalSize) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if !c.ensureState(define.ContainerStateCreated, define.ContainerStateRunning) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "can only resize created or running containers")
	}

	logrus.Infof("Resizing TTY of container %s", c.ID())

	return c.ociRuntime.AttachResize(c, newSize)
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
			execSessions, err := c.getActiveExecSessions()
			if err != nil {
				return err
			}
			if len(execSessions) != 0 {
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

	if c.state.State == define.ContainerStateRemoving {
		return errors.Wrapf(define.ErrCtrStateInvalid, "cannot mount container %s as it is being removed", c.ID())
	}

	defer c.newContainerEvent(events.Mount)
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
func (c *Container) Wait(ctx context.Context) (int32, error) {
	return c.WaitWithInterval(ctx, DefaultWaitInterval)
}

// WaitWithInterval blocks until the container to exit and returns its exit
// code. The argument is the interval at which checks the container's status.
func (c *Container) WaitWithInterval(ctx context.Context, waitTimeout time.Duration) (int32, error) {
	if !c.valid {
		return -1, define.ErrCtrRemoved
	}

	exitFile, err := c.exitFilePath()
	if err != nil {
		return -1, err
	}
	chWait := make(chan error, 1)

	go func() {
		<-ctx.Done()
		chWait <- define.ErrCanceled
	}()

	for {
		// ignore errors here (with exception of cancellation), it is only used to avoid waiting
		// too long.
		_, e := WaitForFile(exitFile, chWait, waitTimeout)
		if e == define.ErrCanceled {
			return -1, define.ErrCanceled
		}

		stopped, code, err := c.isStopped()
		if err != nil {
			return -1, err
		}
		if stopped {
			return code, nil
		}
	}
}

type waitResult struct {
	code int32
	err  error
}

func (c *Container) WaitForConditionWithInterval(ctx context.Context, waitTimeout time.Duration, conditions ...define.ContainerStatus) (int32, error) {
	if !c.valid {
		return -1, define.ErrCtrRemoved
	}

	if len(conditions) == 0 {
		panic("at least one condition should be passed")
	}

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	resultChan := make(chan waitResult)
	waitForExit := false
	wantedStates := make(map[define.ContainerStatus]bool, len(conditions))

	for _, condition := range conditions {
		if condition == define.ContainerStateStopped || condition == define.ContainerStateExited {
			waitForExit = true
			continue
		}
		wantedStates[condition] = true
	}

	trySend := func(code int32, err error) {
		select {
		case resultChan <- waitResult{code, err}:
		case <-ctx.Done():
		}
	}

	var wg sync.WaitGroup

	if waitForExit {
		wg.Add(1)
		go func() {
			defer wg.Done()

			code, err := c.WaitWithInterval(ctx, waitTimeout)
			trySend(code, err)
		}()
	}

	if len(wantedStates) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				state, err := c.State()
				if err != nil {
					trySend(-1, err)
					return
				}
				if _, found := wantedStates[state]; found {
					trySend(-1, nil)
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(waitTimeout):
					continue
				}
			}
		}()
	}

	var result waitResult
	select {
	case result = <-resultChan:
		cancelFn()
	case <-ctx.Done():
		result = waitResult{-1, define.ErrCanceled}
	}
	wg.Wait()
	return result.code, result.err
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
	if !c.ensureState(define.ContainerStateConfigured, define.ContainerStateCreated, define.ContainerStateStopped, define.ContainerStateStopping, define.ContainerStateExited) {
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

	// Check for running exec sessions
	sessions, err := c.getActiveExecSessions()
	if err != nil {
		return err
	}
	if len(sessions) > 0 {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s has active exec sessions, refusing to clean up", c.ID())
	}

	defer c.newContainerEvent(events.Cleanup)
	return c.cleanup(ctx)
}

// Batch starts a batch operation on the given container
// All commands in the passed function will execute under the same lock and
// without synchronizing state after each operation
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
	if c.ensureState(define.ContainerStateCreated, define.ContainerStateRunning, define.ContainerStatePaused, define.ContainerStateStopped, define.ContainerStateStopping) {
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

// ReloadNetwork reconfigures the container's network.
// Technically speaking, it will tear down and then reconfigure the container's
// network namespace, which will result in all firewall rules being recreated.
// It is mostly intended to be used in cases where the system firewall has been
// reloaded, and existing rules have been wiped out. It is expected that some
// downtime will result, as the rules are destroyed as part of this process.
// At present, this only works on root containers; it may be expanded to restart
// slirp4netns in the future to work with rootless containers as well.
// Requires that the container must be running or created.
func (c *Container) ReloadNetwork() error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	if !c.ensureState(define.ContainerStateCreated, define.ContainerStateRunning) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "cannot reload network unless container network has been configured")
	}

	return c.reloadNetwork()
}

// Refresh is DEPRECATED and REMOVED.
func (c *Container) Refresh(ctx context.Context) error {
	// This has been deprecated for a long while, and is in the process of
	// being removed.
	return define.ErrNotImplemented
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
	// CheckpointImageID tells the API to restore the container from
	// checkpoint image with ID set in CheckpointImageID
	CheckpointImageID string
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
	// IgnoreVolumes tells the API to not export or not to import
	// the content of volumes associated with the container
	IgnoreVolumes bool
	// Pre Checkpoint container and leave container running
	PreCheckPoint bool
	// Dump container with Pre Checkpoint images
	WithPrevious bool
	// ImportPrevious tells the API to restore container with two
	// images. One is TargetFile, the other is ImportPrevious.
	ImportPrevious string
	// CreateImage tells Podman to create an OCI image from container
	// checkpoint in the local image store.
	CreateImage string
	// Compression tells the API which compression to use for
	// the exported checkpoint archive.
	Compression archive.Compression
	// If Pod is set the container should be restored into the
	// given Pod. If Pod is empty it is a restore without a Pod.
	// Restoring a non Pod container into a Pod or a Pod container
	// without a Pod is theoretically possible, but will
	// probably not work if a PID namespace is shared.
	// A shared PID namespace means that a Pod container has PID 1
	// in the infrastructure container, but without the infrastructure
	// container no PID 1 will be in the namespace and that is not
	// possible.
	Pod string
	// PrintStats tells the API to fill out the statistics about
	// how much time each component in the stack requires to
	// checkpoint a container.
	PrintStats bool
	// FileLocks tells the API to checkpoint/restore a container
	// with file-locks
	FileLocks bool
}

// Checkpoint checkpoints a container
// The return values *define.CRIUCheckpointRestoreStatistics and int64 (time
// the runtime needs to checkpoint the container) are only set if
// options.PrintStats is set to true. Not setting options.PrintStats to true
// will return nil and 0.
func (c *Container) Checkpoint(ctx context.Context, options ContainerCheckpointOptions) (*define.CRIUCheckpointRestoreStatistics, int64, error) {
	logrus.Debugf("Trying to checkpoint container %s", c.ID())

	if options.TargetFile != "" {
		if err := c.prepareCheckpointExport(); err != nil {
			return nil, 0, err
		}
	}

	if options.WithPrevious {
		if err := c.canWithPrevious(); err != nil {
			return nil, 0, err
		}
	}

	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, 0, err
		}
	}
	return c.checkpoint(ctx, options)
}

// Restore restores a container
// The return values *define.CRIUCheckpointRestoreStatistics and int64 (time
// the runtime needs to restore the container) are only set if
// options.PrintStats is set to true. Not setting options.PrintStats to true
// will return nil and 0.
func (c *Container) Restore(ctx context.Context, options ContainerCheckpointOptions) (*define.CRIUCheckpointRestoreStatistics, int64, error) {
	if options.Pod == "" {
		logrus.Debugf("Trying to restore container %s", c.ID())
	} else {
		logrus.Debugf("Trying to restore container %s into pod %s", c.ID(), options.Pod)
	}
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, 0, err
		}
	}
	defer c.newContainerEvent(events.Restore)
	return c.restore(ctx, options)
}

// Indicate whether or not the container should restart
func (c *Container) ShouldRestart(ctx context.Context) bool {
	logrus.Debugf("Checking if container %s should restart", c.ID())
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return false
		}
	}
	return c.shouldRestart()
}

// CopyFromArchive copies the contents from the specified tarStream to path
// *inside* the container.
func (c *Container) CopyFromArchive(ctx context.Context, containerPath string, chown bool, rename map[string]string, tarStream io.Reader) (func() error, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	return c.copyFromArchive(containerPath, chown, rename, tarStream)
}

// CopyToArchive copies the contents from the specified path *inside* the
// container to the tarStream.
func (c *Container) CopyToArchive(ctx context.Context, containerPath string, tarStream io.Writer) (func() error, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	return c.copyToArchive(containerPath, tarStream)
}

// Stat the specified path *inside* the container and return a file info.
func (c *Container) Stat(ctx context.Context, containerPath string) (*define.FileInfo, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	var mountPoint string
	var err error
	if c.state.Mounted {
		mountPoint = c.state.Mountpoint
	} else {
		mountPoint, err = c.mount()
		if err != nil {
			return nil, err
		}
		defer func() {
			if err := c.unmount(false); err != nil {
				logrus.Errorf("Unmounting container %s: %v", c.ID(), err)
			}
		}()
	}

	info, _, _, err := c.stat(mountPoint, containerPath)
	return info, err
}
