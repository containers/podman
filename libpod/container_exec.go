package libpod

import (
	"bufio"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/containers/common/pkg/capabilities"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/storage/pkg/stringid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/remotecommand"
)

// ExecConfig contains the configuration of an exec session
type ExecConfig struct {
	// Command the the command that will be invoked in the exec session.
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
}

// ExecSession contains information on a single exec session attached to a given
// container.
type ExecSession struct {
	// Id is the ID of the exec session.
	// Named somewhat strangely to not conflict with ID().
	Id string `json:"id"`
	// ContainerId is the ID of the container this exec session belongs to.
	// Named somewhat strangely to not conflict with ContainerID().
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
		return nil, errors.Wrapf(define.ErrInternal, "given exec session does not have a configuration block")
	}

	output := new(define.InspectExecSession)
	output.CanRemove = e.State != define.ExecStateRunning
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
		return "", errors.Wrapf(define.ErrInvalidArg, "must provide a configuration to ExecCreate")
	}
	if len(config.Command) == 0 {
		return "", errors.Wrapf(define.ErrInvalidArg, "must provide a non-empty command to start an exec session")
	}

	// Verify that we are in a good state to continue
	if !c.ensureState(define.ContainerStateRunning) {
		return "", errors.Wrapf(define.ErrCtrStateInvalid, "can only create exec sessions on running containers")
	}

	// Generate an ID for our new exec session
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

	// Make our new exec session
	session := new(ExecSession)
	session.Id = sessionID
	session.ContainerId = c.ID()
	session.State = define.ExecStateCreated
	session.Config = new(ExecConfig)
	if err := JSONDeepCopy(config, session.Config); err != nil {
		return "", errors.Wrapf(err, "error copying exec configuration into exec session")
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
// Returns immediately upon starting the exec session.
func (c *Container) ExecStart(sessionID string) error {
	// Will be implemented in part 2, migrating Start and implementing
	// detached Start.
	return define.ErrNotImplemented
}

// ExecStartAndAttach starts and attaches to an exec session in a container.
// TODO: Should we include detach keys in the signature to allow override?
// TODO: How do we handle AttachStdin/AttachStdout/AttachStderr?
func (c *Container) ExecStartAndAttach(sessionID string, streams *define.AttachStreams) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	// Verify that we are in a good state to continue
	if !c.ensureState(define.ContainerStateRunning) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "can only start exec sessions when their container is running")
	}

	session, ok := c.state.ExecSessions[sessionID]
	if !ok {
		return errors.Wrapf(define.ErrNoSuchExecSession, "container %s has no exec session with ID %s", c.ID(), sessionID)
	}

	if session.State != define.ExecStateCreated {
		return errors.Wrapf(define.ErrExecSessionStateInvalid, "can only start created exec sessions, while container %s session %s state is %q", c.ID(), session.ID(), session.State.String())
	}

	logrus.Infof("Going to start container %s exec session %s and attach to it", c.ID(), session.ID())

	opts, err := prepareForExec(c, session)
	if err != nil {
		return err
	}

	pid, attachChan, err := c.ociRuntime.ExecContainer(c, session.ID(), opts, streams)
	if err != nil {
		return err
	}

	c.newContainerEvent(events.Exec)
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

// ExecHTTPStartAndAttach starts and performs an HTTP attach to an exec session.
func (c *Container) ExecHTTPStartAndAttach(sessionID string, httpCon net.Conn, httpBuf *bufio.ReadWriter, streams *HTTPAttachStreams, detachKeys *string, cancel <-chan bool) (deferredErr error) {
	// TODO: How do we combine streams with the default streams set in the exec session?

	// The flow here is somewhat strange, because we need to determine if
	// there's a terminal ASAP (for error handling).
	// Until we know, assume it's true (don't add standard stream headers).
	// Add a defer to ensure our invariant (HTTP session is closed) is
	// maintained.
	isTerminal := true
	defer func() {
		hijackWriteErrorAndClose(deferredErr, c.ID(), isTerminal, httpCon, httpBuf)
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
		return errors.Wrapf(define.ErrNoSuchExecSession, "container %s has no exec session with ID %s", c.ID(), sessionID)
	}
	// We can now finally get the real value of isTerminal.
	isTerminal = session.Config.Terminal

	// Verify that we are in a good state to continue
	if !c.ensureState(define.ContainerStateRunning) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "can only start exec sessions when their container is running")
	}

	if session.State != define.ExecStateCreated {
		return errors.Wrapf(define.ErrExecSessionStateInvalid, "can only start created exec sessions, while container %s session %s state is %q", c.ID(), session.ID(), session.State.String())
	}

	logrus.Infof("Going to start container %s exec session %s and attach to it", c.ID(), session.ID())

	execOpts, err := prepareForExec(c, session)
	if err != nil {
		return err
	}

	if streams == nil {
		streams = new(HTTPAttachStreams)
		streams.Stdin = session.Config.AttachStdin
		streams.Stdout = session.Config.AttachStdout
		streams.Stderr = session.Config.AttachStderr
	}

	pid, attachChan, err := c.ociRuntime.ExecContainerHTTP(c, session.ID(), execOpts, httpCon, httpBuf, streams, cancel)
	if err != nil {
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
		return errors.Wrapf(define.ErrNoSuchExecSession, "container %s has no exec session with ID %s", c.ID(), sessionID)
	}

	if session.State != define.ExecStateRunning {
		return errors.Wrapf(define.ErrExecSessionStateInvalid, "container %s exec session %s is %q, can only stop running sessions", c.ID(), session.ID(), session.State.String())
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
	exitCode, err := c.readExecExitCode(session.ID())
	if err != nil {
		cleanupErr = err
	}
	session.ExitCode = exitCode
	session.PID = 0
	session.State = define.ExecStateStopped

	if err := c.save(); err != nil {
		if cleanupErr != nil {
			logrus.Errorf("Error stopping container %s exec session %s: %v", c.ID(), session.ID(), cleanupErr)
		}
		cleanupErr = err
	}

	if err := c.cleanupExecBundle(session.ID()); err != nil {
		if cleanupErr != nil {
			logrus.Errorf("Error stopping container %s exec session %s: %v", c.ID(), session.ID(), cleanupErr)
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
		return errors.Wrapf(define.ErrNoSuchExecSession, "container %s has no exec session with ID %s", c.ID(), sessionID)
	}

	if session.State == define.ExecStateRunning {
		return errors.Wrapf(define.ErrExecSessionStateInvalid, "cannot clean up container %s exec session %s as it is running", c.ID(), session.ID())
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
		return errors.Wrapf(define.ErrNoSuchExecSession, "container %s has no exec session with ID %s", c.ID(), sessionID)
	}

	logrus.Infof("Removing container %s exec session %s", c.ID(), session.ID())

	// Update status of exec session if running, so we cna check if it
	// stopped in the meantime.
	if session.State == define.ExecStateRunning {
		stopped, err := c.ociRuntime.ExecUpdateStatus(c, session.ID())
		if err != nil {
			return err
		}
		if stopped {
			session.State = define.ExecStateStopped
			// TODO: should we retrieve exit code here?
			// TODO: Might be worth saving state here.
		}
	}

	if session.State == define.ExecStateRunning {
		if !force {
			return errors.Wrapf(define.ErrExecSessionStateInvalid, "container %s exec session %s is still running, cannot remove", c.ID(), session.ID())
		}

		// Stop the session
		if err := c.ociRuntime.ExecStopContainer(c, session.ID(), c.StopTimeout()); err != nil {
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
func (c *Container) ExecResize(sessionID string, newSize remotecommand.TerminalSize) error {
	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return err
		}
	}

	session, ok := c.state.ExecSessions[sessionID]
	if !ok {
		return errors.Wrapf(define.ErrNoSuchExecSession, "container %s has no exec session with ID %s", c.ID(), sessionID)
	}

	logrus.Infof("Resizing container %s exec session %s to %+v", c.ID(), session.ID(), newSize)

	if session.State != define.ExecStateRunning {
		return errors.Wrapf(define.ErrExecSessionStateInvalid, "cannot resize container %s exec session %s as it is not running", c.ID(), session.ID())
	}

	return c.ociRuntime.ExecAttachResize(c, sessionID, newSize)
}

// Exec emulates the old Libpod exec API, providing a single call to create,
// run, and remove an exec session. Returns exit code and error. Exit code is
// not guaranteed to be set sanely if error is not nil.
func (c *Container) Exec(config *ExecConfig, streams *define.AttachStreams, resize <-chan remotecommand.TerminalSize) (int, error) {
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
	if resize != nil {
		go func() {
			logrus.Debugf("Sending resize events to exec session %s", sessionID)
			for resizeRequest := range resize {
				if err := c.ExecResize(sessionID, resizeRequest); err != nil {
					// Assume the exec session went down.
					logrus.Warnf("Error resizing exec session %s: %v", sessionID, err)
					return
				}
			}
		}()
	}

	if err := c.ExecStartAndAttach(sessionID, streams); err != nil {
		return -1, err
	}

	session, err := c.ExecSession(sessionID)
	if err != nil {
		return -1, err
	}
	exitCode := session.ExitCode
	if err := c.ExecRemove(sessionID, false); err != nil {
		return -1, err
	}

	if exitCode != 0 {
		return exitCode, errors.Wrapf(define.ErrOCIRuntime, "exec session exited with non-zero exit code %d", exitCode)
	}

	return exitCode, nil
}

// cleanup an exec session after its done
func (c *Container) cleanupExecBundle(sessionID string) error {
	if err := os.RemoveAll(c.execBundlePath(sessionID)); err != nil && !os.IsNotExist(err) {
		return err
	}

	return c.ociRuntime.ExecContainerCleanup(c, sessionID)
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
func (c *Container) createExecBundle(sessionID string) (err error) {
	bundlePath := c.execBundlePath(sessionID)
	if createErr := os.MkdirAll(bundlePath, execDirPermission); createErr != nil {
		return createErr
	}
	defer func() {
		if err != nil {
			if err2 := os.RemoveAll(bundlePath); err != nil {
				logrus.Warnf("error removing exec bundle after creation caused another error: %v", err2)
			}
		}
	}()
	if err2 := os.MkdirAll(c.execExitFileDir(sessionID), execDirPermission); err2 != nil {
		// The directory is allowed to exist
		if !os.IsExist(err2) {
			err = errors.Wrapf(err2, "error creating OCI runtime exit file path %s", c.execExitFileDir(sessionID))
		}
	}
	return
}

// readExecExitCode reads the exit file for an exec session and returns
// the exit code
func (c *Container) readExecExitCode(sessionID string) (int, error) {
	exitFile := filepath.Join(c.execExitFileDir(sessionID), c.ID())
	chWait := make(chan error)
	defer close(chWait)

	_, err := WaitForFile(exitFile, chWait, time.Second*5)
	if err != nil {
		return -1, err
	}
	ec, err := ioutil.ReadFile(exitFile)
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

	return -1, errors.Wrapf(define.ErrNoSuchExecSession, "no exec session with ID %s found in container %s", sessionID, c.ID())
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
				logrus.Errorf("Error checking container %s exec sessions: %v", c.ID(), lastErr)
			}
			lastErr = err
			continue
		}
		if !alive {
			if err := c.cleanupExecBundle(id); err != nil {
				if lastErr != nil {
					logrus.Errorf("Error checking container %s exec sessions: %v", c.ID(), lastErr)
				}
				lastErr = err
			}

			_, isLegacy := c.state.LegacyExecSessions[id]
			if isLegacy {
				delete(c.state.LegacyExecSessions, id)
				needSave = true
			} else {
				session := c.state.ExecSessions[id]
				exitCode, err := c.readExecExitCode(session.ID())
				if err != nil {
					if lastErr != nil {
						logrus.Errorf("Error checking container %s exec sessions: %v", c.ID(), lastErr)
					}
					lastErr = err
				}
				session.ExitCode = exitCode
				session.PID = 0
				session.State = define.ExecStateStopped

				needSave = true
			}
		} else {
			activeSessions = append(activeSessions, id)
		}
	}
	if needSave {
		if err := c.save(); err != nil {
			if lastErr != nil {
				logrus.Errorf("Error reaping exec sessions for container %s: %v", c.ID(), lastErr)
			}
			lastErr = err
		}
	}

	return activeSessions, lastErr
}

// removeAllExecSessions stops and removes all the container's exec sessions
func (c *Container) removeAllExecSessions() error {
	knownSessions := c.getKnownExecSessions()

	var lastErr error
	for _, id := range knownSessions {
		if err := c.ociRuntime.ExecStopContainer(c, id, c.StopTimeout()); err != nil {
			if lastErr != nil {
				logrus.Errorf("Error stopping container %s exec sessions: %v", c.ID(), lastErr)
			}
			lastErr = err
			continue
		}

		if err := c.cleanupExecBundle(id); err != nil {
			if lastErr != nil {
				logrus.Errorf("Error stopping container %s exec sessions: %v", c.ID(), lastErr)
			}
			lastErr = err
		}
	}
	// Delete all exec sessions
	if err := c.runtime.state.RemoveContainerExecSessions(c); err != nil {
		if lastErr != nil {
			logrus.Errorf("Error stopping container %s exec sessions: %v", c.ID(), lastErr)
		}
		lastErr = err
	}
	c.state.ExecSessions = nil
	c.state.LegacyExecSessions = nil
	if err := c.save(); err != nil {
		if lastErr != nil {
			logrus.Errorf("Error stopping container %s exec sessions: %v", c.ID(), lastErr)
		}
		lastErr = err
	}

	return lastErr
}

// Make an ExecOptions struct to start the OCI runtime and prepare its exec
// bundle.
func prepareForExec(c *Container, session *ExecSession) (*ExecOptions, error) {
	// TODO: check logic here - should we set Privileged if the container is
	// privileged?
	var capList []string
	if session.Config.Privileged || c.config.Privileged {
		capList = capabilities.AllCapabilities()
	}

	user := c.config.User
	if session.Config.User != "" {
		user = session.Config.User
	}

	if err := c.createExecBundle(session.ID()); err != nil {
		return nil, err
	}

	opts := new(ExecOptions)
	opts.Cmd = session.Config.Command
	opts.CapAdd = capList
	opts.Env = session.Config.Environment
	opts.Terminal = session.Config.Terminal
	opts.Cwd = session.Config.WorkDir
	opts.User = user
	opts.PreserveFDs = session.Config.PreserveFDs
	opts.DetachKeys = session.Config.DetachKeys

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
		return errors.Wrapf(err, "error syncing container %s state to remove exec session %s", c.ID(), sessionID)
	}

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
