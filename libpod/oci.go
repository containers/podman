package libpod

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/cgroups"
	"github.com/containers/storage/pkg/idtools"
	"github.com/coreos/go-systemd/activation"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	kwait "k8s.io/apimachinery/pkg/util/wait"

	// TODO import these functions into libpod and remove the import
	// Trying to keep libpod from depending on CRI-O code
	"github.com/projectatomic/libpod/utils"
)

// OCI code is undergoing heavy rewrite

const (
	// CgroupfsCgroupsManager represents cgroupfs native cgroup manager
	CgroupfsCgroupsManager = "cgroupfs"
	// SystemdCgroupsManager represents systemd native cgroup manager
	SystemdCgroupsManager = "systemd"

	// ContainerCreateTimeout represents the value of container creating timeout
	ContainerCreateTimeout = 240 * time.Second

	// Timeout before declaring that runtime has failed to kill a given
	// container
	killContainerTimeout = 5 * time.Second
	// DefaultShmSize is the default shm size
	DefaultShmSize = 64 * 1024 * 1024
	// NsRunDir is the default directory in which running network namespaces
	// are stored
	NsRunDir = "/var/run/netns"
)

// OCIRuntime represents an OCI-compatible runtime that libpod can call into
// to perform container operations
type OCIRuntime struct {
	name          string
	path          string
	conmonPath    string
	conmonEnv     []string
	cgroupManager string
	tmpDir        string
	exitsDir      string
	socketsDir    string
	logSizeMax    int64
	noPivot       bool
}

// syncInfo is used to return data from monitor process to daemon
type syncInfo struct {
	Pid     int    `json:"pid"`
	Message string `json:"message,omitempty"`
}

// Make a new OCI runtime with provided options
func newOCIRuntime(name string, path string, conmonPath string, conmonEnv []string, cgroupManager string, tmpDir string, logSizeMax int64, noPivotRoot bool) (*OCIRuntime, error) {
	runtime := new(OCIRuntime)
	runtime.name = name
	runtime.path = path
	runtime.conmonPath = conmonPath
	runtime.conmonEnv = conmonEnv
	runtime.cgroupManager = cgroupManager
	runtime.tmpDir = tmpDir
	runtime.logSizeMax = logSizeMax
	runtime.noPivot = noPivotRoot

	runtime.exitsDir = filepath.Join(runtime.tmpDir, "exits")
	runtime.socketsDir = filepath.Join(runtime.tmpDir, "socket")

	if cgroupManager != CgroupfsCgroupsManager && cgroupManager != SystemdCgroupsManager {
		return nil, errors.Wrapf(ErrInvalidArg, "invalid cgroup manager specified: %s", cgroupManager)
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

// newPipe creates a unix socket pair for communication
func newPipe() (parent *os.File, child *os.File, err error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), "parent"), os.NewFile(uintptr(fds[0]), "child"), nil
}

// Create systemd unit name for cgroup scopes
func createUnitName(prefix string, name string) string {
	return fmt.Sprintf("%s-%s.scope", prefix, name)
}

// Wait for a container which has been sent a signal to stop
func waitContainerStop(ctr *Container, timeout time.Duration) error {
	done := make(chan struct{})
	chControl := make(chan struct{})
	go func() {
		for {
			select {
			case <-chControl:
				return
			default:
				// Check if the process is still around
				err := unix.Kill(ctr.state.PID, 0)
				if err == unix.ESRCH {
					close(done)
					return
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
		return errors.Errorf("container %s did not die within timeout", ctr.ID())
	}
}

// Wait for a set of given PIDs to stop
func waitPidsStop(pids []int, timeout time.Duration) error {
	done := make(chan struct{})
	chControl := make(chan struct{})
	go func() {
		for {
			select {
			case <-chControl:
				return
			default:
				allClosed := true
				for _, pid := range pids {
					if err := unix.Kill(pid, 0); err != unix.ESRCH {
						allClosed = false
						break
					}
				}
				if allClosed {
					close(done)
					return
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

// CreateContainer creates a container in the OCI runtime
// TODO terminal support for container
// Presently just ignoring conmon opts related to it
func (r *OCIRuntime) createContainer(ctr *Container, cgroupParent string) (err error) {
	if ctr.state.UserNSRoot == "" {
		// no need of an intermediate mount ns
		return r.createOCIContainer(ctr, cgroupParent)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runtime.LockOSThread()

		fd, err := os.Open(fmt.Sprintf("/proc/%d/task/%d/ns/mnt", os.Getpid(), unix.Gettid()))
		if err != nil {
			return
		}
		defer fd.Close()

		// create a new mountns on the current thread
		if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
			return
		}
		defer unix.Setns(int(fd.Fd()), unix.CLONE_NEWNS)

		// don't spread our mounts around
		err = unix.Mount("/", "/", "none", unix.MS_REC|unix.MS_SLAVE, "")
		if err != nil {
			return
		}
		err = unix.Mount(ctr.state.Mountpoint, ctr.state.RealMountpoint, "none", unix.MS_BIND, "")
		if err != nil {
			return
		}
		if err := idtools.MkdirAllAs(ctr.state.DestinationRunDir, 0700, ctr.RootUID(), ctr.RootGID()); err != nil {
			return
		}

		err = unix.Mount(ctr.state.RunDir, ctr.state.DestinationRunDir, "none", unix.MS_BIND, "")
		if err != nil {
			return
		}
		err = r.createOCIContainer(ctr, cgroupParent)
	}()
	wg.Wait()

	return err
}

func (r *OCIRuntime) createOCIContainer(ctr *Container, cgroupParent string) (err error) {
	var stderrBuf bytes.Buffer

	parentPipe, childPipe, err := newPipe()
	if err != nil {
		return errors.Wrapf(err, "error creating socket pair")
	}

	childStartPipe, parentStartPipe, err := newPipe()
	if err != nil {
		return errors.Wrapf(err, "error creating socket pair for start pipe")
	}

	defer parentPipe.Close()
	defer parentStartPipe.Close()

	args := []string{}
	if r.cgroupManager == SystemdCgroupsManager {
		args = append(args, "-s")
	}
	args = append(args, "-c", ctr.ID())
	args = append(args, "-u", ctr.ID())
	args = append(args, "-r", r.path)
	args = append(args, "-b", ctr.bundlePath())
	args = append(args, "-p", filepath.Join(ctr.state.RunDir, "pidfile"))
	args = append(args, "-l", ctr.LogPath())
	args = append(args, "--exit-dir", r.exitsDir)
	if ctr.config.ConmonPidFile != "" {
		args = append(args, "--conmon-pidfile", ctr.config.ConmonPidFile)
	}
	if len(ctr.config.ExitCommand) > 0 {
		args = append(args, "--exit-command", ctr.config.ExitCommand[0])
		for _, arg := range ctr.config.ExitCommand[1:] {
			args = append(args, []string{"--exit-command-arg", arg}...)
		}
	}
	args = append(args, "--socket-dir-path", r.socketsDir)
	if ctr.config.Spec.Process.Terminal {
		args = append(args, "-t")
	} else if ctr.config.Stdin {
		args = append(args, "-i")
	}
	if r.logSizeMax >= 0 {
		args = append(args, "--log-size-max", fmt.Sprintf("%v", r.logSizeMax))
	}
	if r.noPivot {
		args = append(args, "--no-pivot")
	}

	if logrus.GetLevel() == logrus.DebugLevel {
		logrus.Debug("%s messages will be logged to syslog", r.conmonPath)
		args = append(args, "--syslog")
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

	cmd.ExtraFiles = append(cmd.ExtraFiles, childPipe, childStartPipe)
	// 0, 1 and 2 are stdin, stdout and stderr
	cmd.Env = append(r.conmonEnv, fmt.Sprintf("_OCI_SYNCPIPE=%d", 3))
	cmd.Env = append(cmd.Env, fmt.Sprintf("_OCI_STARTPIPE=%d", 4))
	cmd.Env = append(cmd.Env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", GetRootlessRuntimeDir()))
	if notify, ok := os.LookupEnv("NOTIFY_SOCKET"); ok {
		cmd.Env = append(cmd.Env, fmt.Sprintf("NOTIFY_SOCKET=%s", notify))
	}
	if listenfds, ok := os.LookupEnv("LISTEN_FDS"); ok {
		cmd.Env = append(cmd.Env, fmt.Sprintf("LISTEN_FDS=%s", listenfds), "LISTEN_PID=1")
		fds := activation.Files(false)
		cmd.ExtraFiles = append(cmd.ExtraFiles, fds...)
	}

	if selinux.GetEnabled() {
		// Set the label of the conmon process to be level :s0
		// This will allow the container processes to talk to fifo-files
		// passed into the container by conmon
		plabel, err := selinux.CurrentLabel()
		if err != nil {
			childPipe.Close()
			return errors.Wrapf(err, "Failed to get current SELinux label")
		}

		c := selinux.NewContext(plabel)
		runtime.LockOSThread()
		if c["level"] != "s0" && c["level"] != "" {
			c["level"] = "s0"
			if err := label.SetProcessLabel(c.Get()); err != nil {
				runtime.UnlockOSThread()
				return err
			}
		}
		err = cmd.Start()
		// Ignore error returned from SetProcessLabel("") call,
		// can't recover.
		label.SetProcessLabel("")
		runtime.UnlockOSThread()
	} else {
		err = cmd.Start()
	}
	if err != nil {
		childPipe.Close()
		return err
	}

	// We don't need childPipe on the parent side
	childPipe.Close()
	childStartPipe.Close()

	// Move conmon to specified cgroup
	if os.Getuid() == 0 {
		if r.cgroupManager == SystemdCgroupsManager {
			unitName := createUnitName("libpod-conmon", ctr.ID())

			logrus.Infof("Running conmon under slice %s and unitName %s", cgroupParent, unitName)
			if err = utils.RunUnderSystemdScope(cmd.Process.Pid, cgroupParent, unitName); err != nil {
				logrus.Warnf("Failed to add conmon to systemd sandbox cgroup: %v", err)
			}
		} else {
			cgroupPath := filepath.Join(ctr.config.CgroupParent, fmt.Sprintf("libpod-%s", ctr.ID()), "conmon")
			control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(cgroupPath), &spec.LinuxResources{})
			if err != nil {
				logrus.Warnf("Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
			} else {
				// we need to remove this defer and delete the cgroup once conmon exits
				// maybe need a conmon monitor?
				if err := control.Add(cgroups.Process{Pid: cmd.Process.Pid}); err != nil {
					logrus.Warnf("Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
				}
			}
		}
	}

	/* We set the cgroup, now the child can start creating children */
	someData := []byte{0}
	_, err = parentStartPipe.Write(someData)
	if err != nil {
		return err
	}

	/* Wait for initial setup and fork, and reap child */
	err = cmd.Wait()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			if err2 := r.deleteContainer(ctr); err2 != nil {
				logrus.Errorf("Error removing container %s from runtime after creation failed", ctr.ID())
			}
		}
	}()

	// Wait to get container pid from conmon
	type syncStruct struct {
		si  *syncInfo
		err error
	}
	ch := make(chan syncStruct)
	go func() {
		var si *syncInfo
		if err = json.NewDecoder(parentPipe).Decode(&si); err != nil {
			ch <- syncStruct{err: err}
			return
		}
		ch <- syncStruct{si: si}
	}()

	select {
	case ss := <-ch:
		if ss.err != nil {
			return errors.Wrapf(ss.err, "error reading container (probably exited) json message")
		}
		logrus.Debugf("Received container pid: %d", ss.si.Pid)
		if ss.si.Pid == -1 {
			if ss.si.Message != "" {
				return errors.Wrapf(ErrInternal, "container create failed: %s", ss.si.Message)
			}
			return errors.Wrapf(ErrInternal, "container create failed")
		}
	case <-time.After(ContainerCreateTimeout):
		return errors.Wrapf(ErrInternal, "container creation timeout")
	}
	return nil
}

// updateContainerStatus retrieves the current status of the container from the
// runtime. It updates the container's state but does not save it.
func (r *OCIRuntime) updateContainerStatus(ctr *Container) error {
	state := new(spec.State)

	// Store old state so we know if we were already stopped
	oldState := ctr.state.State

	out, err := exec.Command(r.path, "state", ctr.ID()).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "does not exist") {
			ctr.removeConmonFiles()
			ctr.state.State = ContainerStateConfigured
			return nil
		}
		return errors.Wrapf(err, "error getting container %s state. stderr/out: %s", ctr.ID(), out)
	}
	if err := json.NewDecoder(bytes.NewBuffer(out)).Decode(state); err != nil {
		return errors.Wrapf(err, "error decoding container status for container %s", ctr.ID())
	}
	ctr.state.PID = state.Pid

	switch state.Status {
	case "created":
		ctr.state.State = ContainerStateCreated
	case "paused":
		ctr.state.State = ContainerStatePaused
	case "running":
		ctr.state.State = ContainerStateRunning
	case "stopped":
		ctr.state.State = ContainerStateStopped
	default:
		return errors.Wrapf(ErrInternal, "unrecognized status returned by runtime for container %s: %s",
			ctr.ID(), state.Status)
	}

	// Only grab exit status if we were not already stopped
	// If we were, it should already be in the database
	if ctr.state.State == ContainerStateStopped && oldState != ContainerStateStopped {
		exitFile := filepath.Join(r.exitsDir, ctr.ID())
		var fi os.FileInfo
		err = kwait.ExponentialBackoff(
			kwait.Backoff{
				Duration: 500 * time.Millisecond,
				Factor:   1.2,
				Steps:    6,
			},
			func() (bool, error) {
				var err error
				fi, err = os.Stat(exitFile)
				if err != nil {
					// wait longer
					return false, nil
				}
				return true, nil
			})
		if err != nil {
			ctr.state.ExitCode = -1
			ctr.state.FinishedTime = time.Now()
			logrus.Errorf("No exit file for container %s found: %v", ctr.ID(), err)
			return nil
		}

		ctr.state.FinishedTime = getFinishedTime(fi)
		statusCodeStr, err := ioutil.ReadFile(exitFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read exit file for container %s", ctr.ID())
		}
		statusCode, err := strconv.Atoi(string(statusCodeStr))
		if err != nil {
			return errors.Wrapf(err, "error converting exit status code for container %s to int",
				ctr.ID())
		}
		ctr.state.ExitCode = int32(statusCode)

		oomFilePath := filepath.Join(ctr.bundlePath(), "oom")
		if _, err = os.Stat(oomFilePath); err == nil {
			ctr.state.OOMKilled = true
		}

	}

	return nil
}

// startContainer starts the given container
// Sets time the container was started, but does not save it.
func (r *OCIRuntime) startContainer(ctr *Container) error {
	// TODO: streams should probably *not* be our STDIN/OUT/ERR - redirect to buffers?
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, r.path, "start", ctr.ID()); err != nil {
		return err
	}

	ctr.state.StartedTime = time.Now()

	return nil
}

// killContainer sends the given signal to the given container
func (r *OCIRuntime) killContainer(ctr *Container, signal uint) error {
	logrus.Debugf("Sending signal %d to container %s", signal, ctr.ID())
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, r.path, "kill", ctr.ID(), fmt.Sprintf("%d", signal)); err != nil {
		return errors.Wrapf(err, "error sending signal to container %s", ctr.ID())
	}

	return nil
}

// stopContainer stops a container, first using its given stop signal (or
// SIGTERM if no signal was specified), then using SIGKILL
// Timeout is given in seconds. If timeout is 0, the container will be
// immediately kill with SIGKILL
// Does not set finished time for container, assumes you will run updateStatus
// after to pull the exit code
func (r *OCIRuntime) stopContainer(ctr *Container, timeout uint) error {
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
		if err := r.killContainer(ctr, stopSignal); err != nil {
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

	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, r.path, "kill", "--all", ctr.ID(), "KILL"); err != nil {
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

// deleteContainer deletes a container from the OCI runtime
func (r *OCIRuntime) deleteContainer(ctr *Container) error {
	_, err := utils.ExecCmd(r.path, "delete", "--force", ctr.ID())
	return err
}

// pauseContainer pauses the given container
func (r *OCIRuntime) pauseContainer(ctr *Container) error {
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, r.path, "pause", ctr.ID())
}

// unpauseContainer unpauses the given container
func (r *OCIRuntime) unpauseContainer(ctr *Container) error {
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, r.path, "resume", ctr.ID())
}

// execContainer executes a command in a running container
// TODO: Add --detach support
// TODO: Convert to use conmon
// TODO: add --pid-file and use that to generate exec session tracking
func (r *OCIRuntime) execContainer(c *Container, cmd, capAdd, env []string, tty bool, user, sessionID string) (*exec.Cmd, error) {
	if len(cmd) == 0 {
		return nil, errors.Wrapf(ErrInvalidArg, "must provide a command to execute")
	}

	if sessionID == "" {
		return nil, errors.Wrapf(ErrEmptyID, "must provide a session ID for exec")
	}

	args := []string{}

	// TODO - should we maintain separate logpaths for exec sessions?
	args = append(args, "--log", c.LogPath())

	args = append(args, "exec")

	args = append(args, "--cwd", c.config.Spec.Process.Cwd)

	args = append(args, "--pid-file", c.execPidPath(sessionID))

	if tty {
		args = append(args, "--tty")
	}

	if user != "" {
		args = append(args, "--user", user)
	}

	if c.config.Spec.Process.NoNewPrivileges {
		args = append(args, "--no-new-privs")
	}

	for _, cap := range capAdd {
		args = append(args, "--cap", cap)
	}

	for _, envVar := range env {
		args = append(args, "--env", envVar)
	}

	// Append container ID and command
	args = append(args, c.ID())
	args = append(args, cmd...)

	logrus.Debugf("Starting runtime %s with following arguments: %v", r.path, args)

	execCmd := exec.Command(r.path, args...)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	return execCmd, nil
}

// execStopContainer stops all active exec sessions in a container
// It will also stop all other processes in the container. It is only intended
// to be used to assist in cleanup when removing a container.
// SIGTERM is used by default to stop processes. If SIGTERM fails, SIGKILL will be used.
func (r *OCIRuntime) execStopContainer(ctr *Container, timeout uint) error {
	// Do we have active exec sessions?
	if len(ctr.state.ExecSessions) == 0 {
		return nil
	}

	// Get a list of active exec sessions
	execSessions := []int{}
	for _, session := range ctr.state.ExecSessions {
		pid := session.PID
		// Ping the PID with signal 0 to see if it still exists
		if err := unix.Kill(pid, 0); err == unix.ESRCH {
			continue
		}

		execSessions = append(execSessions, pid)
	}

	// All the sessions may be dead
	// If they are, just return
	if len(execSessions) == 0 {
		return nil
	}

	// If timeout is 0, just use SIGKILL
	if timeout > 0 {
		// Stop using SIGTERM by default
		// Use SIGSTOP after a timeout
		logrus.Debugf("Killing all processes in container %s with SIGTERM", ctr.ID())
		if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, r.path, "kill", "--all", ctr.ID(), "TERM"); err != nil {
			return errors.Wrapf(err, "error sending SIGTERM to container %s processes", ctr.ID())
		}

		// Wait for all processes to stop
		if err := waitPidsStop(execSessions, time.Duration(timeout)*time.Second); err != nil {
			logrus.Warnf("Timed out stopping container %s exec sessions", ctr.ID())
		} else {
			// No error, all exec sessions are dead
			return nil
		}
	}

	// Send SIGKILL
	logrus.Debugf("Killing all processes in container %s with SIGKILL", ctr.ID())
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, r.path, "kill", "--all", ctr.ID(), "KILL"); err != nil {
		return errors.Wrapf(err, "error sending SIGKILL to container %s processes", ctr.ID())
	}

	// Give the processes a few seconds to go down
	if err := waitPidsStop(execSessions, killContainerTimeout); err != nil {
		return errors.Wrapf(err, "failed to kill container %s exec sessions", ctr.ID())
	}

	return nil
}
