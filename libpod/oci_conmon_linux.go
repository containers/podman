// +build linux

package libpod

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/libpod/libpod/config"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/containers/libpod/pkg/lookup"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/libpod/utils"
	pmount "github.com/containers/storage/pkg/mount"
	"github.com/coreos/go-systemd/activation"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// ConmonOCIRuntime is an OCI runtime managed by Conmon.
// TODO: Make all calls to OCI runtime have a timeout.
type ConmonOCIRuntime struct {
	name              string
	path              string
	conmonPath        string
	conmonEnv         []string
	cgroupManager     string
	tmpDir            string
	exitsDir          string
	socketsDir        string
	logSizeMax        int64
	noPivot           bool
	reservePorts      bool
	supportsJSON      bool
	supportsNoCgroups bool
	sdNotify          bool
}

// Make a new Conmon-based OCI runtime with the given options.
// Conmon will wrap the given OCI runtime, which can be `runc`, `crun`, or
// any runtime with a runc-compatible CLI.
// The first path that points to a valid executable will be used.
// Deliberately private. Someone should not be able to construct this outside of
// libpod.
func newConmonOCIRuntime(name string, paths []string, conmonPath string, runtimeCfg *config.Config, supportsJSON, supportsNoCgroups bool) (OCIRuntime, error) {
	if name == "" {
		return nil, errors.Wrapf(define.ErrInvalidArg, "the OCI runtime must be provided a non-empty name")
	}

	runtime := new(ConmonOCIRuntime)
	runtime.name = name
	runtime.conmonPath = conmonPath

	runtime.conmonEnv = runtimeCfg.ConmonEnvVars
	runtime.cgroupManager = runtimeCfg.CgroupManager
	runtime.tmpDir = runtimeCfg.TmpDir
	runtime.logSizeMax = runtimeCfg.MaxLogSize
	runtime.noPivot = runtimeCfg.NoPivotRoot
	runtime.reservePorts = runtimeCfg.EnablePortReservation
	runtime.sdNotify = runtimeCfg.SDNotify

	// TODO: probe OCI runtime for feature and enable automatically if
	// available.
	runtime.supportsJSON = supportsJSON
	runtime.supportsNoCgroups = supportsNoCgroups

	foundPath := false
	for _, path := range paths {
		stat, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, errors.Wrapf(err, "cannot stat %s", path)
		}
		if !stat.Mode().IsRegular() {
			continue
		}
		foundPath = true
		runtime.path = path
		logrus.Debugf("using runtime %q", path)
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
		return nil, errors.Wrapf(define.ErrInvalidArg, "no valid executable found for OCI runtime %s", name)
	}

	runtime.exitsDir = filepath.Join(runtime.tmpDir, "exits")
	runtime.socketsDir = filepath.Join(runtime.tmpDir, "socket")

	if runtime.cgroupManager != define.CgroupfsCgroupsManager && runtime.cgroupManager != define.SystemdCgroupsManager {
		return nil, errors.Wrapf(define.ErrInvalidArg, "invalid cgroup manager specified: %s", runtime.cgroupManager)
	}

	// Create the exit files and attach sockets directories
	if err := os.MkdirAll(runtime.exitsDir, 0750); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return nil, errors.Wrapf(err, "error creating OCI runtime exit files directory %s",
				runtime.exitsDir)
		}
	}
	if err := os.MkdirAll(runtime.socketsDir, 0750); err != nil {
		// The directory is allowed to exist
		if !os.IsExist(err) {
			return nil, errors.Wrapf(err, "error creating OCI runtime attach sockets directory %s",
				runtime.socketsDir)
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

// CreateContainer creates a container.
func (r *ConmonOCIRuntime) CreateContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (err error) {
	if len(ctr.config.IDMappings.UIDMap) != 0 || len(ctr.config.IDMappings.GIDMap) != 0 {
		for _, i := range []string{ctr.state.RunDir, ctr.runtime.config.TmpDir, ctr.config.StaticDir, ctr.state.Mountpoint, ctr.runtime.config.VolumePath} {
			if err := makeAccessible(i, ctr.RootUID(), ctr.RootGID()); err != nil {
				return err
			}
		}

		// if we are running a non privileged container, be sure to umount some kernel paths so they are not
		// bind mounted inside the container at all.
		if !ctr.config.Privileged && !rootless.IsRootless() {
			ch := make(chan error)
			go func() {
				runtime.LockOSThread()
				err := func() error {
					fd, err := os.Open(fmt.Sprintf("/proc/%d/task/%d/ns/mnt", os.Getpid(), unix.Gettid()))
					if err != nil {
						return err
					}
					defer errorhandling.CloseQuiet(fd)

					// create a new mountns on the current thread
					if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
						return err
					}
					defer func() {
						if err := unix.Setns(int(fd.Fd()), unix.CLONE_NEWNS); err != nil {
							logrus.Errorf("unable to clone new namespace: %q", err)
						}
					}()

					// don't spread our mounts around.  We are setting only /sys to be slave
					// so that the cleanup process is still able to umount the storage and the
					// changes are propagated to the host.
					err = unix.Mount("/sys", "/sys", "none", unix.MS_REC|unix.MS_SLAVE, "")
					if err != nil {
						return errors.Wrapf(err, "cannot make /sys slave")
					}

					mounts, err := pmount.GetMounts()
					if err != nil {
						return err
					}
					for _, m := range mounts {
						if !strings.HasPrefix(m.Mountpoint, "/sys/kernel") {
							continue
						}
						err = unix.Unmount(m.Mountpoint, 0)
						if err != nil && !os.IsNotExist(err) {
							return errors.Wrapf(err, "cannot unmount %s", m.Mountpoint)
						}
					}
					return r.createOCIContainer(ctr, restoreOptions)
				}()
				ch <- err
			}()
			err := <-ch
			return err
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
	exitFile, err := r.ExitFilePath(ctr)
	if err != nil {
		return err
	}

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
		return errors.Wrapf(err, "getting stdout pipe")
	}
	errPipe, err := cmd.StderrPipe()
	if err != nil {
		return errors.Wrapf(err, "getting stderr pipe")
	}

	if err := cmd.Start(); err != nil {
		out, err2 := ioutil.ReadAll(errPipe)
		if err2 != nil {
			return errors.Wrapf(err, "error getting container %s state", ctr.ID())
		}
		if strings.Contains(string(out), "does not exist") {
			if err := ctr.removeConmonFiles(); err != nil {
				logrus.Debugf("unable to remove conmon files for container %s", ctr.ID())
			}
			ctr.state.ExitCode = -1
			ctr.state.FinishedTime = time.Now()
			ctr.state.State = define.ContainerStateExited
			return nil
		}
		return errors.Wrapf(err, "error getting container %s state. stderr/out: %s", ctr.ID(), out)
	}
	defer func() {
		_ = cmd.Wait()
	}()

	if err := errPipe.Close(); err != nil {
		return err
	}
	out, err := ioutil.ReadAll(outPipe)
	if err != nil {
		return errors.Wrapf(err, "error reading stdout: %s", ctr.ID())
	}
	if err := json.NewDecoder(bytes.NewBuffer(out)).Decode(state); err != nil {
		return errors.Wrapf(err, "error decoding container status for container %s", ctr.ID())
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
		return errors.Wrapf(define.ErrInternal, "unrecognized status returned by runtime for container %s: %s",
			ctr.ID(), state.Status)
	}

	// Only grab exit status if we were not already stopped
	// If we were, it should already be in the database
	if ctr.state.State == define.ContainerStateStopped && oldState != define.ContainerStateStopped {
		var fi os.FileInfo
		chWait := make(chan error)
		defer close(chWait)

		_, err := WaitForFile(exitFile, chWait, time.Second*5)
		if err == nil {
			fi, err = os.Stat(exitFile)
		}
		if err != nil {
			ctr.state.ExitCode = -1
			ctr.state.FinishedTime = time.Now()
			logrus.Errorf("No exit file for container %s found: %v", ctr.ID(), err)
			return nil
		}

		return ctr.handleExitFile(exitFile, fi)
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
	if notify, ok := os.LookupEnv("NOTIFY_SOCKET"); ok {
		env = append(env, fmt.Sprintf("NOTIFY_SOCKET=%s", notify))
	}
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, "start", ctr.ID()); err != nil {
		return err
	}

	ctr.state.StartedTime = time.Now()

	return nil
}

// KillContainer sends the given signal to the given container.
// If all is set, send to all PIDs in the container.
// All is only supported if the container created cgroups.
func (r *ConmonOCIRuntime) KillContainer(ctr *Container, signal uint, all bool) error {
	logrus.Debugf("Sending signal %d to container %s", signal, ctr.ID())
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	var args []string
	if all {
		args = []string{"kill", "--all", ctr.ID(), fmt.Sprintf("%d", signal)}
	} else {
		args = []string{"kill", ctr.ID(), fmt.Sprintf("%d", signal)}
	}
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, args...); err != nil {
		return errors.Wrapf(err, "error sending signal to container %s", ctr.ID())
	}

	return nil
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

	stopSignal := ctr.config.StopSignal
	if stopSignal == 0 {
		stopSignal = uint(syscall.SIGTERM)
	}

	if timeout > 0 {
		if err := r.KillContainer(ctr, stopSignal, all); err != nil {
			// Is the container gone?
			// If so, it probably died between the first check and
			// our sending the signal
			// The container is stopped, so exit cleanly
			err := unix.Kill(ctr.state.PID, 0)
			if err == unix.ESRCH {
				return nil
			}

			return err
		}

		if err := waitContainerStop(ctr, time.Duration(timeout)*time.Second); err != nil {
			logrus.Warnf("Timed out stopping container %s, resorting to SIGKILL", ctr.ID())
		} else {
			// No error, the container is dead
			return nil
		}
	}

	if err := r.KillContainer(ctr, 9, all); err != nil {
		// Again, check if the container is gone. If it is, exit cleanly.
		err := unix.Kill(ctr.state.PID, 0)
		if err == unix.ESRCH {
			return nil
		}

		return errors.Wrapf(err, "error sending SIGKILL to container %s", ctr.ID())
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
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, "delete", "--force", ctr.ID())
}

// PauseContainer pauses the given container.
func (r *ConmonOCIRuntime) PauseContainer(ctr *Container) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, "pause", ctr.ID())
}

// UnpauseContainer unpauses the given container.
func (r *ConmonOCIRuntime) UnpauseContainer(ctr *Container) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, "resume", ctr.ID())
}

// ExecContainer executes a command in a running container
// TODO: Split into Create/Start/Attach/Wait
func (r *ConmonOCIRuntime) ExecContainer(c *Container, sessionID string, options *ExecOptions) (int, chan error, error) {
	if options == nil {
		return -1, nil, errors.Wrapf(define.ErrInvalidArg, "must provide an ExecOptions struct to ExecContainer")
	}
	if len(options.Cmd) == 0 {
		return -1, nil, errors.Wrapf(define.ErrInvalidArg, "must provide a command to execute")
	}

	if sessionID == "" {
		return -1, nil, errors.Wrapf(define.ErrEmptyID, "must provide a session ID for exec")
	}

	// create sync pipe to receive the pid
	parentSyncPipe, childSyncPipe, err := newPipe()
	if err != nil {
		return -1, nil, errors.Wrapf(err, "error creating socket pair")
	}

	defer errorhandling.CloseQuiet(parentSyncPipe)

	// create start pipe to set the cgroup before running
	// attachToExec is responsible for closing parentStartPipe
	childStartPipe, parentStartPipe, err := newPipe()
	if err != nil {
		return -1, nil, errors.Wrapf(err, "error creating socket pair")
	}

	// We want to make sure we close the parent{Start,Attach}Pipes if we fail
	// but also don't want to close them after attach to exec is called
	attachToExecCalled := false

	defer func() {
		if !attachToExecCalled {
			errorhandling.CloseQuiet(parentStartPipe)
		}
	}()

	// create the attach pipe to allow attach socket to be created before
	// $RUNTIME exec starts running. This is to make sure we can capture all output
	// from the process through that socket, rather than half reading the log, half attaching to the socket
	// attachToExec is responsible for closing parentAttachPipe
	parentAttachPipe, childAttachPipe, err := newPipe()
	if err != nil {
		return -1, nil, errors.Wrapf(err, "error creating socket pair")
	}

	defer func() {
		if !attachToExecCalled {
			errorhandling.CloseQuiet(parentAttachPipe)
		}
	}()

	childrenClosed := false
	defer func() {
		if !childrenClosed {
			errorhandling.CloseQuiet(childSyncPipe)
			errorhandling.CloseQuiet(childAttachPipe)
			errorhandling.CloseQuiet(childStartPipe)
		}
	}()

	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return -1, nil, err
	}

	finalEnv := make([]string, 0, len(options.Env))
	for k, v := range options.Env {
		finalEnv = append(finalEnv, fmt.Sprintf("%s=%s", k, v))
	}

	processFile, err := prepareProcessExec(c, options.Cmd, finalEnv, options.Terminal, options.Cwd, options.User, sessionID)
	if err != nil {
		return -1, nil, err
	}

	var ociLog string
	if logrus.GetLevel() != logrus.DebugLevel && r.supportsJSON {
		ociLog = c.execOCILog(sessionID)
	}
	args := r.sharedConmonArgs(c, sessionID, c.execBundlePath(sessionID), c.execPidPath(sessionID), c.execLogPath(sessionID), c.execExitFileDir(sessionID), ociLog)

	if options.PreserveFDs > 0 {
		args = append(args, formatRuntimeOpts("--preserve-fds", fmt.Sprintf("%d", options.PreserveFDs))...)
	}

	for _, capability := range options.CapAdd {
		args = append(args, formatRuntimeOpts("--cap", capability)...)
	}

	if options.Terminal {
		args = append(args, "-t")
	}

	// Append container ID and command
	args = append(args, "-e")
	// TODO make this optional when we can detach
	args = append(args, "--exec-attach")
	args = append(args, "--exec-process-spec", processFile.Name())

	logrus.WithFields(logrus.Fields{
		"args": args,
	}).Debugf("running conmon: %s", r.conmonPath)
	execCmd := exec.Command(r.conmonPath, args...)

	if options.Streams != nil {
		if options.Streams.AttachInput {
			execCmd.Stdin = options.Streams.InputStream
		}
		if options.Streams.AttachOutput {
			execCmd.Stdout = options.Streams.OutputStream
		}
		if options.Streams.AttachError {
			execCmd.Stderr = options.Streams.ErrorStream
		}
	}

	conmonEnv, extraFiles, err := r.configureConmonEnv(runtimeDir)
	if err != nil {
		return -1, nil, err
	}

	if options.PreserveFDs > 0 {
		for fd := 3; fd < int(3+options.PreserveFDs); fd++ {
			execCmd.ExtraFiles = append(execCmd.ExtraFiles, os.NewFile(uintptr(fd), fmt.Sprintf("fd-%d", fd)))
		}
	}

	// we don't want to step on users fds they asked to preserve
	// Since 0-2 are used for stdio, start the fds we pass in at preserveFDs+3
	execCmd.Env = append(r.conmonEnv, fmt.Sprintf("_OCI_SYNCPIPE=%d", options.PreserveFDs+3), fmt.Sprintf("_OCI_STARTPIPE=%d", options.PreserveFDs+4), fmt.Sprintf("_OCI_ATTACHPIPE=%d", options.PreserveFDs+5))
	execCmd.Env = append(execCmd.Env, conmonEnv...)

	execCmd.ExtraFiles = append(execCmd.ExtraFiles, childSyncPipe, childStartPipe, childAttachPipe)
	execCmd.ExtraFiles = append(execCmd.ExtraFiles, extraFiles...)
	execCmd.Dir = c.execBundlePath(sessionID)
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	err = startCommandGivenSelinux(execCmd)

	// We don't need children pipes  on the parent side
	errorhandling.CloseQuiet(childSyncPipe)
	errorhandling.CloseQuiet(childAttachPipe)
	errorhandling.CloseQuiet(childStartPipe)
	childrenClosed = true

	if err != nil {
		return -1, nil, errors.Wrapf(err, "cannot start container %s", c.ID())
	}
	if err := r.moveConmonToCgroupAndSignal(c, execCmd, parentStartPipe); err != nil {
		return -1, nil, err
	}

	if options.PreserveFDs > 0 {
		for fd := 3; fd < int(3+options.PreserveFDs); fd++ {
			// These fds were passed down to the runtime.  Close them
			// and not interfere
			if err := os.NewFile(uintptr(fd), fmt.Sprintf("fd-%d", fd)).Close(); err != nil {
				logrus.Debugf("unable to close file fd-%d", fd)
			}
		}
	}

	// TODO Only create if !detach
	// Attach to the container before starting it
	attachChan := make(chan error)
	go func() {
		// attachToExec is responsible for closing pipes
		attachChan <- c.attachToExec(options.Streams, options.DetachKeys, options.Resize, sessionID, parentStartPipe, parentAttachPipe)
		close(attachChan)
	}()
	attachToExecCalled = true

	pid, err := readConmonPipeData(parentSyncPipe, ociLog)

	return pid, attachChan, err
}

// ExecStopContainer stops a given exec session in a running container.
func (r *ConmonOCIRuntime) ExecStopContainer(ctr *Container, sessionID string, timeout uint) error {
	session, ok := ctr.state.ExecSessions[sessionID]
	if !ok {
		// TODO This should probably be a separate error
		return errors.Wrapf(define.ErrInvalidArg, "no exec session with ID %s found in container %s", sessionID, ctr.ID())
	}

	logrus.Debugf("Going to stop container %s exec session %s", ctr.ID(), sessionID)

	// Is the session dead?
	// Ping the PID with signal 0 to see if it still exists.
	if err := unix.Kill(session.PID, 0); err != nil {
		if err == unix.ESRCH {
			return nil
		}
		return errors.Wrapf(err, "error pinging container %s exec session %s PID %d with signal 0", ctr.ID(), sessionID, session.PID)
	}

	if timeout > 0 {
		// Use SIGTERM by default, then SIGSTOP after timeout.
		logrus.Debugf("Killing exec session %s (PID %d) of container %s with SIGTERM", sessionID, session.PID, ctr.ID())
		if err := unix.Kill(session.PID, unix.SIGTERM); err != nil {
			if err == unix.ESRCH {
				return nil
			}
			return errors.Wrapf(err, "error killing container %s exec session %s PID %d with SIGTERM", ctr.ID(), sessionID, session.PID)
		}

		// Wait for the PID to stop
		if err := waitPidStop(session.PID, time.Duration(timeout)*time.Second); err != nil {
			logrus.Warnf("Timed out waiting for container %s exec session %s to stop, resorting to SIGKILL", ctr.ID(), sessionID)
		} else {
			// No error, container is dead
			return nil
		}
	}

	// SIGTERM did not work. On to SIGKILL.
	logrus.Debugf("Killing exec session %s (PID %d) of container %s with SIGKILL", sessionID, session.PID, ctr.ID())
	if err := unix.Kill(session.PID, unix.SIGTERM); err != nil {
		if err == unix.ESRCH {
			return nil
		}
		return errors.Wrapf(err, "error killing container %s exec session %s PID %d with SIGKILL", ctr.ID(), sessionID, session.PID)
	}

	// Wait for the PID to stop
	if err := waitPidStop(session.PID, killContainerTimeout*time.Second); err != nil {
		return errors.Wrapf(err, "timed out waiting for container %s exec session %s PID %d to stop after SIGKILL", ctr.ID(), sessionID, session.PID)
	}

	return nil
}

// ExecCleanupContainer cleans up files created when a command is run via
// ExecContainer. This includes the attach socket for the exec session.
func (r *ConmonOCIRuntime) ExecContainerCleanup(ctr *Container, sessionID string) error {
	// Clean up the sockets dir. Issue #3962
	// Also ignore if it doesn't exist for some reason; hence the conditional return below
	if err := os.RemoveAll(filepath.Join(r.socketsDir, sessionID)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CheckpointContainer checkpoints the given container.
func (r *ConmonOCIRuntime) CheckpointContainer(ctr *Container, options ContainerCheckpointOptions) error {
	if err := label.SetSocketLabel(ctr.ProcessLabel()); err != nil {
		return err
	}
	// imagePath is used by CRIU to store the actual checkpoint files
	imagePath := ctr.CheckpointPath()
	// workPath will be used to store dump.log and stats-dump
	workPath := ctr.bundlePath()
	logrus.Debugf("Writing checkpoint to %s", imagePath)
	logrus.Debugf("Writing checkpoint logs to %s", workPath)
	args := []string{}
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
	args = append(args, ctr.ID())
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, nil, r.path, args...)
}

// SupportsCheckpoint checks if the OCI runtime supports checkpointing
// containers.
func (r *ConmonOCIRuntime) SupportsCheckpoint() bool {
	// Check if the runtime implements checkpointing. Currently only
	// runc's checkpoint/restore implementation is supported.
	cmd := exec.Command(r.path, "checkpoint", "-h")
	if err := cmd.Start(); err != nil {
		return false
	}
	if err := cmd.Wait(); err == nil {
		return true
	}
	return false
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

// AttachSocketPath is the path to a single container's attach socket.
func (r *ConmonOCIRuntime) AttachSocketPath(ctr *Container) (string, error) {
	if ctr == nil {
		return "", errors.Wrapf(define.ErrInvalidArg, "must provide a valid container to get attach socket path")
	}

	return filepath.Join(r.socketsDir, ctr.ID(), "attach"), nil
}

// ExecAttachSocketPath is the path to a container's exec session attach socket.
func (r *ConmonOCIRuntime) ExecAttachSocketPath(ctr *Container, sessionID string) (string, error) {
	// We don't even use container, so don't validity check it
	if sessionID == "" {
		return "", errors.Wrapf(define.ErrInvalidArg, "must provide a valid session ID to get attach socket path")
	}

	return filepath.Join(r.socketsDir, sessionID, "attach"), nil
}

// ExitFilePath is the path to a container's exit file.
func (r *ConmonOCIRuntime) ExitFilePath(ctr *Container) (string, error) {
	if ctr == nil {
		return "", errors.Wrapf(define.ErrInvalidArg, "must provide a valid container to get exit file path")
	}
	return filepath.Join(r.exitsDir, ctr.ID()), nil
}

// RuntimeInfo provides information on the runtime.
func (r *ConmonOCIRuntime) RuntimeInfo() (map[string]interface{}, error) {
	runtimePackage := packageVersion(r.path)
	conmonPackage := packageVersion(r.conmonPath)
	runtimeVersion, err := r.getOCIRuntimeVersion()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting version of OCI runtime %s", r.name)
	}
	conmonVersion, err := r.getConmonVersion()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting conmon version")
	}

	info := make(map[string]interface{})
	info["Conmon"] = map[string]interface{}{
		"path":    r.conmonPath,
		"package": conmonPackage,
		"version": conmonVersion,
	}
	info["OCIRuntime"] = map[string]interface{}{
		"name":    r.name,
		"path":    r.path,
		"package": runtimePackage,
		"version": runtimeVersion,
	}

	return info, nil
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
	done := make(chan struct{})
	chControl := make(chan struct{})
	go func() {
		for {
			select {
			case <-chControl:
				return
			default:
				if err := unix.Kill(pid, 0); err != nil {
					if err == unix.ESRCH {
						close(done)
						return
					}
					logrus.Errorf("Error pinging PID %d with signal 0: %v", pid, err)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		close(chControl)
		return errors.Errorf("given PIDs did not die within timeout")
	}
}

// createOCIContainer generates this container's main conmon instance and prepares it for starting
func (r *ConmonOCIRuntime) createOCIContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (err error) {
	var stderrBuf bytes.Buffer

	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}

	parentSyncPipe, childSyncPipe, err := newPipe()
	if err != nil {
		return errors.Wrapf(err, "error creating socket pair")
	}
	defer errorhandling.CloseQuiet(parentSyncPipe)

	childStartPipe, parentStartPipe, err := newPipe()
	if err != nil {
		return errors.Wrapf(err, "error creating socket pair for start pipe")
	}

	defer errorhandling.CloseQuiet(parentStartPipe)

	var ociLog string
	if logrus.GetLevel() != logrus.DebugLevel && r.supportsJSON {
		ociLog = filepath.Join(ctr.state.RunDir, "oci-log")
	}
	args := r.sharedConmonArgs(ctr, ctr.ID(), ctr.bundlePath(), filepath.Join(ctr.state.RunDir, "pidfile"), ctr.LogPath(), r.exitsDir, ociLog)

	if ctr.config.Spec.Process.Terminal {
		args = append(args, "-t")
	} else if ctr.config.Stdin {
		args = append(args, "-i")
	}

	if ctr.config.ConmonPidFile != "" {
		args = append(args, "--conmon-pidfile", ctr.config.ConmonPidFile)
	}

	if r.noPivot {
		args = append(args, "--no-pivot")
	}

	if len(ctr.config.ExitCommand) > 0 {
		args = append(args, "--exit-command", ctr.config.ExitCommand[0])
		for _, arg := range ctr.config.ExitCommand[1:] {
			args = append(args, []string{"--exit-command-arg", arg}...)
		}
	}

	if restoreOptions != nil {
		args = append(args, "--restore", ctr.CheckpointPath())
		if restoreOptions.TCPEstablished {
			args = append(args, "--runtime-opt", "--tcp-established")
		}
	}

	logrus.WithFields(logrus.Fields{
		"args": args,
	}).Debugf("running conmon: %s", r.conmonPath)

	cmd := exec.Command(r.conmonPath, args...)
	cmd.Dir = ctr.bundlePath()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	// TODO this is probably a really bad idea for some uses
	// Make this configurable
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if ctr.config.Spec.Process.Terminal {
		cmd.Stderr = &stderrBuf
	}

	// 0, 1 and 2 are stdin, stdout and stderr
	conmonEnv, envFiles, err := r.configureConmonEnv(runtimeDir)
	if err != nil {
		return err
	}

	cmd.Env = append(r.conmonEnv, fmt.Sprintf("_OCI_SYNCPIPE=%d", 3), fmt.Sprintf("_OCI_STARTPIPE=%d", 4))
	cmd.Env = append(cmd.Env, conmonEnv...)
	cmd.ExtraFiles = append(cmd.ExtraFiles, childSyncPipe, childStartPipe)
	cmd.ExtraFiles = append(cmd.ExtraFiles, envFiles...)

	if r.reservePorts && !ctr.config.NetMode.IsSlirp4netns() {
		ports, err := bindPorts(ctr.config.PortMappings)
		if err != nil {
			return err
		}

		// Leak the port we bound in the conmon process.  These fd's won't be used
		// by the container and conmon will keep the ports busy so that another
		// process cannot use them.
		cmd.ExtraFiles = append(cmd.ExtraFiles, ports...)
	}

	if ctr.config.NetMode.IsSlirp4netns() {
		if ctr.config.PostConfigureNetNS {
			ctr.rootlessSlirpSyncR, ctr.rootlessSlirpSyncW, err = os.Pipe()
			if err != nil {
				return errors.Wrapf(err, "failed to create rootless network sync pipe")
			}
		} else {
			if ctr.rootlessSlirpSyncR != nil {
				defer errorhandling.CloseQuiet(ctr.rootlessSlirpSyncR)
			}
			if ctr.rootlessSlirpSyncW != nil {
				defer errorhandling.CloseQuiet(ctr.rootlessSlirpSyncW)
			}
		}
		// Leak one end in conmon, the other one will be leaked into slirp4netns
		cmd.ExtraFiles = append(cmd.ExtraFiles, ctr.rootlessSlirpSyncW)
	}

	err = startCommandGivenSelinux(cmd)
	// regardless of whether we errored or not, we no longer need the children pipes
	childSyncPipe.Close()
	childStartPipe.Close()
	if err != nil {
		return err
	}
	if err := r.moveConmonToCgroupAndSignal(ctr, cmd, parentStartPipe); err != nil {
		return err
	}
	/* Wait for initial setup and fork, and reap child */
	err = cmd.Wait()
	if err != nil {
		return err
	}

	pid, err := readConmonPipeData(parentSyncPipe, ociLog)
	if err != nil {
		if err2 := r.DeleteContainer(ctr); err2 != nil {
			logrus.Errorf("Error removing container %s from runtime after creation failed", ctr.ID())
		}
		return err
	}
	ctr.state.PID = pid

	conmonPID, err := readConmonPidFile(ctr.config.ConmonPidFile)
	if err != nil {
		logrus.Warnf("error reading conmon pid file for container %s: %s", ctr.ID(), err.Error())
	} else if conmonPID > 0 {
		// conmon not having a pid file is a valid state, so don't set it if we don't have it
		logrus.Infof("Got Conmon PID as %d", conmonPID)
		ctr.state.ConmonPID = conmonPID
	}

	return nil
}

// prepareProcessExec returns the path of the process.json used in runc exec -p
// caller is responsible to close the returned *os.File if needed.
func prepareProcessExec(c *Container, cmd, env []string, tty bool, cwd, user, sessionID string) (*os.File, error) {
	f, err := ioutil.TempFile(c.execBundlePath(sessionID), "exec-process-")
	if err != nil {
		return nil, err
	}
	pspec := c.config.Spec.Process
	pspec.SelinuxLabel = c.config.ProcessLabel
	pspec.Args = cmd
	// We need to default this to false else it will inherit terminal as true
	// from the container.
	pspec.Terminal = false
	if tty {
		pspec.Terminal = true
	}
	if len(env) > 0 {
		pspec.Env = append(pspec.Env, env...)
	}

	if cwd != "" {
		pspec.Cwd = cwd

	}

	overrides := c.getUserOverrides()
	execUser, err := lookup.GetUserGroupInfo(c.state.Mountpoint, user, overrides)
	if err != nil {
		return nil, err
	}

	// If user was set, look it up in the container to get a UID to use on
	// the host
	if user != "" {
		sgids := make([]uint32, 0, len(execUser.Sgids))
		for _, sgid := range execUser.Sgids {
			sgids = append(sgids, uint32(sgid))
		}
		processUser := spec.User{
			UID:            uint32(execUser.Uid),
			GID:            uint32(execUser.Gid),
			AdditionalGids: sgids,
		}

		pspec.User = processUser
	}

	hasHomeSet := false
	for _, s := range pspec.Env {
		if strings.HasPrefix(s, "HOME=") {
			hasHomeSet = true
			break
		}
	}
	if !hasHomeSet {
		pspec.Env = append(pspec.Env, fmt.Sprintf("HOME=%s", execUser.Home))
	}

	processJSON, err := json.Marshal(pspec)
	if err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(f.Name(), processJSON, 0644); err != nil {
		return nil, err
	}
	return f, nil
}

// configureConmonEnv gets the environment values to add to conmon's exec struct
// TODO this may want to be less hardcoded/more configurable in the future
func (r *ConmonOCIRuntime) configureConmonEnv(runtimeDir string) ([]string, []*os.File, error) {
	env := make([]string, 0, 6)
	env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir))
	env = append(env, fmt.Sprintf("_CONTAINERS_USERNS_CONFIGURED=%s", os.Getenv("_CONTAINERS_USERNS_CONFIGURED")))
	env = append(env, fmt.Sprintf("_CONTAINERS_ROOTLESS_UID=%s", os.Getenv("_CONTAINERS_ROOTLESS_UID")))
	home, err := util.HomeDir()
	if err != nil {
		return nil, nil, err
	}
	env = append(env, fmt.Sprintf("HOME=%s", home))

	extraFiles := make([]*os.File, 0)
	if notify, ok := os.LookupEnv("NOTIFY_SOCKET"); ok {
		env = append(env, fmt.Sprintf("NOTIFY_SOCKET=%s", notify))
	}
	if !r.sdNotify {
		if listenfds, ok := os.LookupEnv("LISTEN_FDS"); ok {
			env = append(env, fmt.Sprintf("LISTEN_FDS=%s", listenfds), "LISTEN_PID=1")
			fds := activation.Files(false)
			extraFiles = append(extraFiles, fds...)
		}
	} else {
		logrus.Debug("disabling SD notify")
	}
	return env, extraFiles, nil
}

// sharedConmonArgs takes common arguments for exec and create/restore and formats them for the conmon CLI
func (r *ConmonOCIRuntime) sharedConmonArgs(ctr *Container, cuuid, bundlePath, pidPath, logPath, exitDir, ociLogPath string) []string {
	// set the conmon API version to be able to use the correct sync struct keys
	args := []string{"--api-version", "1"}
	if r.cgroupManager == define.SystemdCgroupsManager && !ctr.config.NoCgroups {
		args = append(args, "-s")
	}
	args = append(args, "-c", ctr.ID())
	args = append(args, "-u", cuuid)
	args = append(args, "-r", r.path)
	args = append(args, "-b", bundlePath)
	args = append(args, "-p", pidPath)

	var logDriver string
	switch ctr.LogDriver() {
	case JournaldLogging:
		logDriver = JournaldLogging
	case JSONLogging:
		fallthrough
	default: //nolint-stylecheck
		// No case here should happen except JSONLogging, but keep this here in case the options are extended
		logrus.Errorf("%s logging specified but not supported. Choosing k8s-file logging instead", ctr.LogDriver())
		fallthrough
	case "":
		// to get here, either a user would specify `--log-driver ""`, or this came from another place in libpod
		// since the former case is obscure, and the latter case isn't an error, let's silently fallthrough
		fallthrough
	case KubernetesLogging:
		logDriver = fmt.Sprintf("%s:%s", KubernetesLogging, logPath)
	}

	args = append(args, "-l", logDriver)
	args = append(args, "--exit-dir", exitDir)
	args = append(args, "--socket-dir-path", r.socketsDir)
	if r.logSizeMax >= 0 {
		args = append(args, "--log-size-max", fmt.Sprintf("%v", r.logSizeMax))
	}

	logLevel := logrus.GetLevel()
	args = append(args, "--log-level", logLevel.String())

	if logLevel == logrus.DebugLevel {
		logrus.Debugf("%s messages will be logged to syslog", r.conmonPath)
		args = append(args, "--syslog")
	}
	if ociLogPath != "" {
		args = append(args, "--runtime-arg", "--log-format=json", "--runtime-arg", "--log", fmt.Sprintf("--runtime-arg=%s", ociLogPath))
	}
	if ctr.config.NoCgroups {
		logrus.Debugf("Running with no CGroups")
		args = append(args, "--runtime-arg", "--cgroup-manager", "--runtime-arg", "disabled")
	}
	return args
}

// startCommandGivenSelinux starts a container ensuring to set the labels of
// the process to make sure SELinux doesn't block conmon communication, if SELinux is enabled
func startCommandGivenSelinux(cmd *exec.Cmd) error {
	if !selinux.GetEnabled() {
		return cmd.Start()
	}
	// Set the label of the conmon process to be level :s0
	// This will allow the container processes to talk to fifo-files
	// passed into the container by conmon
	var (
		plabel string
		con    selinux.Context
		err    error
	)
	plabel, err = selinux.CurrentLabel()
	if err != nil {
		return errors.Wrapf(err, "Failed to get current SELinux label")
	}

	con, err = selinux.NewContext(plabel)
	if err != nil {
		return errors.Wrapf(err, "Failed to get new context from SELinux label")
	}

	runtime.LockOSThread()
	if con["level"] != "s0" && con["level"] != "" {
		con["level"] = "s0"
		if err = label.SetProcessLabel(con.Get()); err != nil {
			runtime.UnlockOSThread()
			return err
		}
	}
	err = cmd.Start()
	// Ignore error returned from SetProcessLabel("") call,
	// can't recover.
	if labelErr := label.SetProcessLabel(""); labelErr != nil {
		logrus.Errorf("unable to set process label: %q", err)
	}
	runtime.UnlockOSThread()
	return err
}

// moveConmonToCgroupAndSignal gets a container's cgroupParent and moves the conmon process to that cgroup
// it then signals for conmon to start by sending nonse data down the start fd
func (r *ConmonOCIRuntime) moveConmonToCgroupAndSignal(ctr *Container, cmd *exec.Cmd, startFd *os.File) error {
	mustCreateCgroup := true
	// If cgroup creation is disabled - just signal.
	if ctr.config.NoCgroups {
		mustCreateCgroup = false
	}

	if mustCreateCgroup {
		cgroupParent := ctr.CgroupParent()
		if r.cgroupManager == define.SystemdCgroupsManager {
			unitName := createUnitName("libpod-conmon", ctr.ID())

			realCgroupParent := cgroupParent
			splitParent := strings.Split(cgroupParent, "/")
			if strings.HasSuffix(cgroupParent, ".slice") && len(splitParent) > 1 {
				realCgroupParent = splitParent[len(splitParent)-1]
			}

			logrus.Infof("Running conmon under slice %s and unitName %s", realCgroupParent, unitName)
			if err := utils.RunUnderSystemdScope(cmd.Process.Pid, realCgroupParent, unitName); err != nil {
				logrus.Warnf("Failed to add conmon to systemd sandbox cgroup: %v", err)
			}
		} else {
			cgroupPath := filepath.Join(ctr.config.CgroupParent, "conmon")
			control, err := cgroups.New(cgroupPath, &spec.LinuxResources{})
			if err != nil {
				logrus.Warnf("Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
			} else {
				// we need to remove this defer and delete the cgroup once conmon exits
				// maybe need a conmon monitor?
				if err := control.AddPid(cmd.Process.Pid); err != nil {
					logrus.Warnf("Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
				}
			}
		}
	}

	/* We set the cgroup, now the child can start creating children */
	if err := writeConmonPipeData(startFd); err != nil {
		return err
	}
	return nil
}

// newPipe creates a unix socket pair for communication
func newPipe() (parent *os.File, child *os.File, err error) {
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
		contents, err := ioutil.ReadFile(pidFile)
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
func readConmonPipeData(pipe *os.File, ociLog string) (int, error) {
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
		if err != nil {
			ch <- syncStruct{err: err}
		}
		if err := json.Unmarshal(b, &si); err != nil {
			ch <- syncStruct{err: err}
			return
		}
		ch <- syncStruct{si: si}
	}()

	data := -1
	select {
	case ss := <-ch:
		if ss.err != nil {
			if ociLog != "" {
				ociLogData, err := ioutil.ReadFile(ociLog)
				if err == nil {
					var ociErr ociError
					if err := json.Unmarshal(ociLogData, &ociErr); err == nil {
						return -1, getOCIRuntimeError(ociErr.Msg)
					}
				}
			}
			return -1, errors.Wrapf(ss.err, "container create failed (no logs from conmon)")
		}
		logrus.Debugf("Received: %d", ss.si.Data)
		if ss.si.Data < 0 {
			if ociLog != "" {
				ociLogData, err := ioutil.ReadFile(ociLog)
				if err == nil {
					var ociErr ociError
					if err := json.Unmarshal(ociLogData, &ociErr); err == nil {
						return ss.si.Data, getOCIRuntimeError(ociErr.Msg)
					}
				}
			}
			// If we failed to parse the JSON errors, then print the output as it is
			if ss.si.Message != "" {
				return ss.si.Data, getOCIRuntimeError(ss.si.Message)
			}
			return ss.si.Data, errors.Wrapf(define.ErrInternal, "container create failed")
		}
		data = ss.si.Data
	case <-time.After(define.ContainerCreateTimeout):
		return -1, errors.Wrapf(define.ErrInternal, "container creation timeout")
	}
	return data, nil
}

// writeConmonPipeData writes nonse data to a pipe
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
