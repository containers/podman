package oci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/cgroups"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/projectatomic/libpod/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	// ContainerStateCreated represents the created state of a container
	ContainerStateCreated = "created"
	// ContainerStatePaused represents the paused state of a container
	ContainerStatePaused = "paused"
	// ContainerStateRunning represents the running state of a container
	ContainerStateRunning = "running"
	// ContainerStateStopped represents the stopped state of a container
	ContainerStateStopped = "stopped"
	// ContainerCreateTimeout represents the value of container creating timeout
	ContainerCreateTimeout = 240 * time.Second

	// CgroupfsCgroupsManager represents cgroupfs native cgroup manager
	CgroupfsCgroupsManager = "cgroupfs"
	// SystemdCgroupsManager represents systemd native cgroup manager
	SystemdCgroupsManager = "systemd"
	// ContainerExitsDir is the location of container exit dirs
	ContainerExitsDir = "/var/run/crio/exits"
	// ContainerAttachSocketDir is the location for container attach sockets
	ContainerAttachSocketDir = "/var/run/crio"

	// killContainerTimeout is the timeout that we wait for the container to
	// be SIGKILLed.
	killContainerTimeout = 2 * time.Minute
)

// New creates a new Runtime with options provided
func New(runtimeTrustedPath string,
	runtimeUntrustedPath string,
	trustLevel string,
	conmonPath string,
	conmonEnv []string,
	cgroupManager string,
	containerExitsDir string,
	logSizeMax int64,
	noPivot bool) (*Runtime, error) {
	r := &Runtime{
		name:              filepath.Base(runtimeTrustedPath),
		trustedPath:       runtimeTrustedPath,
		untrustedPath:     runtimeUntrustedPath,
		trustLevel:        trustLevel,
		conmonPath:        conmonPath,
		conmonEnv:         conmonEnv,
		cgroupManager:     cgroupManager,
		containerExitsDir: containerExitsDir,
		logSizeMax:        logSizeMax,
		noPivot:           noPivot,
	}
	return r, nil
}

// Runtime stores the information about a oci runtime
type Runtime struct {
	name              string
	trustedPath       string
	untrustedPath     string
	trustLevel        string
	conmonPath        string
	conmonEnv         []string
	cgroupManager     string
	containerExitsDir string
	logSizeMax        int64
	noPivot           bool
}

// syncInfo is used to return data from monitor process to daemon
type syncInfo struct {
	Pid     int    `json:"pid"`
	Message string `json:"message,omitempty"`
}

// exitCodeInfo is used to return the monitored process exit code to the daemon
type exitCodeInfo struct {
	ExitCode int32  `json:"exit_code"`
	Message  string `json:"message,omitempty"`
}

// Name returns the name of the OCI Runtime
func (r *Runtime) Name() string {
	return r.name
}

// Path returns the full path the OCI Runtime executable.
// Depending if the container is privileged and/or trusted,
// this will return either the trusted or untrusted runtime path.
func (r *Runtime) Path(c *Container) string {
	if !c.trusted {
		// We have an explicitly untrusted container.
		if c.privileged {
			logrus.Warnf("Running an untrusted but privileged container")
			return r.trustedPath
		}

		if r.untrustedPath != "" {
			return r.untrustedPath
		}

		return r.trustedPath
	}

	// Our container is trusted. Let's look at the configured trust level.
	if r.trustLevel == "trusted" {
		return r.trustedPath
	}

	// Our container is trusted, but we are running untrusted.
	// We will use the untrusted container runtime if it's set
	// and if it's not a privileged container.
	if c.privileged || r.untrustedPath == "" {
		return r.trustedPath
	}

	return r.untrustedPath
}

// Version returns the version of the OCI Runtime
func (r *Runtime) Version() (string, error) {
	runtimeVersion, err := getOCIVersion(r.trustedPath, "-v")
	if err != nil {
		return "", err
	}
	return runtimeVersion, nil
}

func getOCIVersion(name string, args ...string) (string, error) {
	out, err := utils.ExecCmd(name, args...)
	if err != nil {
		return "", err
	}

	firstLine := out[:strings.Index(out, "\n")]
	v := firstLine[strings.LastIndex(firstLine, " ")+1:]
	return v, nil
}

// CreateContainer creates a container.
func (r *Runtime) CreateContainer(c *Container, cgroupParent string) (err error) {
	var stderrBuf bytes.Buffer
	parentPipe, childPipe, err := newPipe()
	childStartPipe, parentStartPipe, err := newPipe()
	if err != nil {
		return fmt.Errorf("error creating socket pair: %v", err)
	}
	defer parentPipe.Close()
	defer parentStartPipe.Close()

	var args []string
	if r.cgroupManager == SystemdCgroupsManager {
		args = append(args, "-s")
	}
	args = append(args, "-c", c.id)
	args = append(args, "-u", c.id)
	args = append(args, "-r", r.Path(c))
	args = append(args, "-b", c.bundlePath)
	args = append(args, "-p", filepath.Join(c.bundlePath, "pidfile"))
	args = append(args, "-l", c.logPath)
	args = append(args, "--exit-dir", r.containerExitsDir)
	args = append(args, "--socket-dir-path", ContainerAttachSocketDir)
	if r.logSizeMax >= 0 {
		args = append(args, "--log-size-max", fmt.Sprintf("%v", r.logSizeMax))
	}
	if r.noPivot {
		args = append(args, "--no-pivot")
	}
	if c.terminal {
		args = append(args, "-t")
	} else if c.stdin {
		args = append(args, "-i")
	}
	logrus.WithFields(logrus.Fields{
		"args": args,
	}).Debugf("running conmon: %s", r.conmonPath)

	cmd := exec.Command(r.conmonPath, args...)
	cmd.Dir = c.bundlePath
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if c.terminal {
		cmd.Stderr = &stderrBuf
	}
	cmd.ExtraFiles = append(cmd.ExtraFiles, childPipe, childStartPipe)
	// 0, 1 and 2 are stdin, stdout and stderr
	cmd.Env = append(r.conmonEnv, fmt.Sprintf("_OCI_SYNCPIPE=%d", 3))
	cmd.Env = append(cmd.Env, fmt.Sprintf("_OCI_STARTPIPE=%d", 4))

	err = cmd.Start()
	if err != nil {
		childPipe.Close()
		return err
	}

	// We don't need childPipe on the parent side
	childPipe.Close()
	childStartPipe.Close()

	// Move conmon to specified cgroup
	if r.cgroupManager == SystemdCgroupsManager {
		logrus.Infof("Running conmon under slice %s and unitName %s", cgroupParent, createUnitName("crio-conmon", c.id))
		if err = utils.RunUnderSystemdScope(cmd.Process.Pid, cgroupParent, createUnitName("crio-conmon", c.id)); err != nil {
			logrus.Warnf("Failed to add conmon to systemd sandbox cgroup: %v", err)
		}
	} else {
		control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(filepath.Join(cgroupParent, "/crio-conmon-"+c.id)), &rspec.LinuxResources{})
		if err != nil {
			logrus.Warnf("Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
		} else {
			// Here we should defer a crio-connmon- cgroup hierarchy deletion, but it will
			// always fail as conmon's pid is still there.
			// Fortunately, kubelet takes care of deleting this for us, so the leak will
			// only happens in corner case where one does a manual deletion of the container
			// through e.g. runc. This should be handled by implementing a conmon monitoring
			// routine that does the cgroup cleanup once conmon is terminated.
			if err := control.Add(cgroups.Process{Pid: cmd.Process.Pid}); err != nil {
				logrus.Warnf("Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
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

	// We will delete all container resources if creation fails
	defer func() {
		if err != nil {
			r.DeleteContainer(c)
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
			return fmt.Errorf("error reading container (probably exited) json message: %v", ss.err)
		}
		logrus.Debugf("Received container pid: %d", ss.si.Pid)
		if ss.si.Pid == -1 {
			if ss.si.Message != "" {
				logrus.Errorf("Container creation error: %s", ss.si.Message)
				return fmt.Errorf("container create failed: %s", ss.si.Message)
			}
			logrus.Errorf("Container creation failed")
			return fmt.Errorf("container create failed")
		}
	case <-time.After(ContainerCreateTimeout):
		logrus.Errorf("Container creation timeout (%v)", ContainerCreateTimeout)
		return fmt.Errorf("create container timeout")
	}
	return nil
}

func createUnitName(prefix string, name string) string {
	return fmt.Sprintf("%s-%s.scope", prefix, name)
}

// StartContainer starts a container.
func (r *Runtime) StartContainer(c *Container) error {
	c.opLock.Lock()
	defer c.opLock.Unlock()
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, r.Path(c), "start", c.id); err != nil {
		return err
	}
	c.state.Started = time.Now()
	return nil
}

// ExecSyncResponse is returned from ExecSync.
type ExecSyncResponse struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int32
}

// ExecSyncError wraps command's streams, exit code and error on ExecSync error.
type ExecSyncError struct {
	Stdout   bytes.Buffer
	Stderr   bytes.Buffer
	ExitCode int32
	Err      error
}

func (e ExecSyncError) Error() string {
	return fmt.Sprintf("command error: %+v, stdout: %s, stderr: %s, exit code %d", e.Err, e.Stdout.Bytes(), e.Stderr.Bytes(), e.ExitCode)
}

func prepareExec() (pidFile, parentPipe, childPipe *os.File, err error) {
	parentPipe, childPipe, err = os.Pipe()
	if err != nil {
		return nil, nil, nil, err
	}

	pidFile, err = ioutil.TempFile("", "pidfile")
	if err != nil {
		parentPipe.Close()
		childPipe.Close()
		return nil, nil, nil, err
	}

	return
}

func parseLog(log []byte) (stdout, stderr []byte) {
	// Split the log on newlines, which is what separates entries.
	lines := bytes.SplitAfter(log, []byte{'\n'})
	for _, line := range lines {
		// Ignore empty lines.
		if len(line) == 0 {
			continue
		}

		// The format of log lines is "DATE pipe REST".
		parts := bytes.SplitN(line, []byte{' '}, 3)
		if len(parts) < 3 {
			// Ignore the line if it's formatted incorrectly, but complain
			// about it so it can be debugged.
			logrus.Warnf("hit invalid log format: %q", string(line))
			continue
		}

		pipe := string(parts[1])
		content := parts[2]

		switch pipe {
		case "stdout":
			stdout = append(stdout, content...)
		case "stderr":
			stderr = append(stderr, content...)
		default:
			// Complain about unknown pipes.
			logrus.Warnf("hit invalid log format [unknown pipe %s]: %q", pipe, string(line))
			continue
		}
	}

	return stdout, stderr
}

// ExecSync execs a command in a container and returns it's stdout, stderr and return code.
func (r *Runtime) ExecSync(c *Container, command []string, timeout int64) (resp *ExecSyncResponse, err error) {
	pidFile, parentPipe, childPipe, err := prepareExec()
	if err != nil {
		return nil, ExecSyncError{
			ExitCode: -1,
			Err:      err,
		}
	}
	defer parentPipe.Close()
	defer func() {
		if e := os.Remove(pidFile.Name()); e != nil {
			logrus.Warnf("could not remove temporary PID file %s", pidFile.Name())
		}
	}()

	logFile, err := ioutil.TempFile("", "crio-log-"+c.id)
	if err != nil {
		return nil, ExecSyncError{
			ExitCode: -1,
			Err:      err,
		}
	}
	logPath := logFile.Name()
	defer func() {
		logFile.Close()
		os.RemoveAll(logPath)
	}()

	f, err := ioutil.TempFile("", "exec-process")
	if err != nil {
		return nil, ExecSyncError{
			ExitCode: -1,
			Err:      err,
		}
	}
	defer os.RemoveAll(f.Name())

	var args []string
	args = append(args, "-c", c.id)
	args = append(args, "-r", r.Path(c))
	args = append(args, "-p", pidFile.Name())
	args = append(args, "-e")
	if c.terminal {
		args = append(args, "-t")
	}
	if timeout > 0 {
		args = append(args, "-T")
		args = append(args, fmt.Sprintf("%d", timeout))
	}
	args = append(args, "-l", logPath)
	args = append(args, "--socket-dir-path", ContainerAttachSocketDir)

	pspec := c.Spec().Process
	pspec.Env = append(pspec.Env, r.conmonEnv...)
	pspec.Args = command
	processJSON, err := json.Marshal(pspec)
	if err != nil {
		return nil, ExecSyncError{
			ExitCode: -1,
			Err:      err,
		}
	}

	if err := ioutil.WriteFile(f.Name(), processJSON, 0644); err != nil {
		return nil, ExecSyncError{
			ExitCode: -1,
			Err:      err,
		}
	}

	args = append(args, "--exec-process-spec", f.Name())

	cmd := exec.Command(r.conmonPath, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.ExtraFiles = append(cmd.ExtraFiles, childPipe)
	// 0, 1 and 2 are stdin, stdout and stderr
	cmd.Env = append(r.conmonEnv, fmt.Sprintf("_OCI_SYNCPIPE=%d", 3))

	err = cmd.Start()
	if err != nil {
		childPipe.Close()
		return nil, ExecSyncError{
			Stdout:   stdoutBuf,
			Stderr:   stderrBuf,
			ExitCode: -1,
			Err:      err,
		}
	}

	// We don't need childPipe on the parent side
	childPipe.Close()

	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(unix.WaitStatus); ok {
				return nil, ExecSyncError{
					Stdout:   stdoutBuf,
					Stderr:   stderrBuf,
					ExitCode: int32(status.ExitStatus()),
					Err:      err,
				}
			}
		} else {
			return nil, ExecSyncError{
				Stdout:   stdoutBuf,
				Stderr:   stderrBuf,
				ExitCode: -1,
				Err:      err,
			}
		}
	}

	var ec *exitCodeInfo
	if err := json.NewDecoder(parentPipe).Decode(&ec); err != nil {
		return nil, ExecSyncError{
			Stdout:   stdoutBuf,
			Stderr:   stderrBuf,
			ExitCode: -1,
			Err:      err,
		}
	}

	logrus.Infof("Received container exit code: %v, message: %s", ec.ExitCode, ec.Message)

	if ec.ExitCode == -1 {
		return nil, ExecSyncError{
			Stdout:   stdoutBuf,
			Stderr:   stderrBuf,
			ExitCode: -1,
			Err:      fmt.Errorf(ec.Message),
		}
	}

	// The actual logged output is not the same as stdoutBuf and stderrBuf,
	// which are used for getting error information. For the actual
	// ExecSyncResponse we have to read the logfile.
	// XXX: Currently runC dups the same console over both stdout and stderr,
	//      so we can't differentiate between the two.

	logBytes, err := ioutil.ReadFile(logPath)
	if err != nil {
		return nil, ExecSyncError{
			Stdout:   stdoutBuf,
			Stderr:   stderrBuf,
			ExitCode: -1,
			Err:      err,
		}
	}

	// We have to parse the log output into {stdout, stderr} buffers.
	stdoutBytes, stderrBytes := parseLog(logBytes)
	return &ExecSyncResponse{
		Stdout:   stdoutBytes,
		Stderr:   stderrBytes,
		ExitCode: ec.ExitCode,
	}, nil
}

func waitContainerStop(ctx context.Context, c *Container, timeout time.Duration) error {
	done := make(chan struct{})
	// we could potentially re-use "done" channel to exit the loop on timeout
	// but we use another channel "chControl" so that we won't never incur in the
	// case the "done" channel is closed in the "default" select case and we also
	// reach the timeout in the select below. If that happens we could raise
	// a panic closing a closed channel so better be safe and use another new
	// channel just to control the loop.
	chControl := make(chan struct{})
	go func() {
		for {
			select {
			case <-chControl:
				return
			default:
				// Check if the process is still around
				err := unix.Kill(c.state.Pid, 0)
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
	case <-ctx.Done():
		close(chControl)
		return ctx.Err()
	case <-time.After(timeout):
		close(chControl)
		err := unix.Kill(c.state.Pid, unix.SIGKILL)
		if err != nil && err != unix.ESRCH {
			return fmt.Errorf("failed to kill process: %v", err)
		}
	}

	c.state.Finished = time.Now()
	return nil
}

// StopContainer stops a container. Timeout is given in seconds.
func (r *Runtime) StopContainer(ctx context.Context, c *Container, timeout int64) error {
	c.opLock.Lock()
	defer c.opLock.Unlock()

	// Check if the process is around before sending a signal
	err := unix.Kill(c.state.Pid, 0)
	if err == unix.ESRCH {
		c.state.Finished = time.Now()
		return nil
	}

	if timeout > 0 {
		if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, r.Path(c), "kill", c.id, c.GetStopSignal()); err != nil {
			return fmt.Errorf("failed to stop container %s, %v", c.id, err)
		}
		err = waitContainerStop(ctx, c, time.Duration(timeout)*time.Second)
		if err == nil {
			return nil
		}
		logrus.Warnf("Stop container %q timed out: %v", c.ID(), err)
	}

	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, r.Path(c), "kill", "--all", c.id, "KILL"); err != nil {
		return fmt.Errorf("failed to stop container %s, %v", c.id, err)
	}

	return waitContainerStop(ctx, c, killContainerTimeout)
}

// DeleteContainer deletes a container.
func (r *Runtime) DeleteContainer(c *Container) error {
	c.opLock.Lock()
	defer c.opLock.Unlock()
	_, err := utils.ExecCmd(r.Path(c), "delete", "--force", c.id)
	return err
}

// SetStartFailed sets the container state appropriately after a start failure
func (r *Runtime) SetStartFailed(c *Container, err error) {
	c.opLock.Lock()
	defer c.opLock.Unlock()
	// adjust finished and started times
	c.state.Finished, c.state.Started = c.state.Created, c.state.Created
	c.state.Error = err.Error()
}

// UpdateStatus refreshes the status of the container.
func (r *Runtime) UpdateStatus(c *Container) error {
	c.opLock.Lock()
	defer c.opLock.Unlock()
	out, err := exec.Command(r.Path(c), "state", c.id).CombinedOutput()
	if err != nil {
		// there are many code paths that could lead to have a bad state in the
		// underlying runtime.
		// On any error like a container went away or we rebooted and containers
		// went away we do not error out stopping kubernetes to recover.
		// We always populate the fields below so kube can restart/reschedule
		// containers failing.
		c.state.Status = ContainerStateStopped
		c.state.Finished = time.Now()
		c.state.ExitCode = 255
		return nil
	}
	if err := json.NewDecoder(bytes.NewBuffer(out)).Decode(&c.state); err != nil {
		return fmt.Errorf("failed to decode container status for %s: %s", c.id, err)
	}

	if c.state.Status == ContainerStateStopped {
		exitFilePath := filepath.Join(r.containerExitsDir, c.id)
		var fi os.FileInfo
		err = kwait.ExponentialBackoff(
			kwait.Backoff{
				Duration: 500 * time.Millisecond,
				Factor:   1.2,
				Steps:    6,
			},
			func() (bool, error) {
				var err error
				fi, err = os.Stat(exitFilePath)
				if err != nil {
					// wait longer
					return false, nil
				}
				return true, nil
			})
		if err != nil {
			logrus.Warnf("failed to find container exit file: %v", err)
			c.state.ExitCode = -1
		} else {
			c.state.Finished = getFinishedTime(fi)
			statusCodeStr, err := ioutil.ReadFile(exitFilePath)
			if err != nil {
				return fmt.Errorf("failed to read exit file: %v", err)
			}
			statusCode, err := strconv.Atoi(string(statusCodeStr))
			if err != nil {
				return fmt.Errorf("status code conversion failed: %v", err)
			}
			c.state.ExitCode = int32(statusCode)
		}

		oomFilePath := filepath.Join(c.bundlePath, "oom")
		if _, err = os.Stat(oomFilePath); err == nil {
			c.state.OOMKilled = true
		}
	}

	return nil
}

// ContainerStatus returns the state of a container.
func (r *Runtime) ContainerStatus(c *Container) *ContainerState {
	c.opLock.Lock()
	defer c.opLock.Unlock()
	return c.state
}

// newPipe creates a unix socket pair for communication
func newPipe() (parent *os.File, child *os.File, err error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), "parent"), os.NewFile(uintptr(fds[0]), "child"), nil
}

// RuntimeReady checks if the runtime is up and ready to accept
// basic containers e.g. container only needs host network.
func (r *Runtime) RuntimeReady() (bool, error) {
	return true, nil
}

// NetworkReady checks if the runtime network is up and ready to
// accept containers which require container network.
func (r *Runtime) NetworkReady() (bool, error) {
	return true, nil
}

// PauseContainer pauses a container.
func (r *Runtime) PauseContainer(c *Container) error {
	c.opLock.Lock()
	defer c.opLock.Unlock()
	_, err := utils.ExecCmd(r.Path(c), "pause", c.id)
	return err
}

// UnpauseContainer unpauses a container.
func (r *Runtime) UnpauseContainer(c *Container) error {
	c.opLock.Lock()
	defer c.opLock.Unlock()
	_, err := utils.ExecCmd(r.Path(c), "resume", c.id)
	return err
}
