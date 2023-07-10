package libpod

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/containers/common/pkg/resize"
	"github.com/containers/common/pkg/util"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/storage/pkg/stringid"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// ExecConfig contains the configuration of an exec session
type ExecConfig struct {
	// Command is the command that will be invoked in the exec session.
	// Must not be empty.
	Command []string `json:"command"`
	// Terminal is whether the exec session will allocate a pseudoterminal.
	Terminal bool `json:"terminal,omitempty"`
	// AttachStdin is whether the STDIN stream will be forwarded to the exec
	// session's first process when attaching. Only available if Terminal is
	// false.
	AttachStdin bool `json:"attachStdin,omitempty"`
	// AttachStdout is whether the STDOUT stream will be forwarded to the
	// exec session's first process when attaching. Only available if
	// Terminal is false.
	AttachStdout bool `json:"attachStdout,omitempty"`
	// AttachStderr is whether the STDERR stream will be forwarded to the
	// exec session's first process when attaching. Only available if
	// Terminal is false.
	AttachStderr bool `json:"attachStderr,omitempty"`
	// DetachKeys are keys that will be used to detach from the exec
	// session. Here, nil will use the default detach keys, where a pointer
	// to the empty string ("") will disable detaching via detach keys.
	DetachKeys *string `json:"detachKeys,omitempty"`
	// Environment is a set of environment variables that will be set for
	// the first process started by the exec session.
	Environment map[string]string `json:"environment,omitempty"`
	// Privileged is whether the exec session will be privileged - that is,
	// will be granted additional capabilities.
	Privileged bool `json:"privileged,omitempty"`
	// User is the user the exec session will be run as.
	// If set to "" the exec session will be started as the same user the
	// container was started as.
	User string `json:"user,omitempty"`
	// WorkDir is the working directory for the first process that will be
	// launched by the exec session.
	// If set to "" the exec session will be started in / within the
	// container.
	WorkDir string `json:"workDir,omitempty"`
	// PreserveFDs indicates that a number of extra FDs from the process
	// running libpod will be passed into the container. These are assumed
	// to begin at 3 (immediately after the standard streams). The number
	// given is the number that will be passed into the exec session,
	// starting at 3.
	PreserveFDs uint `json:"preserveFds,omitempty"`
	// ExitCommand is the exec session's exit command.
	// This command will be executed when the exec session exits.
	// If unset, no command will be executed.
	// Two arguments will be appended to the exit command by Libpod:
	// The ID of the exec session, and the ID of the container the exec
	// session is a part of (in that order).
	ExitCommand []string `json:"exitCommand,omitempty"`
	// ExitCommandDelay is a delay (in seconds) between the container
	// exiting, and the exit command being executed. If set to 0, there is
	// no delay. If set, ExitCommand must also be set.
	ExitCommandDelay uint `json:"exitCommandDelay,omitempty"`
}

// ExecSession contains information on a single exec session attached to a given
// container.
type ExecSession struct {
	// Id is the ID of the exec session.
	// Named somewhat strangely to not conflict with ID().
	//nolint:stylecheck,revive
	Id string `json:"id"`
	// ContainerId is the ID of the container this exec session belongs to.
	// Named somewhat strangely to not conflict with ContainerID().
	//nolint:stylecheck,revive
	ContainerId string `json:"containerId"`

	// State is the state of the exec session.
	State define.ContainerExecStatus `json:"state"`
	// PID is the PID of the process created by the exec session.
	PID int `json:"pid,omitempty"`
	// ExitCode is the exit code of the exec session, if it has exited.
	ExitCode int `json:"exitCode,omitempty"`

	// Config is the configuration of this exec session.
	// Cannot be empty.
	Config *ExecConfig `json:"config"`
}

// ID returns the ID of an exec session.
func (e *ExecSession) ID() string {
	return e.Id
}

// ContainerID returns the ID of the container this exec session was started in.
func (e *ExecSession) ContainerID() string {
	return e.ContainerId
}

// Inspect inspects the given exec session and produces detailed output on its
// configuration and current state.
func (e *ExecSession) Inspect() (*define.InspectExecSession, error) {
	if e.Config == nil {
		return nil, fmt.Errorf("given exec session does not have a configuration block: %w", define.ErrInternal)
	}

	output := new(define.InspectExecSession)
	output.CanRemove = e.State == define.ExecStateStopped
	output.ContainerID = e.ContainerId
	if e.Config.DetachKeys != nil {
		output.DetachKeys = *e.Config.DetachKeys
	}
	output.ExitCode = e.ExitCode
	output.ID = e.Id
	output.OpenStderr = e.Config.AttachStderr
	output.OpenStdin = e.Config.AttachStdin
	output.OpenStdout = e.Config.AttachStdout
	output.Running = e.State == define.ExecStateRunning
	output.Pid = e.PID
	output.ProcessConfig = new(define.InspectExecProcess)
	if len(e.Config.Command) > 0 {
		output.ProcessConfig.Entrypoint = e.Config.Command[0]
		if len(e.Config.Command) > 1 {
			output.ProcessConfig.Arguments = make([]string, 0, len(e.Config.Command)-1)
			output.ProcessConfig.Arguments = append(output.ProcessConfig.Arguments, e.Config.Command[1:]...)
		}
	}
	output.ProcessConfig.Privileged = e.Config.Privileged
	output.ProcessConfig.Tty = e.Config.Terminal
	output.ProcessConfig.User = e.Config.User

	return output, nil
}

// legacyExecSession contains information on an active exec session. It is a
// holdover from a previous Podman version and is DEPRECATED.
type legacyExecSession struct {
	ID      string   `json:"id"`
	Command []string `json:"command"`
	PID     int      `json:"pid"`
}

// ExecCreate creates a new exec session for the container.
// The session is not started. The ID of the new exec session will be returned.
func (c *Container) ExecCreate(config *ExecConfig) (string, error) {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return "", err
		}
	}

	// Verify our config
	if config == nil {
		return "", fmt.Errorf("must provide a configuration to ExecCreate: %w", define.ErrInvalidArg)
	}
	if len(config.Command) == 0 {
		return "", fmt.Errorf("must provide a non-empty command to start an exec session: %w", define.ErrInvalidArg)
	}
	if config.ExitCommandDelay > 0 && len(config.ExitCommand) == 0 {
		return "", fmt.Errorf("must provide a non-empty exit command if giving an exit command delay: %w", define.ErrInvalidArg)
	}

	// Verify that we are in a good state to continue
	if !c.ensureState(define.ContainerStateRunning) {
		return "", fmt.Errorf("can only create exec sessions on running containers: %w", define.ErrCtrStateInvalid)
	}

	// Generate an ID for our new exec session
	sessionID := stringid.GenerateRandomID()
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
			sessionID = stringid.GenerateRandomID()
		}
	}

	// Make our new exec session
	session := new(ExecSession)
	session.Id = sessionID
	session.ContainerId = c.ID()
	session.State = define.ExecStateCreated
	session.Config = new(ExecConfig)
	if err := JSONDeepCopy(config, session.Config); err != nil {
		return "", fmt.Errorf("copying exec configuration into exec session: %w", err)
	}

	if len(session.Config.ExitCommand) > 0 {
		session.Config.ExitCommand = append(session.Config.ExitCommand, []string{session.ID(), c.ID()}...)
	}

	if c.state.ExecSessions == nil {
		c.state.ExecSessions = make(map[string]*ExecSession)
	}

	// Need to add to container state and exec session registry
	c.state.ExecSessions[session.ID()] = session
	if err := c.save(); err != nil {
		return "", err
	}
	if err := c.runtime.state.AddExecSession(c, session); err != nil {
		return "", err
	}

	logrus.Infof("Created exec session %s in container %s", session.ID(), c.ID())

	return sessionID, nil
}

// ExecStart starts an exec session in the container, but does not attach to it.
// Returns immediately upon starting the exec session, unlike other ExecStart
// functions, which will only return when the exec session exits.
func (c *Container) ExecStart(sessionID string) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	// Verify that we are in a good state to continue
	if !c.ensureState(define.ContainerStateRunning) {
		return fmt.Errorf("can only start exec sessions when their container is running: %w", define.ErrCtrStateInvalid)
	}

	session, ok := c.state.ExecSessions[sessionID]
	if !ok {
		return fmt.Errorf("container %s has no exec session with ID %s: %w", c.ID(), sessionID, define.ErrNoSuchExecSession)
	}

	if session.State != define.ExecStateCreated {
		return fmt.Errorf("can only start created exec sessions, while container %s session %s state is %q: %w", c.ID(), session.ID(), session.State.String(), define.ErrExecSessionStateInvalid)
	}

	logrus.Infof("Going to start container %s exec session %s and attach to it", c.ID(), session.ID())

	opts, err := prepareForExec(c, session)
	if err != nil {
		return err
	}

	pid, err := c.ociRuntime.ExecContainerDetached(c, session.ID(), opts, session.Config.AttachStdin)
	if err != nil {
		return err
	}

	c.newContainerEvent(events.Exec)
	logrus.Debugf("Successfully started exec session %s in container %s", session.ID(), c.ID())

	// Update and save session to reflect PID/running
	session.PID = pid
	session.State = define.ExecStateRunning

	return c.save()
}

func (c *Container) ExecStartAndAttach(sessionID string, streams *define.AttachStreams, newSize *resize.TerminalSize) error {
	return c.execStartAndAttach(sessionID, streams, newSize, false)
}

// ExecStartAndAttach starts and attaches to an exec session in a container.
// newSize resizes the tty to this size before the process is started, must be nil if the exec session has no tty
func (c *Container) execStartAndAttach(sessionID string, streams *define.AttachStreams, newSize *resize.TerminalSize, isHealthcheck bool) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	// Verify that we are in a good state to continue
	if !c.ensureState(define.ContainerStateRunning) {
		return fmt.Errorf("can only start exec sessions when their container is running: %w", define.ErrCtrStateInvalid)
	}

	session, ok := c.state.ExecSessions[sessionID]
	if !ok {
		return fmt.Errorf("container %s has no exec session with ID %s: %w", c.ID(), sessionID, define.ErrNoSuchExecSession)
	}

	if session.State != define.ExecStateCreated {
		return fmt.Errorf("can only start created exec sessions, while container %s session %s state is %q: %w", c.ID(), session.ID(), session.State.String(), define.ErrExecSessionStateInvalid)
	}

	logrus.Infof("Going to start container %s exec session %s and attach to it", c.ID(), session.ID())

	opts, err := prepareForExec(c, session)
	if err != nil {
		return err
	}

	pid, attachChan, err := c.ociRuntime.ExecContainer(c, session.ID(), opts, streams, newSize)
	if err != nil {
		return err
	}

	if isHealthcheck {
		c.newContainerEvent(events.HealthStatus)
	} else {
		c.newContainerEvent(events.Exec)
	}

	logrus.Debugf("Successfully started exec session %s in container %s", session.ID(), c.ID())

	var lastErr error

	// Update and save session to reflect PID/running
	session.PID = pid
	session.State = define.ExecStateRunning

	if err := c.save(); err != nil {
		lastErr = err
	}

	// Unlock so other processes can use the container
	if !c.batched {
		c.lock.Unlock()
	}

	tmpErr := <-attachChan
	if lastErr != nil {
		logrus.Errorf("Container %s exec session %s error: %v", c.ID(), session.ID(), lastErr)
	}
	lastErr = tmpErr

	exitCode, exitCodeErr := c.readExecExitCode(session.ID())

	// Lock again.
	// Important: we must lock and sync *before* the above error is handled.
	// We need info from the database to handle the error.
	if !c.batched {
		c.lock.Lock()
	}
	// We can't reuse the old exec session (things may have changed from
	// other use, the container was unlocked).
	// So re-sync and get a fresh copy.
	// If we can't do this, no point in continuing, any attempt to save
	// would write garbage to the DB.
	if err := c.syncContainer(); err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, define.ErrCtrRemoved) {
			// We can't save status, but since the container has
			// been entirely removed, we don't have to; exit cleanly
			return lastErr
		}
		if lastErr != nil {
			logrus.Errorf("Container %s exec session %s error: %v", c.ID(), session.ID(), lastErr)
		}
		return fmt.Errorf("syncing container %s state to update exec session %s: %w", c.ID(), sessionID, err)
	}

	// Now handle the error from readExecExitCode above.
	if exitCodeErr != nil {
		newSess, ok := c.state.ExecSessions[sessionID]
		if !ok {
			// The exec session was removed entirely, probably by
			// the cleanup process. When it did so, it should have
			// written an event with the exit code.
			// Given that, there's nothing more we can do.
			logrus.Infof("Container %s exec session %s already removed", c.ID(), session.ID())
			return lastErr
		}

		if newSess.State == define.ExecStateStopped {
			// Exec session already cleaned up.
			// Exit code should be recorded, so it's OK if we were
			// not able to read it.
			logrus.Infof("Container %s exec session %s already cleaned up", c.ID(), session.ID())
			return lastErr
		}

		if lastErr != nil {
			logrus.Errorf("Container %s exec session %s error: %v", c.ID(), session.ID(), lastErr)
		}
		lastErr = exitCodeErr
	}

	logrus.Debugf("Container %s exec session %s completed with exit code %d", c.ID(), session.ID(), exitCode)

	if err := justWriteExecExitCode(c, session.ID(), exitCode); err != nil {
		if lastErr != nil {
			logrus.Errorf("Container %s exec session %s error: %v", c.ID(), session.ID(), lastErr)
		}
		lastErr = err
	}

	// Clean up after ourselves
	if err := c.cleanupExecBundle(session.ID()); err != nil {
		if lastErr != nil {
			logrus.Errorf("Container %s exec session %s error: %v", c.ID(), session.ID(), lastErr)
		}
		lastErr = err
	}

	return lastErr
}

// ExecHTTPStartAndAttach starts and performs an HTTP attach to an exec session.
// newSize resizes the tty to this size before the process is started, must be nil if the exec session has no tty
func (c *Container) ExecHTTPStartAndAttach(sessionID string, r *http.Request, w http.ResponseWriter,
	streams *HTTPAttachStreams, detachKeys *string, cancel <-chan bool, hijackDone chan<- bool, newSize *resize.TerminalSize) error {
	// TODO: How do we combine streams with the default streams set in the exec session?

	// Ensure that we don't leak a goroutine here
	defer func() {
		close(hijackDone)
	}()

	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	session, ok := c.state.ExecSessions[sessionID]
	if !ok {
		return fmt.Errorf("container %s has no exec session with ID %s: %w", c.ID(), sessionID, define.ErrNoSuchExecSession)
	}

	// Verify that we are in a good state to continue
	if !c.ensureState(define.ContainerStateRunning) {
		return fmt.Errorf("can only start exec sessions when their container is running: %w", define.ErrCtrStateInvalid)
	}

	if session.State != define.ExecStateCreated {
		return fmt.Errorf("can only start created exec sessions, while container %s session %s state is %q: %w", c.ID(), session.ID(), session.State.String(), define.ErrExecSessionStateInvalid)
	}

	logrus.Infof("Going to start container %s exec session %s and attach to it", c.ID(), session.ID())

	execOpts, err := prepareForExec(c, session)
	if err != nil {
		session.State = define.ExecStateStopped
		session.ExitCode = define.ExecErrorCodeGeneric

		if err := c.save(); err != nil {
			logrus.Errorf("Saving container %s exec session %s after failure to prepare: %v", err, c.ID(), session.ID())
		}

		return err
	}

	if streams == nil {
		streams = new(HTTPAttachStreams)
		streams.Stdin = session.Config.AttachStdin
		streams.Stdout = session.Config.AttachStdout
		streams.Stderr = session.Config.AttachStderr
	}

	holdConnOpen := make(chan bool)

	defer func() {
		close(holdConnOpen)
	}()

	pid, attachChan, err := c.ociRuntime.ExecContainerHTTP(c, session.ID(), execOpts, r, w, streams, cancel, hijackDone, holdConnOpen, newSize)
	if err != nil {
		session.State = define.ExecStateStopped
		session.ExitCode = define.TranslateExecErrorToExitCode(define.ExecErrorCodeGeneric, err)

		if err := c.save(); err != nil {
			logrus.Errorf("Saving container %s exec session %s after failure to start: %v", err, c.ID(), session.ID())
		}

		return err
	}

	// TODO: Investigate whether more of this can be made common with
	// ExecStartAndAttach

	c.newContainerEvent(events.Exec)
	logrus.Debugf("Successfully started exec session %s in container %s", session.ID(), c.ID())

	var lastErr error

	session.PID = pid
	session.State = define.ExecStateRunning

	if err := c.save(); err != nil {
		lastErr = err
	}

	// Unlock so other processes can use the container
	if !c.batched {
		c.lock.Unlock()
	}

	tmpErr := <-attachChan
	if lastErr != nil {
		logrus.Errorf("Container %s exec session %s error: %v", c.ID(), session.ID(), lastErr)
	}
	lastErr = tmpErr

	exitCode, err := c.readExecExitCode(session.ID())
	if err != nil {
		if lastErr != nil {
			logrus.Errorf("Container %s exec session %s error: %v", c.ID(), session.ID(), lastErr)
		}
		lastErr = err
	}

	logrus.Debugf("Container %s exec session %s completed with exit code %d", c.ID(), session.ID(), exitCode)

	// Lock again
	if !c.batched {
		c.lock.Lock()
	}

	if err := writeExecExitCode(c, session.ID(), exitCode); err != nil {
		if lastErr != nil {
			logrus.Errorf("Container %s exec session %s error: %v", c.ID(), session.ID(), lastErr)
		}
		lastErr = err
	}

	// Clean up after ourselves
	if err := c.cleanupExecBundle(session.ID()); err != nil {
		if lastErr != nil {
			logrus.Errorf("Container %s exec session %s error: %v", c.ID(), session.ID(), lastErr)
		}
		lastErr = err
	}

	return lastErr
}

// ExecStop stops an exec session in the container.
// If a timeout is provided, it will be used; otherwise, the timeout will
// default to the stop timeout of the container.
// Cleanup will be invoked automatically once the session is stopped.
func (c *Container) ExecStop(sessionID string, timeout *uint) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	session, ok := c.state.ExecSessions[sessionID]
	if !ok {
		return fmt.Errorf("container %s has no exec session with ID %s: %w", c.ID(), sessionID, define.ErrNoSuchExecSession)
	}

	if session.State != define.ExecStateRunning {
		return fmt.Errorf("container %s exec session %s is %q, can only stop running sessions: %w", c.ID(), session.ID(), session.State.String(), define.ErrExecSessionStateInvalid)
	}

	logrus.Infof("Stopping container %s exec session %s", c.ID(), session.ID())

	finalTimeout := c.StopTimeout()
	if timeout != nil {
		finalTimeout = *timeout
	}

	// Stop the session
	if err := c.ociRuntime.ExecStopContainer(c, session.ID(), finalTimeout); err != nil {
		return err
	}

	var cleanupErr error

	// Retrieve exit code and update status
	if err := retrieveAndWriteExecExitCode(c, session.ID()); err != nil {
		cleanupErr = err
	}

	if err := c.cleanupExecBundle(session.ID()); err != nil {
		if cleanupErr != nil {
			logrus.Errorf("Stopping container %s exec session %s: %v", c.ID(), session.ID(), cleanupErr)
		}
		cleanupErr = err
	}

	return cleanupErr
}

// ExecCleanup cleans up an exec session in the container, removing temporary
// files associated with it.
func (c *Container) ExecCleanup(sessionID string) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	session, ok := c.state.ExecSessions[sessionID]
	if !ok {
		return fmt.Errorf("container %s has no exec session with ID %s: %w", c.ID(), sessionID, define.ErrNoSuchExecSession)
	}

	if session.State == define.ExecStateRunning {
		// Check if the exec session is still running.
		alive, err := c.ociRuntime.ExecUpdateStatus(c, session.ID())
		if err != nil {
			return err
		}

		if alive {
			return fmt.Errorf("cannot clean up container %s exec session %s as it is running: %w", c.ID(), session.ID(), define.ErrExecSessionStateInvalid)
		}

		if err := retrieveAndWriteExecExitCode(c, session.ID()); err != nil {
			return err
		}
	}

	logrus.Infof("Cleaning up container %s exec session %s", c.ID(), session.ID())

	return c.cleanupExecBundle(session.ID())
}

// ExecRemove removes an exec session in the container.
// If force is given, the session will be stopped first if it is running.
func (c *Container) ExecRemove(sessionID string, force bool) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	session, ok := c.state.ExecSessions[sessionID]
	if !ok {
		return fmt.Errorf("container %s has no exec session with ID %s: %w", c.ID(), sessionID, define.ErrNoSuchExecSession)
	}

	logrus.Infof("Removing container %s exec session %s", c.ID(), session.ID())

	// Update status of exec session if running, so we can check if it
	// stopped in the meantime.
	if session.State == define.ExecStateRunning {
		running, err := c.ociRuntime.ExecUpdateStatus(c, session.ID())
		if err != nil {
			return err
		}
		if !running {
			if err := retrieveAndWriteExecExitCode(c, session.ID()); err != nil {
				return err
			}
		}
	}

	if session.State == define.ExecStateRunning {
		if !force {
			return fmt.Errorf("container %s exec session %s is still running, cannot remove: %w", c.ID(), session.ID(), define.ErrExecSessionStateInvalid)
		}

		// Stop the session
		if err := c.ociRuntime.ExecStopContainer(c, session.ID(), c.StopTimeout()); err != nil {
			return err
		}

		if err := retrieveAndWriteExecExitCode(c, session.ID()); err != nil {
			return err
		}

		if err := c.cleanupExecBundle(session.ID()); err != nil {
			return err
		}
	}

	// First remove exec session from DB.
	if err := c.runtime.state.RemoveExecSession(session); err != nil {
		return err
	}
	// Next, remove it from the container and save state
	delete(c.state.ExecSessions, sessionID)
	if err := c.save(); err != nil {
		return err
	}

	logrus.Debugf("Successfully removed container %s exec session %s", c.ID(), session.ID())

	return nil
}

// ExecResize resizes the TTY of the given exec session. Only available if the
// exec session created a TTY.
func (c *Container) ExecResize(sessionID string, newSize resize.TerminalSize) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	session, ok := c.state.ExecSessions[sessionID]
	if !ok {
		return fmt.Errorf("container %s has no exec session with ID %s: %w", c.ID(), sessionID, define.ErrNoSuchExecSession)
	}

	logrus.Infof("Resizing container %s exec session %s to %+v", c.ID(), session.ID(), newSize)

	if session.State != define.ExecStateRunning {
		return fmt.Errorf("cannot resize container %s exec session %s as it is not running: %w", c.ID(), session.ID(), define.ErrExecSessionStateInvalid)
	}

	// The exec session may have exited since we last updated.
	// Needed to prevent race conditions around short-running exec sessions.
	running, err := c.ociRuntime.ExecUpdateStatus(c, session.ID())
	if err != nil {
		return err
	}
	if !running {
		session.State = define.ExecStateStopped

		if err := c.save(); err != nil {
			logrus.Errorf("Saving state of container %s: %v", c.ID(), err)
		}

		return fmt.Errorf("cannot resize container %s exec session %s as it has stopped: %w", c.ID(), session.ID(), define.ErrExecSessionStateInvalid)
	}

	// Make sure the exec session is still running.

	return c.ociRuntime.ExecAttachResize(c, sessionID, newSize)
}

func (c *Container) Exec(config *ExecConfig, streams *define.AttachStreams, resize <-chan resize.TerminalSize) (int, error) {
	return c.exec(config, streams, resize, false)
}

// Exec emulates the old Libpod exec API, providing a single call to create,
// run, and remove an exec session. Returns exit code and error. Exit code is
// not guaranteed to be set sanely if error is not nil.
func (c *Container) exec(config *ExecConfig, streams *define.AttachStreams, resizeChan <-chan resize.TerminalSize, isHealthcheck bool) (int, error) {
	sessionID, err := c.ExecCreate(config)
	if err != nil {
		return -1, err
	}

	// Start resizing if we have a resize channel.
	// This goroutine may likely leak, given that we cannot close it here.
	// Not a big deal, since it should run for as long as the Podman process
	// does. Could be a big deal for `podman service` but we don't need this
	// API there.
	// TODO: Refactor so this is closed here, before we remove the exec
	// session.
	var size *resize.TerminalSize
	if resizeChan != nil {
		s := <-resizeChan
		size = &s
		go func() {
			logrus.Debugf("Sending resize events to exec session %s", sessionID)
			for resizeRequest := range resizeChan {
				if err := c.ExecResize(sessionID, resizeRequest); err != nil {
					if errors.Is(err, define.ErrExecSessionStateInvalid) {
						// The exec session stopped
						// before we could resize.
						logrus.Infof("Missed resize on exec session %s, already stopped", sessionID)
					} else {
						logrus.Warnf("Error resizing exec session %s: %v", sessionID, err)
					}
					return
				}
			}
		}()
	}

	if err := c.execStartAndAttach(sessionID, streams, size, isHealthcheck); err != nil {
		return -1, err
	}

	session, err := c.execSessionNoCopy(sessionID)
	if err != nil {
		if errors.Is(err, define.ErrNoSuchExecSession) {
			// TODO: If a proper Context is ever plumbed in here, we
			// should use it.
			// As things stand, though, it's not worth it - this
			// should always terminate quickly since it's not
			// streaming.
			diedEvent, err := c.runtime.GetExecDiedEvent(context.Background(), c.ID(), sessionID)
			if err != nil {
				return -1, fmt.Errorf("retrieving exec session %s exit code: %w", sessionID, err)
			}
			return diedEvent.ContainerExitCode, nil
		}
		return -1, err
	}
	exitCode := session.ExitCode
	if err := c.ExecRemove(sessionID, false); err != nil {
		if errors.Is(err, define.ErrNoSuchExecSession) {
			return exitCode, nil
		}
		return -1, err
	}

	return exitCode, nil
}

// cleanupExecBundle cleanups an exec session after its done
// Please be careful when using this function since it might temporarily unlock
// the container when os.RemoveAll($bundlePath) fails with ENOTEMPTY or EBUSY
// errors.
func (c *Container) cleanupExecBundle(sessionID string) (err error) {
	path := c.execBundlePath(sessionID)
	for attempts := 0; attempts < 50; attempts++ {
		err = os.RemoveAll(path)
		if err == nil || os.IsNotExist(err) {
			return nil
		}
		if pathErr, ok := err.(*os.PathError); ok {
			err = pathErr.Err
			if errors.Is(err, unix.ENOTEMPTY) || errors.Is(err, unix.EBUSY) {
				// give other processes a chance to use the container
				if !c.batched {
					if err := c.save(); err != nil {
						return err
					}
					c.lock.Unlock()
				}
				time.Sleep(time.Millisecond * 100)
				if !c.batched {
					c.lock.Lock()
					if err := c.syncContainer(); err != nil {
						return err
					}
				}
				continue
			}
		}
		return
	}
	return
}

// the path to a containers exec session bundle
func (c *Container) execBundlePath(sessionID string) string {
	return filepath.Join(c.bundlePath(), sessionID)
}

// Get PID file path for a container's exec session
func (c *Container) execPidPath(sessionID string) string {
	return filepath.Join(c.execBundlePath(sessionID), "exec_pid")
}

// the log path for an exec session
func (c *Container) execLogPath(sessionID string) string {
	return filepath.Join(c.execBundlePath(sessionID), "exec_log")
}

// the socket conmon creates for an exec session
func (c *Container) execAttachSocketPath(sessionID string) (string, error) {
	return c.ociRuntime.ExecAttachSocketPath(c, sessionID)
}

// execExitFileDir gets the path to the container's exit file
func (c *Container) execExitFileDir(sessionID string) string {
	return filepath.Join(c.execBundlePath(sessionID), "exit")
}

// execOCILog returns the file path for the exec sessions oci log
func (c *Container) execOCILog(sessionID string) string {
	if !c.ociRuntime.SupportsJSONErrors() {
		return ""
	}
	return filepath.Join(c.execBundlePath(sessionID), "oci-log")
}

// create a bundle path and associated files for an exec session
func (c *Container) createExecBundle(sessionID string) (retErr error) {
	bundlePath := c.execBundlePath(sessionID)
	if err := os.MkdirAll(bundlePath, execDirPermission); err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			if err := os.RemoveAll(bundlePath); err != nil {
				logrus.Warnf("Error removing exec bundle after creation caused another error: %v", err)
			}
		}
	}()
	if err := os.MkdirAll(c.execExitFileDir(sessionID), execDirPermission); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return fmt.Errorf("creating OCI runtime exit file path %s: %w", c.execExitFileDir(sessionID), err)
		}
	}
	return nil
}

// readExecExitCode reads the exit file for an exec session and returns
// the exit code
func (c *Container) readExecExitCode(sessionID string) (int, error) {
	exitFile := filepath.Join(c.execExitFileDir(sessionID), c.ID())
	chWait := make(chan error)
	defer close(chWait)

	_, err := util.WaitForFile(exitFile, chWait, time.Second*5)
	if err != nil {
		return -1, err
	}
	ec, err := os.ReadFile(exitFile)
	if err != nil {
		return -1, err
	}
	ecInt, err := strconv.Atoi(string(ec))
	if err != nil {
		return -1, err
	}
	return ecInt, nil
}

// getExecSessionPID gets the PID of an active exec session
func (c *Container) getExecSessionPID(sessionID string) (int, error) {
	session, ok := c.state.ExecSessions[sessionID]
	if ok {
		return session.PID, nil
	}
	oldSession, ok := c.state.LegacyExecSessions[sessionID]
	if ok {
		return oldSession.PID, nil
	}

	return -1, fmt.Errorf("no exec session with ID %s found in container %s: %w", sessionID, c.ID(), define.ErrNoSuchExecSession)
}

// getKnownExecSessions gets a list of all exec sessions we think are running,
// but does not verify their current state.
// Please use getActiveExecSessions() outside of container_exec.go, as this
// function performs further checks to return an accurate list.
func (c *Container) getKnownExecSessions() []string {
	knownSessions := []string{}
	// First check legacy sessions.
	// TODO: This is DEPRECATED and will be removed in a future major
	// release.
	for sessionID := range c.state.LegacyExecSessions {
		knownSessions = append(knownSessions, sessionID)
	}
	// Next check new exec sessions, but only if in running state
	for sessionID, session := range c.state.ExecSessions {
		if session.State == define.ExecStateRunning {
			knownSessions = append(knownSessions, sessionID)
		}
	}

	return knownSessions
}

// getActiveExecSessions checks if there are any active exec sessions in the
// current container. Returns an array of active exec sessions.
// Will continue through errors where possible.
// Currently handles both new and legacy, deprecated exec sessions.
func (c *Container) getActiveExecSessions() ([]string, error) {
	activeSessions := []string{}
	knownSessions := c.getKnownExecSessions()

	// Instead of saving once per iteration, do it once at the end.
	var lastErr error
	needSave := false
	for _, id := range knownSessions {
		alive, err := c.ociRuntime.ExecUpdateStatus(c, id)
		if err != nil {
			if lastErr != nil {
				logrus.Errorf("Checking container %s exec sessions: %v", c.ID(), lastErr)
			}
			lastErr = err
			continue
		}
		if !alive {
			_, isLegacy := c.state.LegacyExecSessions[id]
			if isLegacy {
				delete(c.state.LegacyExecSessions, id)
				needSave = true
			} else {
				session := c.state.ExecSessions[id]
				exitCode, err := c.readExecExitCode(session.ID())
				if err != nil {
					if lastErr != nil {
						logrus.Errorf("Checking container %s exec sessions: %v", c.ID(), lastErr)
					}
					lastErr = err
				}
				session.ExitCode = exitCode
				session.PID = 0
				session.State = define.ExecStateStopped

				c.newExecDiedEvent(session.ID(), exitCode)

				needSave = true
			}
			if err := c.cleanupExecBundle(id); err != nil {
				if lastErr != nil {
					logrus.Errorf("Checking container %s exec sessions: %v", c.ID(), lastErr)
				}
				lastErr = err
			}
		} else {
			activeSessions = append(activeSessions, id)
		}
	}
	if needSave {
		if err := c.save(); err != nil {
			if lastErr != nil {
				logrus.Errorf("Reaping exec sessions for container %s: %v", c.ID(), lastErr)
			}
			lastErr = err
		}
	}

	return activeSessions, lastErr
}

// removeAllExecSessions stops and removes all the container's exec sessions
func (c *Container) removeAllExecSessions() error {
	knownSessions := c.getKnownExecSessions()

	logrus.Debugf("Removing all exec sessions for container %s", c.ID())

	var lastErr error
	for _, id := range knownSessions {
		if err := c.ociRuntime.ExecStopContainer(c, id, c.StopTimeout()); err != nil {
			if lastErr != nil {
				logrus.Errorf("Stopping container %s exec sessions: %v", c.ID(), lastErr)
			}
			lastErr = err
			continue
		}

		if err := c.cleanupExecBundle(id); err != nil {
			if lastErr != nil {
				logrus.Errorf("Stopping container %s exec sessions: %v", c.ID(), lastErr)
			}
			lastErr = err
		}
	}
	// Delete all exec sessions
	if err := c.runtime.state.RemoveContainerExecSessions(c); err != nil {
		if !errors.Is(err, define.ErrCtrRemoved) {
			if lastErr != nil {
				logrus.Errorf("Stopping container %s exec sessions: %v", c.ID(), lastErr)
			}
			lastErr = err
		}
	}
	c.state.ExecSessions = nil
	c.state.LegacyExecSessions = nil

	return lastErr
}

// Make an ExecOptions struct to start the OCI runtime and prepare its exec
// bundle.
func prepareForExec(c *Container, session *ExecSession) (*ExecOptions, error) {
	if err := c.createExecBundle(session.ID()); err != nil {
		return nil, err
	}

	opts := new(ExecOptions)
	opts.Cmd = session.Config.Command
	opts.Env = session.Config.Environment
	opts.Terminal = session.Config.Terminal
	opts.Cwd = session.Config.WorkDir
	opts.User = session.Config.User
	opts.PreserveFDs = session.Config.PreserveFDs
	opts.DetachKeys = session.Config.DetachKeys
	opts.ExitCommand = session.Config.ExitCommand
	opts.ExitCommandDelay = session.Config.ExitCommandDelay
	opts.Privileged = session.Config.Privileged

	return opts, nil
}

// Write an exec session's exit code to the database
func writeExecExitCode(c *Container, sessionID string, exitCode int) error {
	// We can't reuse the old exec session (things may have changed from
	// under use, the container was unlocked).
	// So re-sync and get a fresh copy.
	// If we can't do this, no point in continuing, any attempt to save
	// would write garbage to the DB.
	if err := c.syncContainer(); err != nil {
		if errors.Is(err, define.ErrNoSuchCtr) || errors.Is(err, define.ErrCtrRemoved) {
			// Container's entirely removed. We can't save status,
			// but the container's entirely removed, so we don't
			// need to. Exit without error.
			return nil
		}
		return fmt.Errorf("syncing container %s state to remove exec session %s: %w", c.ID(), sessionID, err)
	}

	return justWriteExecExitCode(c, sessionID, exitCode)
}

func retrieveAndWriteExecExitCode(c *Container, sessionID string) error {
	exitCode, err := c.readExecExitCode(sessionID)
	if err != nil {
		return err
	}

	return justWriteExecExitCode(c, sessionID, exitCode)
}

func justWriteExecExitCode(c *Container, sessionID string, exitCode int) error {
	// Write an event first
	c.newExecDiedEvent(sessionID, exitCode)

	session, ok := c.state.ExecSessions[sessionID]
	if !ok {
		// Exec session already removed.
		logrus.Infof("Container %s exec session %s already removed from database", c.ID(), sessionID)
		return nil
	}

	session.State = define.ExecStateStopped
	session.ExitCode = exitCode
	session.PID = 0

	// Finally, save our changes.
	return c.save()
}
