//go:build linux || freebsd
// +build linux freebsd

package libpod

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/resize"
	cutil "github.com/containers/common/pkg/util"
	conmonConfig "github.com/containers/conmon/runner/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/logs"
	"github.com/containers/podman/v4/pkg/checkpoint/crutils"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage/pkg/homedir"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	// This is Conmon's STDIO_BUF_SIZE. I don't believe we have access to it
	// directly from the Go code, so const it here
	// Important: The conmon attach socket uses an extra byte at the beginning of each
	// message to specify the STREAM so we have to increase the buffer size by one
	bufferSize = conmonConfig.BufSize + 1
)

// ConmonOCIRuntime is an OCI runtime managed by Conmon.
// TODO: Make all calls to OCI runtime have a timeout.
type ConmonOCIRuntime struct {
	name              string
	path              string
	conmonPath        string
	conmonEnv         []string
	tmpDir            string
	exitsDir          string
	logSizeMax        int64
	noPivot           bool
	reservePorts      bool
	runtimeFlags      []string
	supportsJSON      bool
	supportsKVM       bool
	supportsNoCgroups bool
	enableKeyring     bool
}

// Make a new Conmon-based OCI runtime with the given options.
// Conmon will wrap the given OCI runtime, which can be `runc`, `crun`, or
// any runtime with a runc-compatible CLI.
// The first path that points to a valid executable will be used.
// Deliberately private. Someone should not be able to construct this outside of
// libpod.
func newConmonOCIRuntime(name string, paths []string, conmonPath string, runtimeFlags []string, runtimeCfg *config.Config) (OCIRuntime, error) {
	if name == "" {
		return nil, fmt.Errorf("the OCI runtime must be provided a non-empty name: %w", define.ErrInvalidArg)
	}

	// Make lookup tables for runtime support
	supportsJSON := make(map[string]bool, len(runtimeCfg.Engine.RuntimeSupportsJSON))
	supportsNoCgroups := make(map[string]bool, len(runtimeCfg.Engine.RuntimeSupportsNoCgroups))
	supportsKVM := make(map[string]bool, len(runtimeCfg.Engine.RuntimeSupportsKVM))
	for _, r := range runtimeCfg.Engine.RuntimeSupportsJSON {
		supportsJSON[r] = true
	}
	for _, r := range runtimeCfg.Engine.RuntimeSupportsNoCgroups {
		supportsNoCgroups[r] = true
	}
	for _, r := range runtimeCfg.Engine.RuntimeSupportsKVM {
		supportsKVM[r] = true
	}

	runtime := new(ConmonOCIRuntime)
	runtime.name = name
	runtime.conmonPath = conmonPath
	runtime.runtimeFlags = runtimeFlags

	runtime.conmonEnv = runtimeCfg.Engine.ConmonEnvVars
	runtime.tmpDir = runtimeCfg.Engine.TmpDir
	runtime.logSizeMax = runtimeCfg.Containers.LogSizeMax
	runtime.noPivot = runtimeCfg.Engine.NoPivotRoot
	runtime.reservePorts = runtimeCfg.Engine.EnablePortReservation
	runtime.enableKeyring = runtimeCfg.Containers.EnableKeyring

	// TODO: probe OCI runtime for feature and enable automatically if
	// available.

	base := filepath.Base(name)
	runtime.supportsJSON = supportsJSON[base]
	runtime.supportsNoCgroups = supportsNoCgroups[base]
	runtime.supportsKVM = supportsKVM[base]

	foundPath := false
	for _, path := range paths {
		stat, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("cannot stat OCI runtime %s path: %w", name, err)
		}
		if !stat.Mode().IsRegular() {
			continue
		}
		foundPath = true
		logrus.Tracef("found runtime %q", path)
		runtime.path = path
		break
	}

	// Search the $PATH as last fallback
	if !foundPath {
		if foundRuntime, err := exec.LookPath(name); err == nil {
			foundPath = true
			runtime.path = foundRuntime
			logrus.Debugf("using runtime %q from $PATH: %q", name, foundRuntime)
		}
	}

	if !foundPath {
		return nil, fmt.Errorf("no valid executable found for OCI runtime %s: %w", name, define.ErrInvalidArg)
	}

	runtime.exitsDir = filepath.Join(runtime.tmpDir, "exits")

	// Create the exit files and attach sockets directories
	if err := os.MkdirAll(runtime.exitsDir, 0750); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return nil, fmt.Errorf("creating OCI runtime exit files directory: %w", err)
		}
	}
	return runtime, nil
}

// Name returns the name of the runtime being wrapped by Conmon.
func (r *ConmonOCIRuntime) Name() string {
	return r.name
}

// Path returns the path of the OCI runtime being wrapped by Conmon.
func (r *ConmonOCIRuntime) Path() string {
	return r.path
}

// hasCurrentUserMapped checks whether the current user is mapped inside the container user namespace
func hasCurrentUserMapped(ctr *Container) bool {
	if len(ctr.config.IDMappings.UIDMap) == 0 && len(ctr.config.IDMappings.GIDMap) == 0 {
		return true
	}
	uid := os.Geteuid()
	for _, m := range ctr.config.IDMappings.UIDMap {
		if uid >= m.HostID && uid < m.HostID+m.Size {
			return true
		}
	}
	return false
}

// CreateContainer creates a container.
func (r *ConmonOCIRuntime) CreateContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (int64, error) {
	// always make the run dir accessible to the current user so that the PID files can be read without
	// being in the rootless user namespace.
	if err := makeAccessible(ctr.state.RunDir, 0, 0); err != nil {
		return 0, err
	}
	if !hasCurrentUserMapped(ctr) {
		for _, i := range []string{ctr.state.RunDir, ctr.runtime.config.Engine.TmpDir, ctr.config.StaticDir, ctr.state.Mountpoint, ctr.runtime.config.Engine.VolumePath} {
			if err := makeAccessible(i, ctr.RootUID(), ctr.RootGID()); err != nil {
				return 0, err
			}
		}

		// if we are running a non privileged container, be sure to umount some kernel paths so they are not
		// bind mounted inside the container at all.
		if !ctr.config.Privileged && !rootless.IsRootless() {
			return r.createRootlessContainer(ctr, restoreOptions)
		}
	}
	return r.createOCIContainer(ctr, restoreOptions)
}

// UpdateContainerStatus retrieves the current status of the container from the
// runtime. It updates the container's state but does not save it.
// If useRuntime is false, we will not directly hit runc to see the container's
// status, but will instead only check for the existence of the conmon exit file
// and update state to stopped if it exists.
func (r *ConmonOCIRuntime) UpdateContainerStatus(ctr *Container) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}

	// Store old state so we know if we were already stopped
	oldState := ctr.state.State

	state := new(spec.State)

	cmd := exec.Command(r.path, "state", ctr.ID())
	cmd.Env = append(cmd.Env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir))

	outPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("getting stdout pipe: %w", err)
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("getting stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		out, err2 := io.ReadAll(errPipe)
		if err2 != nil {
			return fmt.Errorf("getting container %s state: %w", ctr.ID(), err)
		}
		if strings.Contains(string(out), "does not exist") || strings.Contains(string(out), "No such file") {
			if err := ctr.removeConmonFiles(); err != nil {
				logrus.Debugf("unable to remove conmon files for container %s", ctr.ID())
			}
			ctr.state.ExitCode = -1
			ctr.state.FinishedTime = time.Now()
			ctr.state.State = define.ContainerStateExited
			return ctr.runtime.state.AddContainerExitCode(ctr.ID(), ctr.state.ExitCode)
		}
		return fmt.Errorf("getting container %s state. stderr/out: %s: %w", ctr.ID(), out, err)
	}
	defer func() {
		_ = cmd.Wait()
	}()

	if err := errPipe.Close(); err != nil {
		return err
	}
	out, err := io.ReadAll(outPipe)
	if err != nil {
		return fmt.Errorf("reading stdout: %s: %w", ctr.ID(), err)
	}
	if err := json.NewDecoder(bytes.NewReader(out)).Decode(state); err != nil {
		return fmt.Errorf("decoding container status for container %s: %w", ctr.ID(), err)
	}
	ctr.state.PID = state.Pid

	switch state.Status {
	case "created":
		ctr.state.State = define.ContainerStateCreated
	case "paused":
		ctr.state.State = define.ContainerStatePaused
	case "running":
		ctr.state.State = define.ContainerStateRunning
	case "stopped":
		ctr.state.State = define.ContainerStateStopped
	default:
		return fmt.Errorf("unrecognized status returned by runtime for container %s: %s: %w",
			ctr.ID(), state.Status, define.ErrInternal)
	}

	// Handle ContainerStateStopping - keep it unless the container
	// transitioned to no longer running.
	if oldState == define.ContainerStateStopping && (ctr.state.State == define.ContainerStatePaused || ctr.state.State == define.ContainerStateRunning) {
		ctr.state.State = define.ContainerStateStopping
	}

	return nil
}

// StartContainer starts the given container.
// Sets time the container was started, but does not save it.
func (r *ConmonOCIRuntime) StartContainer(ctr *Container) error {
	// TODO: streams should probably *not* be our STDIN/OUT/ERR - redirect to buffers?
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	if path, ok := os.LookupEnv("PATH"); ok {
		env = append(env, fmt.Sprintf("PATH=%s", path))
	}
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, append(r.runtimeFlags, "start", ctr.ID())...); err != nil {
		return err
	}

	ctr.state.StartedTime = time.Now()

	return nil
}

// UpdateContainer updates the given container's cgroup configuration
func (r *ConmonOCIRuntime) UpdateContainer(ctr *Container, resources *spec.LinuxResources) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	if path, ok := os.LookupEnv("PATH"); ok {
		env = append(env, fmt.Sprintf("PATH=%s", path))
	}
	args := r.runtimeFlags
	args = append(args, "update")
	tempFile, additionalArgs, err := generateResourceFile(resources)
	if err != nil {
		return err
	}
	defer os.Remove(tempFile)

	args = append(args, additionalArgs...)
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, append(args, ctr.ID())...)
}

func generateResourceFile(res *spec.LinuxResources) (string, []string, error) {
	flags := []string{}
	if res == nil {
		return "", flags, nil
	}

	f, err := os.CreateTemp("", "podman")
	if err != nil {
		return "", nil, err
	}

	j, err := json.Marshal(res)
	if err != nil {
		return "", nil, err
	}
	_, err = f.Write(j)
	if err != nil {
		return "", nil, err
	}

	flags = append(flags, "--resources="+f.Name())
	return f.Name(), flags, nil
}

// KillContainer sends the given signal to the given container.
// If all is set, send to all PIDs in the container.
// All is only supported if the container created cgroups.
func (r *ConmonOCIRuntime) KillContainer(ctr *Container, signal uint, all bool) error {
	if _, err := r.killContainer(ctr, signal, all, false); err != nil {
		return err
	}

	return nil
}

// If captureStderr is requested, OCI runtime STDERR will be captured as a
// *bytes.buffer and returned; otherwise, it is set to os.Stderr.
func (r *ConmonOCIRuntime) killContainer(ctr *Container, signal uint, all, captureStderr bool) (*bytes.Buffer, error) {
	logrus.Debugf("Sending signal %d to container %s", signal, ctr.ID())
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return nil, err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	var args []string
	args = append(args, r.runtimeFlags...)
	if all {
		args = append(args, "kill", "--all", ctr.ID(), fmt.Sprintf("%d", signal))
	} else {
		args = append(args, "kill", ctr.ID(), fmt.Sprintf("%d", signal))
	}
	var (
		stderr       io.Writer = os.Stderr
		stderrBuffer *bytes.Buffer
	)
	if captureStderr {
		stderrBuffer = new(bytes.Buffer)
		stderr = stderrBuffer
	}
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, stderr, env, r.path, args...); err != nil {
		// Update container state - there's a chance we failed because
		// the container exited in the meantime.
		if err2 := r.UpdateContainerStatus(ctr); err2 != nil {
			logrus.Infof("Error updating status for container %s: %v", ctr.ID(), err2)
		}
		if ctr.ensureState(define.ContainerStateStopped, define.ContainerStateExited) {
			return stderrBuffer, fmt.Errorf("%w: %s", define.ErrCtrStateInvalid, ctr.state.State)
		}
		return stderrBuffer, fmt.Errorf("sending signal to container %s: %w", ctr.ID(), err)
	}

	return stderrBuffer, nil
}

// StopContainer stops a container, first using its given stop signal (or
// SIGTERM if no signal was specified), then using SIGKILL.
// Timeout is given in seconds. If timeout is 0, the container will be
// immediately kill with SIGKILL.
// Does not set finished time for container, assumes you will run updateStatus
// after to pull the exit code.
func (r *ConmonOCIRuntime) StopContainer(ctr *Container, timeout uint, all bool) error {
	logrus.Debugf("Stopping container %s (PID %d)", ctr.ID(), ctr.state.PID)

	// Ping the container to see if it's alive
	// If it's not, it's already stopped, return
	err := unix.Kill(ctr.state.PID, 0)
	if err == unix.ESRCH {
		return nil
	}

	killCtr := func(signal uint) (bool, error) {
		stderr, err := r.killContainer(ctr, signal, all, true)

		// Before handling error from KillContainer, convert STDERR to a []string
		// (one string per line of output) and print it, ignoring known OCI runtime
		// errors that we don't care about
		stderrLines := strings.Split(stderr.String(), "\n")
		for _, line := range stderrLines {
			if line == "" {
				continue
			}
			if strings.Contains(line, "container not running") || strings.Contains(line, "open pidfd: No such process") || strings.Contains(line, "kill container: No such process") {
				logrus.Debugf("Failure to kill container (already stopped?): logged %s", line)
				continue
			}
			fmt.Fprintf(os.Stderr, "%s\n", line)
		}

		if err != nil {
			// There's an inherent race with the cleanup process (see
			// #16142, #17142). If the container has already been marked as
			// stopped or exited by the cleanup process, we can return
			// immediately.
			if errors.Is(err, define.ErrCtrStateInvalid) && ctr.ensureState(define.ContainerStateStopped, define.ContainerStateExited) {
				return true, nil
			}

			// If the PID is 0, then the container is already stopped.
			if ctr.state.PID == 0 {
				return true, nil
			}

			// Is the container gone?
			// If so, it probably died between the first check and
			// our sending the signal
			// The container is stopped, so exit cleanly
			err := unix.Kill(ctr.state.PID, 0)
			if err == unix.ESRCH {
				return true, nil
			}

			return false, err
		}
		return false, nil
	}

	if timeout > 0 {
		stopSignal := ctr.config.StopSignal
		if stopSignal == 0 {
			stopSignal = uint(syscall.SIGTERM)
		}

		stopped, err := killCtr(stopSignal)
		if err != nil {
			return err
		}
		if stopped {
			return nil
		}

		if err := waitContainerStop(ctr, time.Duration(timeout)*time.Second); err != nil {
			logrus.Debugf("Timed out stopping container %s with %s, resorting to SIGKILL: %v", ctr.ID(), unix.SignalName(syscall.Signal(stopSignal)), err)
			logrus.Warnf("StopSignal %s failed to stop container %s in %d seconds, resorting to SIGKILL", unix.SignalName(syscall.Signal(stopSignal)), ctr.Name(), timeout)
		} else {
			// No error, the container is dead
			return nil
		}
	}

	stopped, err := killCtr(uint(unix.SIGKILL))
	if err != nil {
		return fmt.Errorf("sending SIGKILL to container %s: %w", ctr.ID(), err)
	}
	if stopped {
		return nil
	}

	// Give runtime a few seconds to make it happen
	if err := waitContainerStop(ctr, killContainerTimeout); err != nil {
		return err
	}

	return nil
}

// DeleteContainer deletes a container from the OCI runtime.
func (r *ConmonOCIRuntime) DeleteContainer(ctr *Container) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, append(r.runtimeFlags, "delete", "--force", ctr.ID())...)
}

// PauseContainer pauses the given container.
func (r *ConmonOCIRuntime) PauseContainer(ctr *Container) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, append(r.runtimeFlags, "pause", ctr.ID())...)
}

// UnpauseContainer unpauses the given container.
func (r *ConmonOCIRuntime) UnpauseContainer(ctr *Container) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, append(r.runtimeFlags, "resume", ctr.ID())...)
}

// This filters out ENOTCONN errors which can happen on FreeBSD if the
// other side of the connection is already closed.
func socketCloseWrite(conn *net.UnixConn) error {
	err := conn.CloseWrite()
	if err != nil && errors.Is(err, syscall.ENOTCONN) {
		return nil
	}
	return err
}

// HTTPAttach performs an attach for the HTTP API.
// The caller must handle closing the HTTP connection after this returns.
// The cancel channel is not closed; it is up to the caller to do so after
// this function returns.
// If this is a container with a terminal, we will stream raw. If it is not, we
// will stream with an 8-byte header to multiplex STDOUT and STDERR.
// Returns any errors that occurred, and whether the connection was successfully
// hijacked before that error occurred.
func (r *ConmonOCIRuntime) HTTPAttach(ctr *Container, req *http.Request, w http.ResponseWriter, streams *HTTPAttachStreams, detachKeys *string, cancel <-chan bool, hijackDone chan<- bool, streamAttach, streamLogs bool) (deferredErr error) {
	isTerminal := ctr.Terminal()

	if streams != nil {
		if !streams.Stdin && !streams.Stdout && !streams.Stderr {
			return fmt.Errorf("must specify at least one stream to attach to: %w", define.ErrInvalidArg)
		}
	}

	attachSock, err := r.AttachSocketPath(ctr)
	if err != nil {
		return err
	}

	var conn *net.UnixConn
	if streamAttach {
		newConn, err := openUnixSocket(attachSock)
		if err != nil {
			return fmt.Errorf("failed to connect to container's attach socket: %v: %w", attachSock, err)
		}
		conn = newConn
		defer func() {
			if err := conn.Close(); err != nil {
				logrus.Errorf("Unable to close container %s attach socket: %q", ctr.ID(), err)
			}
		}()

		logrus.Debugf("Successfully connected to container %s attach socket %s", ctr.ID(), attachSock)
	}

	detachString := ctr.runtime.config.Engine.DetachKeys
	if detachKeys != nil {
		detachString = *detachKeys
	}
	detach, err := processDetachKeys(detachString)
	if err != nil {
		return err
	}

	attachStdout := true
	attachStderr := true
	attachStdin := true
	if streams != nil {
		attachStdout = streams.Stdout
		attachStderr = streams.Stderr
		attachStdin = streams.Stdin
	}

	logrus.Debugf("Going to hijack container %s attach connection", ctr.ID())

	// Alright, let's hijack.
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return fmt.Errorf("unable to hijack connection")
	}

	httpCon, httpBuf, err := hijacker.Hijack()
	if err != nil {
		return fmt.Errorf("hijacking connection: %w", err)
	}

	hijackDone <- true

	writeHijackHeader(req, httpBuf)

	// Force a flush after the header is written.
	if err := httpBuf.Flush(); err != nil {
		return fmt.Errorf("flushing HTTP hijack header: %w", err)
	}

	defer func() {
		hijackWriteErrorAndClose(deferredErr, ctr.ID(), isTerminal, httpCon, httpBuf)
	}()

	logrus.Debugf("Hijack for container %s attach session done, ready to stream", ctr.ID())

	// TODO: This is gross. Really, really gross.
	// I want to say we should read all the logs into an array before
	// calling this, in container_api.go, but that could take a lot of
	// memory...
	// On the whole, we need to figure out a better way of doing this,
	// though.
	logSize := 0
	if streamLogs {
		logrus.Debugf("Will stream logs for container %s attach session", ctr.ID())

		// Get all logs for the container
		logChan := make(chan *logs.LogLine)
		logOpts := new(logs.LogOptions)
		logOpts.Tail = -1
		logOpts.WaitGroup = new(sync.WaitGroup)
		errChan := make(chan error)
		go func() {
			var err error
			// In non-terminal mode we need to prepend with the
			// stream header.
			logrus.Debugf("Writing logs for container %s to HTTP attach", ctr.ID())
			for logLine := range logChan {
				if !isTerminal {
					device := logLine.Device
					var header []byte
					headerLen := uint32(len(logLine.Msg))
					if !logLine.Partial() {
						// we append an extra newline in this case so we need to increment the len as well
						headerLen++
					}
					logSize += len(logLine.Msg)
					switch strings.ToLower(device) {
					case "stdin":
						header = makeHTTPAttachHeader(0, headerLen)
					case "stdout":
						header = makeHTTPAttachHeader(1, headerLen)
					case "stderr":
						header = makeHTTPAttachHeader(2, headerLen)
					default:
						logrus.Errorf("Unknown device for log line: %s", device)
						header = makeHTTPAttachHeader(1, headerLen)
					}
					_, err = httpBuf.Write(header)
					if err != nil {
						break
					}
				}
				_, err = httpBuf.Write([]byte(logLine.Msg))
				if err != nil {
					break
				}
				if !logLine.Partial() {
					_, err = httpBuf.Write([]byte("\n"))
					if err != nil {
						break
					}
				}
				err = httpBuf.Flush()
				if err != nil {
					break
				}
			}
			errChan <- err
		}()
		if err := ctr.ReadLog(context.Background(), logOpts, logChan, 0); err != nil {
			return err
		}
		go func() {
			logOpts.WaitGroup.Wait()
			close(logChan)
		}()
		logrus.Debugf("Done reading logs for container %s, %d bytes", ctr.ID(), logSize)
		if err := <-errChan; err != nil {
			return err
		}
	}
	if !streamAttach {
		logrus.Debugf("Done streaming logs for container %s attach, exiting as attach streaming not requested", ctr.ID())
		return nil
	}

	logrus.Debugf("Forwarding attach output for container %s", ctr.ID())

	stdoutChan := make(chan error)
	stdinChan := make(chan error)

	// Handle STDOUT/STDERR
	go func() {
		var err error
		if isTerminal {
			// Hack: return immediately if attachStdout not set to
			// emulate Docker.
			// Basically, when terminal is set, STDERR goes nowhere.
			// Everything does over STDOUT.
			// Therefore, if not attaching STDOUT - we'll never copy
			// anything from here.
			logrus.Debugf("Performing terminal HTTP attach for container %s", ctr.ID())
			if attachStdout {
				err = httpAttachTerminalCopy(conn, httpBuf, ctr.ID())
			}
		} else {
			logrus.Debugf("Performing non-terminal HTTP attach for container %s", ctr.ID())
			err = httpAttachNonTerminalCopy(conn, httpBuf, ctr.ID(), attachStdin, attachStdout, attachStderr)
		}
		stdoutChan <- err
		logrus.Debugf("STDOUT/ERR copy completed")
	}()
	// Next, STDIN. Avoid entirely if attachStdin unset.
	if attachStdin {
		go func() {
			_, err := cutil.CopyDetachable(conn, httpBuf, detach)
			logrus.Debugf("STDIN copy completed")
			stdinChan <- err
		}()
	}

	for {
		select {
		case err := <-stdoutChan:
			if err != nil {
				return err
			}

			return nil
		case err := <-stdinChan:
			if err != nil {
				return err
			}
			// copy stdin is done, close it
			if connErr := socketCloseWrite(conn); connErr != nil {
				logrus.Errorf("Unable to close conn: %v", connErr)
			}
		case <-cancel:
			return nil
		}
	}
}

// isRetryable returns whether the error was caused by a blocked syscall or the
// specified operation on a non blocking file descriptor wasn't ready for completion.
func isRetryable(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EINTR || errno == syscall.EAGAIN
	}
	return false
}

// openControlFile opens the terminal control file.
func openControlFile(ctr *Container, parentDir string) (*os.File, error) {
	controlPath := filepath.Join(parentDir, "ctl")
	for i := 0; i < 600; i++ {
		controlFile, err := os.OpenFile(controlPath, unix.O_WRONLY|unix.O_NONBLOCK, 0)
		if err == nil {
			return controlFile, nil
		}
		if !isRetryable(err) {
			return nil, fmt.Errorf("could not open ctl file for terminal resize for container %s: %w", ctr.ID(), err)
		}
		time.Sleep(time.Second / 10)
	}
	return nil, fmt.Errorf("timeout waiting for %q", controlPath)
}

// AttachResize resizes the terminal used by the given container.
func (r *ConmonOCIRuntime) AttachResize(ctr *Container, newSize resize.TerminalSize) error {
	controlFile, err := openControlFile(ctr, ctr.bundlePath())
	if err != nil {
		return err
	}
	defer controlFile.Close()

	logrus.Debugf("Received a resize event for container %s: %+v", ctr.ID(), newSize)
	if _, err = fmt.Fprintf(controlFile, "%d %d %d\n", 1, newSize.Height, newSize.Width); err != nil {
		return fmt.Errorf("failed to write to ctl file to resize terminal: %w", err)
	}

	return nil
}

// CheckpointContainer checkpoints the given container.
func (r *ConmonOCIRuntime) CheckpointContainer(ctr *Container, options ContainerCheckpointOptions) (int64, error) {
	// imagePath is used by CRIU to store the actual checkpoint files
	imagePath := ctr.CheckpointPath()
	if options.PreCheckPoint {
		imagePath = ctr.PreCheckPointPath()
	}
	// workPath will be used to store dump.log and stats-dump
	workPath := ctr.bundlePath()
	logrus.Debugf("Writing checkpoint to %s", imagePath)
	logrus.Debugf("Writing checkpoint logs to %s", workPath)
	logrus.Debugf("Pre-dump the container %t", options.PreCheckPoint)
	args := []string{}
	args = append(args, r.runtimeFlags...)
	args = append(args, "checkpoint")
	args = append(args, "--image-path")
	args = append(args, imagePath)
	args = append(args, "--work-path")
	args = append(args, workPath)
	if options.KeepRunning {
		args = append(args, "--leave-running")
	}
	if options.TCPEstablished {
		args = append(args, "--tcp-established")
	}
	if options.FileLocks {
		args = append(args, "--file-locks")
	}
	if !options.PreCheckPoint && options.KeepRunning {
		args = append(args, "--leave-running")
	}
	if options.PreCheckPoint {
		args = append(args, "--pre-dump")
	}
	if !options.PreCheckPoint && options.WithPrevious {
		args = append(
			args,
			"--parent-path",
			filepath.Join("..", preCheckpointDir),
		)
	}

	args = append(args, ctr.ID())
	logrus.Debugf("the args to checkpoint: %s %s", r.path, strings.Join(args, " "))

	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return 0, err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	if path, ok := os.LookupEnv("PATH"); ok {
		env = append(env, fmt.Sprintf("PATH=%s", path))
	}

	var runtimeCheckpointStarted time.Time
	err = r.withContainerSocketLabel(ctr, func() error {
		runtimeCheckpointStarted = time.Now()
		return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, args...)
	})

	runtimeCheckpointDuration := func() int64 {
		if options.PrintStats {
			return time.Since(runtimeCheckpointStarted).Microseconds()
		}
		return 0
	}()

	return runtimeCheckpointDuration, err
}

func (r *ConmonOCIRuntime) CheckConmonRunning(ctr *Container) (bool, error) {
	if ctr.state.ConmonPID == 0 {
		// If the container is running or paused, assume Conmon is
		// running. We didn't record Conmon PID on some old versions, so
		// that is likely what's going on...
		// Unusual enough that we should print a warning message though.
		if ctr.ensureState(define.ContainerStateRunning, define.ContainerStatePaused) {
			logrus.Warnf("Conmon PID is not set, but container is running!")
			return true, nil
		}
		// Container's not running, so conmon PID being unset is
		// expected. Conmon is not running.
		return false, nil
	}

	// We have a conmon PID. Ping it with signal 0.
	if err := unix.Kill(ctr.state.ConmonPID, 0); err != nil {
		if err == unix.ESRCH {
			return false, nil
		}
		return false, fmt.Errorf("pinging container %s conmon with signal 0: %w", ctr.ID(), err)
	}
	return true, nil
}

// SupportsCheckpoint checks if the OCI runtime supports checkpointing
// containers.
func (r *ConmonOCIRuntime) SupportsCheckpoint() bool {
	return crutils.CRRuntimeSupportsCheckpointRestore(r.path)
}

// SupportsJSONErrors checks if the OCI runtime supports JSON-formatted error
// messages.
func (r *ConmonOCIRuntime) SupportsJSONErrors() bool {
	return r.supportsJSON
}

// SupportsNoCgroups checks if the OCI runtime supports running containers
// without cgroups (the --cgroup-manager=disabled flag).
func (r *ConmonOCIRuntime) SupportsNoCgroups() bool {
	return r.supportsNoCgroups
}

// SupportsKVM checks if the OCI runtime supports running containers
// without KVM separation
func (r *ConmonOCIRuntime) SupportsKVM() bool {
	return r.supportsKVM
}

// AttachSocketPath is the path to a single container's attach socket.
func (r *ConmonOCIRuntime) AttachSocketPath(ctr *Container) (string, error) {
	if ctr == nil {
		return "", fmt.Errorf("must provide a valid container to get attach socket path: %w", define.ErrInvalidArg)
	}

	return filepath.Join(ctr.bundlePath(), "attach"), nil
}

// ExitFilePath is the path to a container's exit file.
func (r *ConmonOCIRuntime) ExitFilePath(ctr *Container) (string, error) {
	if ctr == nil {
		return "", fmt.Errorf("must provide a valid container to get exit file path: %w", define.ErrInvalidArg)
	}
	return filepath.Join(r.exitsDir, ctr.ID()), nil
}

// RuntimeInfo provides information on the runtime.
func (r *ConmonOCIRuntime) RuntimeInfo() (*define.ConmonInfo, *define.OCIRuntimeInfo, error) {
	runtimePackage := cutil.PackageVersion(r.path)
	conmonPackage := cutil.PackageVersion(r.conmonPath)
	runtimeVersion, err := r.getOCIRuntimeVersion()
	if err != nil {
		return nil, nil, fmt.Errorf("getting version of OCI runtime %s: %w", r.name, err)
	}
	conmonVersion, err := r.getConmonVersion()
	if err != nil {
		return nil, nil, fmt.Errorf("getting conmon version: %w", err)
	}

	conmon := define.ConmonInfo{
		Package: conmonPackage,
		Path:    r.conmonPath,
		Version: conmonVersion,
	}
	ocirt := define.OCIRuntimeInfo{
		Name:    r.name,
		Path:    r.path,
		Package: runtimePackage,
		Version: runtimeVersion,
	}
	return &conmon, &ocirt, nil
}

// makeAccessible changes the path permission and each parent directory to have --x--x--x
func makeAccessible(path string, uid, gid int) error {
	for ; path != "/"; path = filepath.Dir(path) {
		st, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if int(st.Sys().(*syscall.Stat_t).Uid) == uid && int(st.Sys().(*syscall.Stat_t).Gid) == gid {
			continue
		}
		if st.Mode()&0111 != 0111 {
			if err := os.Chmod(path, st.Mode()|0111); err != nil {
				return err
			}
		}
	}
	return nil
}

// Wait for a container which has been sent a signal to stop
func waitContainerStop(ctr *Container, timeout time.Duration) error {
	return waitPidStop(ctr.state.PID, timeout)
}

// Wait for a given PID to stop
func waitPidStop(pid int, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	for {
		select {
		case <-timer.C:
			return fmt.Errorf("given PID did not die within timeout")
		default:
			if err := unix.Kill(pid, 0); err != nil {
				if err == unix.ESRCH {
					return nil
				}
				logrus.Errorf("Pinging PID %d with signal 0: %v", pid, err)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (r *ConmonOCIRuntime) getLogTag(ctr *Container) (string, error) {
	logTag := ctr.LogTag()
	if logTag == "" {
		return "", nil
	}
	data, err := ctr.inspectLocked(false)
	if err != nil {
		// FIXME: this error should probably be returned
		return "", nil //nolint: nilerr
	}
	tmpl, err := template.New("container").Parse(logTag)
	if err != nil {
		return "", fmt.Errorf("template parsing error %s: %w", logTag, err)
	}
	var b bytes.Buffer
	err = tmpl.Execute(&b, data)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

// createOCIContainer generates this container's main conmon instance and prepares it for starting
func (r *ConmonOCIRuntime) createOCIContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (int64, error) {
	var stderrBuf bytes.Buffer

	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return 0, err
	}

	parentSyncPipe, childSyncPipe, err := newPipe()
	if err != nil {
		return 0, fmt.Errorf("creating socket pair: %w", err)
	}
	defer errorhandling.CloseQuiet(parentSyncPipe)

	childStartPipe, parentStartPipe, err := newPipe()
	if err != nil {
		return 0, fmt.Errorf("creating socket pair for start pipe: %w", err)
	}

	defer errorhandling.CloseQuiet(parentStartPipe)

	var ociLog string
	if logrus.GetLevel() != logrus.DebugLevel && r.supportsJSON {
		ociLog = filepath.Join(ctr.state.RunDir, "oci-log")
	}

	logTag, err := r.getLogTag(ctr)
	if err != nil {
		return 0, err
	}

	if ctr.config.CgroupsMode == cgroupSplit {
		if err := utils.MoveUnderCgroupSubtree("runtime"); err != nil {
			return 0, err
		}
	}

	pidfile := ctr.config.PidFile
	if pidfile == "" {
		pidfile = filepath.Join(ctr.state.RunDir, "pidfile")
	}

	args := r.sharedConmonArgs(ctr, ctr.ID(), ctr.bundlePath(), pidfile, ctr.LogPath(), r.exitsDir, ociLog, ctr.LogDriver(), logTag)

	if ctr.config.SdNotifyMode == define.SdNotifyModeContainer && ctr.config.SdNotifySocket != "" {
		args = append(args, fmt.Sprintf("--sdnotify-socket=%s", ctr.config.SdNotifySocket))
	}

	if ctr.Terminal() {
		args = append(args, "-t")
	} else if ctr.config.Stdin {
		args = append(args, "-i")
	}

	if ctr.config.Timeout > 0 {
		args = append(args, fmt.Sprintf("--timeout=%d", ctr.config.Timeout))
	}

	if !r.enableKeyring {
		args = append(args, "--no-new-keyring")
	}
	if ctr.config.ConmonPidFile != "" {
		args = append(args, "--conmon-pidfile", ctr.config.ConmonPidFile)
	}

	if r.noPivot {
		args = append(args, "--no-pivot")
	}

	exitCommand, err := specgenutil.CreateExitCommandArgs(ctr.runtime.storageConfig, ctr.runtime.config, logrus.IsLevelEnabled(logrus.DebugLevel), ctr.AutoRemove(), false)
	if err != nil {
		return 0, err
	}
	exitCommand = append(exitCommand, ctr.config.ID)

	args = append(args, "--exit-command", exitCommand[0])
	for _, arg := range exitCommand[1:] {
		args = append(args, []string{"--exit-command-arg", arg}...)
	}

	// Pass down the LISTEN_* environment (see #10443).
	preserveFDs := ctr.config.PreserveFDs
	if val := os.Getenv("LISTEN_FDS"); val != "" {
		if ctr.config.PreserveFDs > 0 {
			logrus.Warnf("Ignoring LISTEN_FDS to preserve custom user-specified FDs")
		} else {
			fds, err := strconv.Atoi(val)
			if err != nil {
				return 0, fmt.Errorf("converting LISTEN_FDS=%s: %w", val, err)
			}
			preserveFDs = uint(fds)
		}
	}

	if preserveFDs > 0 {
		args = append(args, formatRuntimeOpts("--preserve-fds", fmt.Sprintf("%d", preserveFDs))...)
	}

	if restoreOptions != nil {
		args = append(args, "--restore", ctr.CheckpointPath())
		if restoreOptions.TCPEstablished {
			args = append(args, "--runtime-opt", "--tcp-established")
		}
		if restoreOptions.FileLocks {
			args = append(args, "--runtime-opt", "--file-locks")
		}
		if restoreOptions.Pod != "" {
			mountLabel := ctr.config.MountLabel
			processLabel := ctr.config.ProcessLabel
			if mountLabel != "" {
				args = append(
					args,
					"--runtime-opt",
					fmt.Sprintf(
						"--lsm-mount-context=%s",
						mountLabel,
					),
				)
			}
			if processLabel != "" {
				args = append(
					args,
					"--runtime-opt",
					fmt.Sprintf(
						"--lsm-profile=selinux:%s",
						processLabel,
					),
				)
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"args": args,
	}).Debugf("running conmon: %s", r.conmonPath)

	cmd := exec.Command(r.conmonPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	// TODO this is probably a really bad idea for some uses
	// Make this configurable
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if ctr.Terminal() {
		cmd.Stderr = &stderrBuf
	}

	// 0, 1 and 2 are stdin, stdout and stderr
	conmonEnv := r.configureConmonEnv(runtimeDir)

	var filesToClose []*os.File
	if preserveFDs > 0 {
		for fd := 3; fd < int(3+preserveFDs); fd++ {
			f := os.NewFile(uintptr(fd), fmt.Sprintf("fd-%d", fd))
			filesToClose = append(filesToClose, f)
			cmd.ExtraFiles = append(cmd.ExtraFiles, f)
		}
	}

	cmd.Env = r.conmonEnv
	// we don't want to step on users fds they asked to preserve
	// Since 0-2 are used for stdio, start the fds we pass in at preserveFDs+3
	cmd.Env = append(cmd.Env, fmt.Sprintf("_OCI_SYNCPIPE=%d", preserveFDs+3), fmt.Sprintf("_OCI_STARTPIPE=%d", preserveFDs+4))
	cmd.Env = append(cmd.Env, conmonEnv...)
	cmd.ExtraFiles = append(cmd.ExtraFiles, childSyncPipe, childStartPipe)

	if r.reservePorts && !rootless.IsRootless() && !ctr.config.NetMode.IsSlirp4netns() {
		ports, err := bindPorts(ctr.convertPortMappings())
		if err != nil {
			return 0, err
		}
		filesToClose = append(filesToClose, ports...)

		// Leak the port we bound in the conmon process.  These fd's won't be used
		// by the container and conmon will keep the ports busy so that another
		// process cannot use them.
		cmd.ExtraFiles = append(cmd.ExtraFiles, ports...)
	}

	if ctr.config.NetMode.IsSlirp4netns() || rootless.IsRootless() {
		if ctr.config.PostConfigureNetNS {
			havePortMapping := len(ctr.config.PortMappings) > 0
			if havePortMapping {
				ctr.rootlessPortSyncR, ctr.rootlessPortSyncW, err = os.Pipe()
				if err != nil {
					return 0, fmt.Errorf("failed to create rootless port sync pipe: %w", err)
				}
			}
			ctr.rootlessSlirpSyncR, ctr.rootlessSlirpSyncW, err = os.Pipe()
			if err != nil {
				return 0, fmt.Errorf("failed to create rootless network sync pipe: %w", err)
			}
		}

		if ctr.rootlessSlirpSyncW != nil {
			defer errorhandling.CloseQuiet(ctr.rootlessSlirpSyncW)
			// Leak one end in conmon, the other one will be leaked into slirp4netns
			cmd.ExtraFiles = append(cmd.ExtraFiles, ctr.rootlessSlirpSyncW)
		}

		if ctr.rootlessPortSyncW != nil {
			defer errorhandling.CloseQuiet(ctr.rootlessPortSyncW)
			// Leak one end in conmon, the other one will be leaked into rootlessport
			cmd.ExtraFiles = append(cmd.ExtraFiles, ctr.rootlessPortSyncW)
		}
	}
	var runtimeRestoreStarted time.Time
	if restoreOptions != nil {
		runtimeRestoreStarted = time.Now()
	}
	err = startCommand(cmd, ctr)

	// regardless of whether we errored or not, we no longer need the children pipes
	childSyncPipe.Close()
	childStartPipe.Close()
	if err != nil {
		return 0, err
	}
	if err := r.moveConmonToCgroupAndSignal(ctr, cmd, parentStartPipe); err != nil {
		return 0, err
	}
	/* Wait for initial setup and fork, and reap child */
	err = cmd.Wait()
	if err != nil {
		return 0, err
	}

	pid, err := readConmonPipeData(r.name, parentSyncPipe, ociLog)
	if err != nil {
		if err2 := r.DeleteContainer(ctr); err2 != nil {
			logrus.Errorf("Removing container %s from runtime after creation failed", ctr.ID())
		}
		return 0, err
	}
	ctr.state.PID = pid

	conmonPID, err := readConmonPidFile(ctr.config.ConmonPidFile)
	if err != nil {
		logrus.Warnf("Error reading conmon pid file for container %s: %v", ctr.ID(), err)
	} else if conmonPID > 0 {
		// conmon not having a pid file is a valid state, so don't set it if we don't have it
		logrus.Infof("Got Conmon PID as %d", conmonPID)
		ctr.state.ConmonPID = conmonPID
	}

	runtimeRestoreDuration := func() int64 {
		if restoreOptions != nil && restoreOptions.PrintStats {
			return time.Since(runtimeRestoreStarted).Microseconds()
		}
		return 0
	}()

	// These fds were passed down to the runtime.  Close them
	// and not interfere
	for _, f := range filesToClose {
		errorhandling.CloseQuiet(f)
	}

	return runtimeRestoreDuration, nil
}

// configureConmonEnv gets the environment values to add to conmon's exec struct
// TODO this may want to be less hardcoded/more configurable in the future
func (r *ConmonOCIRuntime) configureConmonEnv(runtimeDir string) []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "LC_") {
			env = append(env, e)
		}
	}
	if path, ok := os.LookupEnv("PATH"); ok {
		env = append(env, fmt.Sprintf("PATH=%s", path))
	}
	if conf, ok := os.LookupEnv("CONTAINERS_CONF"); ok {
		env = append(env, fmt.Sprintf("CONTAINERS_CONF=%s", conf))
	}
	if conf, ok := os.LookupEnv("CONTAINERS_HELPER_BINARY_DIR"); ok {
		env = append(env, fmt.Sprintf("CONTAINERS_HELPER_BINARY_DIR=%s", conf))
	}
	env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir))
	env = append(env, fmt.Sprintf("_CONTAINERS_USERNS_CONFIGURED=%s", os.Getenv("_CONTAINERS_USERNS_CONFIGURED")))
	env = append(env, fmt.Sprintf("_CONTAINERS_ROOTLESS_UID=%s", os.Getenv("_CONTAINERS_ROOTLESS_UID")))
	home := homedir.Get()
	if home != "" {
		env = append(env, fmt.Sprintf("HOME=%s", home))
	}

	return env
}

// sharedConmonArgs takes common arguments for exec and create/restore and formats them for the conmon CLI
func (r *ConmonOCIRuntime) sharedConmonArgs(ctr *Container, cuuid, bundlePath, pidPath, logPath, exitDir, ociLogPath, logDriver, logTag string) []string {
	// set the conmon API version to be able to use the correct sync struct keys
	args := []string{
		"--api-version", "1",
		"-c", ctr.ID(),
		"-u", cuuid,
		"-r", r.path,
		"-b", bundlePath,
		"-p", pidPath,
		"-n", ctr.Name(),
		"--exit-dir", exitDir,
		"--full-attach",
	}
	if len(r.runtimeFlags) > 0 {
		rFlags := []string{}
		for _, arg := range r.runtimeFlags {
			rFlags = append(rFlags, "--runtime-arg", arg)
		}
		args = append(args, rFlags...)
	}

	if ctr.CgroupManager() == config.SystemdCgroupsManager && !ctr.config.NoCgroups && ctr.config.CgroupsMode != cgroupSplit {
		args = append(args, "-s")
	}

	var logDriverArg string
	switch logDriver {
	case define.JournaldLogging:
		logDriverArg = define.JournaldLogging
	case define.NoLogging:
		logDriverArg = define.NoLogging
	case define.PassthroughLogging:
		logDriverArg = define.PassthroughLogging
	//lint:ignore ST1015 the default case has to be here
	default: //nolint:gocritic
		// No case here should happen except JSONLogging, but keep this here in case the options are extended
		logrus.Errorf("%s logging specified but not supported. Choosing k8s-file logging instead", ctr.LogDriver())
		fallthrough
	case "":
		// to get here, either a user would specify `--log-driver ""`, or this came from another place in libpod
		// since the former case is obscure, and the latter case isn't an error, let's silently fallthrough
		fallthrough
	case define.JSONLogging:
		fallthrough
	case define.KubernetesLogging:
		logDriverArg = fmt.Sprintf("%s:%s", define.KubernetesLogging, logPath)
	}

	args = append(args, "-l", logDriverArg)
	logLevel := logrus.GetLevel()
	args = append(args, "--log-level", logLevel.String())

	logrus.Debugf("%s messages will be logged to syslog", r.conmonPath)
	args = append(args, "--syslog")

	size := r.logSizeMax
	if ctr.config.LogSize > 0 {
		size = ctr.config.LogSize
	}
	if size > 0 {
		args = append(args, "--log-size-max", fmt.Sprintf("%v", size))
	}

	if ociLogPath != "" {
		args = append(args, "--runtime-arg", "--log-format=json", "--runtime-arg", "--log", fmt.Sprintf("--runtime-arg=%s", ociLogPath))
	}
	if logTag != "" {
		args = append(args, "--log-tag", logTag)
	}
	if ctr.config.NoCgroups {
		logrus.Debugf("Running with no Cgroups")
		args = append(args, "--runtime-arg", "--cgroup-manager", "--runtime-arg", "disabled")
	}
	return args
}

func startCommand(cmd *exec.Cmd, ctr *Container) error {
	// Make sure to unset the NOTIFY_SOCKET and reset it afterwards if needed.
	switch ctr.config.SdNotifyMode {
	case define.SdNotifyModeContainer, define.SdNotifyModeIgnore:
		if prev := os.Getenv("NOTIFY_SOCKET"); prev != "" {
			if err := os.Unsetenv("NOTIFY_SOCKET"); err != nil {
				logrus.Warnf("Error unsetting NOTIFY_SOCKET %v", err)
			}
			defer func() {
				if err := os.Setenv("NOTIFY_SOCKET", prev); err != nil {
					logrus.Errorf("Resetting NOTIFY_SOCKET=%s", prev)
				}
			}()
		}
	}

	return cmd.Start()
}

// newPipe creates a unix socket pair for communication.
// Returns two files - first is parent, second is child.
func newPipe() (*os.File, *os.File, error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), "parent"), os.NewFile(uintptr(fds[0]), "child"), nil
}

// readConmonPidFile attempts to read conmon's pid from its pid file
func readConmonPidFile(pidFile string) (int, error) {
	// Let's try reading the Conmon pid at the same time.
	if pidFile != "" {
		contents, err := os.ReadFile(pidFile)
		if err != nil {
			return -1, err
		}
		// Convert it to an int
		conmonPID, err := strconv.Atoi(string(contents))
		if err != nil {
			return -1, err
		}
		return conmonPID, nil
	}
	return 0, nil
}

// readConmonPipeData attempts to read a syncInfo struct from the pipe
func readConmonPipeData(runtimeName string, pipe *os.File, ociLog string) (int, error) {
	// syncInfo is used to return data from monitor process to daemon
	type syncInfo struct {
		Data    int    `json:"data"`
		Message string `json:"message,omitempty"`
	}

	// Wait to get container pid from conmon
	type syncStruct struct {
		si  *syncInfo
		err error
	}
	ch := make(chan syncStruct)
	go func() {
		var si *syncInfo
		rdr := bufio.NewReader(pipe)
		b, err := rdr.ReadBytes('\n')
		// ignore EOF here, error is returned even when data was read
		// if it is no valid json unmarshal will fail below
		if err != nil && !errors.Is(err, io.EOF) {
			ch <- syncStruct{err: err}
		}
		if err := json.Unmarshal(b, &si); err != nil {
			ch <- syncStruct{err: fmt.Errorf("conmon bytes %q: %w", string(b), err)}
			return
		}
		ch <- syncStruct{si: si}
	}()

	var data int
	select {
	case ss := <-ch:
		if ss.err != nil {
			if ociLog != "" {
				ociLogData, err := os.ReadFile(ociLog)
				if err == nil {
					var ociErr ociError
					if err := json.Unmarshal(ociLogData, &ociErr); err == nil {
						return -1, getOCIRuntimeError(runtimeName, ociErr.Msg)
					}
				}
			}
			return -1, fmt.Errorf("container create failed (no logs from conmon): %w", ss.err)
		}
		logrus.Debugf("Received: %d", ss.si.Data)
		if ss.si.Data < 0 {
			if ociLog != "" {
				ociLogData, err := os.ReadFile(ociLog)
				if err == nil {
					var ociErr ociError
					if err := json.Unmarshal(ociLogData, &ociErr); err == nil {
						return ss.si.Data, getOCIRuntimeError(runtimeName, ociErr.Msg)
					}
				}
			}
			// If we failed to parse the JSON errors, then print the output as it is
			if ss.si.Message != "" {
				return ss.si.Data, getOCIRuntimeError(runtimeName, ss.si.Message)
			}
			return ss.si.Data, fmt.Errorf("container create failed: %w", define.ErrInternal)
		}
		data = ss.si.Data
	case <-time.After(define.ContainerCreateTimeout):
		return -1, fmt.Errorf("container creation timeout: %w", define.ErrInternal)
	}
	return data, nil
}

// writeConmonPipeData writes nonce data to a pipe
func writeConmonPipeData(pipe *os.File) error {
	someData := []byte{0}
	_, err := pipe.Write(someData)
	return err
}

// formatRuntimeOpts prepends opts passed to it with --runtime-opt for passing to conmon
func formatRuntimeOpts(opts ...string) []string {
	args := make([]string, 0, len(opts)*2)
	for _, o := range opts {
		args = append(args, "--runtime-opt", o)
	}
	return args
}

// getConmonVersion returns a string representation of the conmon version.
func (r *ConmonOCIRuntime) getConmonVersion() (string, error) {
	output, err := utils.ExecCmd(r.conmonPath, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.Replace(output, "\n", ", ", 1), "\n"), nil
}

// getOCIRuntimeVersion returns a string representation of the OCI runtime's
// version.
func (r *ConmonOCIRuntime) getOCIRuntimeVersion() (string, error) {
	output, err := utils.ExecCmd(r.path, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(output, "\n"), nil
}

// Copy data from container to HTTP connection, for terminal attach.
// Container is the container's attach socket connection, http is a buffer for
// the HTTP connection. cid is the ID of the container the attach session is
// running for (used solely for error messages).
func httpAttachTerminalCopy(container *net.UnixConn, http *bufio.ReadWriter, cid string) error {
	buf := make([]byte, bufferSize)
	for {
		numR, err := container.Read(buf)
		logrus.Debugf("Read fd(%d) %d/%d bytes for container %s", int(buf[0]), numR, len(buf), cid)

		if numR > 0 {
			switch buf[0] {
			case AttachPipeStdout:
				// Do nothing
			default:
				logrus.Errorf("Received unexpected attach type %+d, discarding %d bytes", buf[0], numR)
				continue
			}

			numW, err2 := http.Write(buf[1:numR])
			if err2 != nil {
				if err != nil {
					logrus.Errorf("Reading container %s STDOUT: %v", cid, err)
				}
				return err2
			} else if numW+1 != numR {
				return io.ErrShortWrite
			}
			// We need to force the buffer to write immediately, so
			// there isn't a delay on the terminal side.
			if err2 := http.Flush(); err2 != nil {
				if err != nil {
					logrus.Errorf("Reading container %s STDOUT: %v", cid, err)
				}
				return err2
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// Copy data from a container to an HTTP connection, for non-terminal attach.
// Appends a header to multiplex input.
func httpAttachNonTerminalCopy(container *net.UnixConn, http *bufio.ReadWriter, cid string, stdin, stdout, stderr bool) error {
	buf := make([]byte, bufferSize)
	for {
		numR, err := container.Read(buf)
		if numR > 0 {
			var headerBuf []byte

			// Subtract 1 because we strip the first byte (used for
			// multiplexing by Conmon).
			headerLen := uint32(numR - 1)
			// Practically speaking, we could make this buf[0] - 1,
			// but we need to validate it anyway.
			switch buf[0] {
			case AttachPipeStdin:
				headerBuf = makeHTTPAttachHeader(0, headerLen)
				if !stdin {
					continue
				}
			case AttachPipeStdout:
				if !stdout {
					continue
				}
				headerBuf = makeHTTPAttachHeader(1, headerLen)
			case AttachPipeStderr:
				if !stderr {
					continue
				}
				headerBuf = makeHTTPAttachHeader(2, headerLen)
			default:
				logrus.Errorf("Received unexpected attach type %+d, discarding %d bytes", buf[0], numR)
				continue
			}

			numH, err2 := http.Write(headerBuf)
			if err2 != nil {
				if err != nil {
					logrus.Errorf("Reading container %s standard streams: %v", cid, err)
				}

				return err2
			}
			// Hardcoding header length is pretty gross, but
			// fast. Should be safe, as this is a fixed part
			// of the protocol.
			if numH != 8 {
				if err != nil {
					logrus.Errorf("Reading container %s standard streams: %v", cid, err)
				}

				return io.ErrShortWrite
			}

			numW, err2 := http.Write(buf[1:numR])
			if err2 != nil {
				if err != nil {
					logrus.Errorf("Reading container %s standard streams: %v", cid, err)
				}

				return err2
			} else if numW+1 != numR {
				if err != nil {
					logrus.Errorf("Reading container %s standard streams: %v", cid, err)
				}

				return io.ErrShortWrite
			}
			// We need to force the buffer to write immediately, so
			// there isn't a delay on the terminal side.
			if err2 := http.Flush(); err2 != nil {
				if err != nil {
					logrus.Errorf("Reading container %s STDOUT: %v", cid, err)
				}
				return err2
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}
	}
}
