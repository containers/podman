package libpod

import (
	"context"
	"fmt"
	"io"
	ioutil "io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	tasktypes "github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	cio "github.com/containerd/containerd/pkg/cri/io"
	cioutil "github.com/containerd/containerd/pkg/ioutil"
	client "github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/fifo"
	"github.com/containerd/ttrpc"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	// shimv2PIDFileName holds the filename that contains
	// the shimv2 runtime PID
	shimv2PIDFileName = "shim.pid"
	// shimv2AddressFileName holds the filename that contains
	// the shimv2 runtime socket address
	shimv2AddressFileName = "address"
)

// Shimv2Runtime holds information about the runtime
type Shimv2Runtime struct {
	name     string
	path     string
	tmpDir   string
	exitsDir string
	client   *ttrpc.Client
	task     task.TaskService
	ctx      context.Context
	sync.Mutex
	ctrs map[string]containerInfo
}

type containerInfo struct {
	cio *cio.ContainerIO
}

// NewShimv2Runtime returns a new instance of Shimv2Runtime
func NewShimv2Runtime(name string, paths []string, runtimeFlags []string, runtimeConfig *config.Config) (OCIRuntime, error) {
	logrus.Debug("NewShimv2Runtime start")
	defer logrus.Debug("NewShimv2Runtime end")

	s := new(Shimv2Runtime)

	err := s.setRuntimePath(name, paths)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot set shimv2 runtime path")
	}
	s.name = name
	s.ctrs = make(map[string]containerInfo)

	// type namespaceKey struct{}
	// s.ctx = context.WithValue(
	// 	context.Background(),
	// 	namespaceKey{},
	// 	runtimeConfig.Engine.Namespace)
	s.ctx = namespaces.WithNamespace(context.Background(), namespaces.Default)
	if len(runtimeConfig.Engine.Namespace) > 0 {
		s.ctx = namespaces.WithNamespace(context.Background(), runtimeConfig.Engine.Namespace)
	}

	// create the exits directory to store the exit code
	s.tmpDir = runtimeConfig.Engine.TmpDir
	s.exitsDir = filepath.Join(s.tmpDir, "exits")
	if err := os.MkdirAll(s.exitsDir, 0750); err != nil {
		if !os.IsExist(err) {
			return nil, errors.Wrapf(err, "error creating exits directory from shimv2 runtime")
		}
	}

	return s, nil
}

func waitCtrTerminate(sig uint, stopCh chan struct{}, errCh chan error, timeout time.Duration, cancelWait bool) error {
	logrus.Debug("waitCtrTerminate start")
	defer logrus.Debug("waitCtrTerminate end")

	select {
	case err := <-errCh:
		return err
	case <-time.After(timeout):
		if cancelWait {
			close(stopCh)
		}
		return errors.Errorf("stop container with signal %d timed out after %v", sig, timeout)
	}
}

// createExitsFile creates the exits file expected by podman to get the exit code
// returned by the container stop operation
func createExitsFile(ctr *Container, exitCode int) error {
	logrus.Debug("createExitsFile start")
	defer logrus.Debug("createExitsFile end")

	exitFile, err := ctr.exitFilePath()
	if err != nil {
		return err
	}

	logrus.Debug("exit file: %s", exitFile)
	if err := ioutil.WriteFile(exitFile, []byte(strconv.Itoa(exitCode)), 0777); err != nil {
		return err
	}

	return nil
}

// getRuntimeName transforms the shimv2 executable binary name into a valid
// runtime name expected by the shimv2 runtime
func (r *Shimv2Runtime) getRuntimeName() string {
	runtime := filepath.Base(r.path)
	return strings.Replace(runtime, "-", ".", -1)
}

// setRuntimePath gets the first valid binary path as the runtime executable binary
func (r *Shimv2Runtime) setRuntimePath(name string, paths []string) error {
	for _, path := range paths {
		stat, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if !stat.Mode().IsRegular() {
			continue
		}
		r.path = path
		return nil
	}

	// Search the $PATH as last fallback
	if foundRuntime, err := exec.LookPath(name); err == nil {
		r.path = foundRuntime
		return nil
	}

	return errors.Wrapf(define.ErrInvalidArg, "no valid executable found for Shimv2 runtime %s", name)
}

// connectToShimv2runtime creates a client for running container operations against
// the shimv2 runtime
func (r *Shimv2Runtime) connectToShimv2runtime(ctr *Container, address string) error {
	// if address is empty, lets read it from the address file
	if len(address) == 0 {
		addressFile := filepath.Join(ctr.bundlePath(), shimv2AddressFileName)
		b, err := ioutil.ReadFile(addressFile)
		if err != nil {
			return err
		}
		address = string(b)
		logrus.Debugf("shimv2 runtime socket address %s", address)
	}

	conn, err := client.Connect(address, client.AnonDialer)
	if err != nil {
		return err
	}

	options := ttrpc.WithOnClose(func() { conn.Close() })
	client := ttrpc.NewClient(conn, options)

	r.client = client
	r.task = task.NewTaskClient(client)

	return nil
}

func (r *Shimv2Runtime) wait(ctrID, execID string) error {
	if _, err := r.task.Wait(r.ctx, &task.WaitRequest{
		ID:     ctrID,
		ExecID: execID,
	}); err != nil {
		logrus.Errorf("error running task wait: %v", err)
		if !errors.Is(err, ttrpc.ErrClosed) {
			return errdefs.FromGRPC(err)
		}
		return errdefs.ErrNotFound
	}

	return nil
}

func (r *Shimv2Runtime) Name() string {
	return r.name
}

func (r *Shimv2Runtime) Path() string {
	return r.path
}

func (r *Shimv2Runtime) CreateContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (retErr error) {
	logrus.Debug("CreateContainer start")
	defer logrus.Debug("CreateContainer end")

	// make run dir accessible to the current user so that the PID files can be read without
	// being in the rootless user namespace
	if err := makeAccessible(ctr.state.RunDir, 0, 0); err != nil {
		return err
	}

	// create log file expected by shimv2 runtime
	f, err := fifo.OpenFifo(r.ctx, filepath.Join(ctr.bundlePath(), "log"),
		unix.O_RDONLY|unix.O_CREAT|unix.O_NONBLOCK, 0o700)
	if err != nil {
		return err
	}

	// copy shimv2 runtime logs to stderr
	go func() {
		defer f.Close()
		if _, err := io.Copy(os.Stderr, f); err != nil {
			logrus.WithError(err).Error("copy shimv2 runtime logs")
		}
	}()

	// create shimv2 runtime
	args := []string{"-id", ctr.ID(), "start"}
	// enable debug in shimv2 runtime
	switch logrus.GetLevel() {
	case logrus.DebugLevel, logrus.TraceLevel:
		args = append([]string{"-debug"}, args...)
	}
	cmd, err := client.Command(r.ctx, r.getRuntimeName(), "", "", ctr.bundlePath(), nil, args...)
	if err != nil {
		return err
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}

	// get shimv2 runtime PID
	b, err := ioutil.ReadFile(filepath.Join(ctr.bundlePath(), shimv2PIDFileName))
	if err != nil {
		return errors.Wrapf(err, "cannot read shimv2 runtime PID")
	}
	shimv2RuntimePID, err := strconv.Atoi(string(b))
	if err != nil {
		return errors.Wrapf(err, "cannot convert shimv2 runtime PID to int")
	}
	ctr.state.ConmonPID = shimv2RuntimePID

	// connect to shimv2 runtime
	address := strings.TrimSpace(string(out))
	if err := r.connectToShimv2runtime(ctr, address); err != nil {
		return errors.Wrap(err, "cannot connect to shimv2 runtime")
	}

	// create container IO
	containerIO, err := cio.NewContainerIO(ctr.ID(),
		cio.WithNewFIFOs(ctr.config.StaticDir, ctr.config.Spec.Process.Terminal, ctr.config.Stdin))
	if err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			containerIO.Close()
		}
	}()

	// redirect container creation output to log
	if len(ctr.config.LogPath) == 0 {
		ctr.config.LogPath = filepath.Join(ctr.config.StaticDir, "ctr.log")
	}
	f, err = os.OpenFile(ctr.LogPath(), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}

	var stdoutCh, stderrCh <-chan struct{}
	wc := cioutil.NewSerialWriteCloser(f)
	stdout, stdoutCh := cio.NewCRILogger(ctr.LogPath(), wc, cio.Stdout, -1)
	stderr, stderrCh := cio.NewCRILogger(ctr.LogPath(), wc, cio.Stderr, -1)

	go func() {
		if stdoutCh != nil {
			<-stdoutCh
		}
		if stderrCh != nil {
			<-stderrCh
		}
		logrus.Debugf("finish redirecting log file %s, closing it", ctr.LogPath())
		f.Close()
	}()

	containerIO.AddOutput(ctr.LogPath(), stdout, stderr)
	containerIO.Pipe()

	r.Lock()
	r.ctrs[ctr.ID()] = containerInfo{
		cio: containerIO,
	}
	r.Unlock()

	defer func() {
		if retErr != nil {
			logrus.Debugf("cleaning up container %s: %v", ctr.ID(), err)
			if cleanupErr := r.DeleteContainer(ctr); cleanupErr != nil {
				logrus.Debugf("DeleteContainer failed for container %s: %v", ctr.ID(), cleanupErr)
			}
		}
	}()

	// create container
	request := &task.CreateTaskRequest{
		ID:       ctr.ID(),
		Bundle:   ctr.bundlePath(),
		Stdin:    containerIO.Config().Stdin,
		Stdout:   containerIO.Config().Stdout,
		Stderr:   containerIO.Config().Stderr,
		Terminal: containerIO.Config().Terminal,
	}

	createdCh := make(chan error)
	go func() {
		if resp, err := r.task.Create(r.ctx, request); err != nil {
			createdCh <- errdefs.FromGRPC(err)
		} else {
			ctr.state.PID = int(resp.Pid)
		}
		close(createdCh)
	}()

	// check container creation
	select {
	case err = <-createdCh:
		if err != nil {
			return errors.Errorf("container creation failed: %v", err)
		}
	case <-time.After(define.ContainerCreateTimeout):
		if err := r.DeleteContainer(ctr); err != nil {
			return err
		}
		<-createdCh
		return errors.Errorf("container creation timeout (%v)", define.ContainerCreateTimeout)
	}

	return nil
}

func (r *Shimv2Runtime) StartContainer(ctr *Container) error {
	logrus.Debug("StartContainer start")
	defer logrus.Debug("StartContainer end")

	if r.task == nil {
		if err := r.connectToShimv2runtime(ctr, ""); err != nil {
			return errors.Wrapf(err, "cannot connect to shimv2 runtime")
		}
	}

	if _, err := r.task.Start(r.ctx, &task.StartRequest{
		ID:     ctr.ID(),
		ExecID: "",
	}); err != nil {
		return errdefs.FromGRPC(err)
	}

	if err := r.UpdateContainerStatus(ctr); err != nil {
		logrus.Debugf("error updating container state: %v", err)
	}

	return nil
}

func (r *Shimv2Runtime) UpdateContainerStatus(ctr *Container) error {
	logrus.Debug("updateContainerStatus start")
	defer logrus.Debug("updateContainerStatus end")

	if r.task == nil {
		if err := r.connectToShimv2runtime(ctr, ""); err != nil {
			return errors.Wrapf(err, "cannot connect to shimv2 runtime")
		}
	}

	response, err := r.task.State(r.ctx, &task.StateRequest{
		ID: ctr.ID(),
	})
	if err != nil {
		if !errors.Is(err, ttrpc.ErrClosed) {
			return errdefs.FromGRPC(err)
		}
		return errdefs.ErrNotFound
	}

	exitCode := int32(response.ExitStatus)
	switch response.Status {
	case tasktypes.StatusCreated:
		ctr.state.State = define.ContainerStateCreated
	case tasktypes.StatusRunning:
		ctr.state.State = define.ContainerStateRunning
		ctr.state.StartedTime = time.Now()
	case tasktypes.StatusStopped:
		ctr.state.State = define.ContainerStateStopped
		ctr.state.FinishedTime = time.Now()
		ctr.state.ExitCode = exitCode
		// Write an event for the container's death
		ctr.newContainerExitedEvent(ctr.state.ExitCode)
	case tasktypes.StatusPaused:
		ctr.state.State = define.ContainerStatePaused
	default:
		return errors.Wrapf(define.ErrInternal, "unrecognized state returned for container %s: %s",
			ctr.ID(), response.Status)
	}

	if exitCode != 0 {
		oomFilePath := filepath.Join(ctr.bundlePath(), "oom")
		if _, err = os.Stat(oomFilePath); err == nil {
			ctr.state.OOMKilled = true
		}
	}
	return nil
}

func (r *Shimv2Runtime) ExitFilePath(ctr *Container) (string, error) {
	if ctr == nil {
		return "", errors.Wrapf(define.ErrInvalidArg, "must provide a valid container to get exit file path")
	}
	return filepath.Join(r.exitsDir, ctr.ID()), nil
}

// Not needed by the shimv2 runtime but it's currently used as part of the container init workflow
func (r *Shimv2Runtime) AttachSocketPath(ctr *Container) (string, error) {
	if ctr == nil {
		return "", errors.Wrapf(define.ErrInvalidArg, "must provide a valid container to get attach socket path")
	}

	return filepath.Join(ctr.bundlePath(), "attach"), nil
}

func (r *Shimv2Runtime) DeleteContainer(ctr *Container) error {
	logrus.Debug("DeleteContainer start")
	defer logrus.Debug("DeleteContainer end")

	if r.task == nil {
		if err := r.connectToShimv2runtime(ctr, ""); err != nil {
			return errors.Wrapf(err, "cannot connect to shimv2 runtime")
		}
	}

	// run delete task
	if _, err := r.task.Delete(r.ctx, &task.DeleteRequest{
		ID:     ctr.ID(),
		ExecID: "",
	}); err != nil && !errors.Is(err, ttrpc.ErrClosed) {
		return errdefs.FromGRPC(err)
	}

	// run shutdown task
	_, err := r.task.Shutdown(r.ctx, &task.ShutdownRequest{
		ID: ctr.ID(),
	})
	if err != nil && !errors.Is(err, ttrpc.ErrClosed) {
		return err
	}

	return nil
}

func (r *Shimv2Runtime) StopContainer(ctr *Container, timeout uint, all bool) error {
	logrus.Debug("StopContainer start")
	defer logrus.Debug("StopContainer end")

	ctrID := ctr.ID()

	// update container state once it has been stopped
	defer func() {
		exitCode := -1
		if err := r.UpdateContainerStatus(ctr); err != nil {
			logrus.Errorf("error updating container state: %v", err)
		} else {
			exitCode = int(ctr.state.ExitCode)
		}

		// podman expects a /run/libpod/exits/ctrID file with an exit code
		if err := createExitsFile(ctr, exitCode); err != nil {
			logrus.Errorf("error creating exits file: %v", err)
		}
	}()

	if r.task == nil {
		if err := r.connectToShimv2runtime(ctr, ""); err != nil {
			return errors.Wrapf(err, "cannot connect to shimv2 runtime")
		}
	}

	// wait for container to be stopped
	errCh := make(chan error)
	stopCh := make(chan struct{})
	go func() {
		select {
		case <-stopCh:
			return
		default:
			// let's just ignore the error
			err := r.wait(ctrID, "")
			if err != nil && !errors.Is(err, errdefs.ErrNotFound) {
				errCh <- errdefs.FromGRPC(err)
			}

			close(errCh)
		}
	}()

	// get stop signal from config
	stopSignal := ctr.config.StopSignal
	if stopSignal == 0 {
		stopSignal = uint(syscall.SIGTERM)
	}

	if timeout > 0 {
		logrus.Debugf("kill container with signal %d and timeout %v", stopSignal, timeout)
		if err := r.KillContainer(ctr, stopSignal, all); err != nil {
			return errors.Wrapf(err, "error sending signal %d to container %s", stopSignal, ctr.ID())
		}

		// give runtime a few seconds to do the friendly termination
		if err := waitCtrTerminate(stopSignal, stopCh, errCh, time.Duration(timeout)*time.Second, false); err != nil {
			logrus.Debugf("timed out stopping container %s: %v", ctr.ID(), err)
		} else {
			return nil
		}
	}

	stopSignal = 9
	logrus.Debugf("kill container with signal %d and default timeout %v", stopSignal, killContainerTimeout)
	if err := r.KillContainer(ctr, stopSignal, all); err != nil {
		return errors.Wrapf(err, "error sending signal %d to container %s", stopSignal, ctr.ID())
	}

	// give runtime a few seconds to do the hard termination
	if err := waitCtrTerminate(stopSignal, stopCh, errCh, killContainerTimeout, true); err != nil {
		return errors.Wrapf(err, "error waiting for container termination")
	}

	return nil
}

func (r *Shimv2Runtime) KillContainer(ctr *Container, signal uint, all bool) error {
	logrus.Debug("KillContainer start")
	defer logrus.Debug("KillContainer end")

	if r.task == nil {
		if err := r.connectToShimv2runtime(ctr, ""); err != nil {
			return errors.Wrapf(err, "cannot connect to shimv2 runtime")
		}
	}

	if _, err := r.task.Kill(r.ctx, &task.KillRequest{
		ID:     ctr.ID(),
		ExecID: "",
		Signal: uint32(signal),
		All:    all,
	}); err != nil {
		logrus.Error(err)
		return errdefs.FromGRPC(err)
	}

	return nil
}

// AttachContainer attaches stdio to container stdio
func (r *Shimv2Runtime) AttachContainer(ctr *Container, inputStream io.Reader, outputStream, errorStream io.WriteCloser, tty bool) error {
	logrus.Debug("AttachContainer start")
	defer logrus.Debug("AttachContainer end")

	if r.task == nil {
		if err := r.connectToShimv2runtime(ctr, ""); err != nil {
			return errors.Wrapf(err, "cannot connect to shimv2 runtime")
		}
	}

	r.Lock()
	cInfo, ok := r.ctrs[ctr.ID()]
	r.Unlock()
	if !ok {
		return errors.New("Could not retrieve container information")
	}

	opts := cio.AttachOptions{
		Stdin:     inputStream,
		Stdout:    outputStream,
		Stderr:    errorStream,
		Tty:       tty,
		StdinOnce: ctr.Stdin(),
		CloseStdin: func() error {
			return r.closeIO(ctr.ID(), "")
		},
	}

	cInfo.cio.Attach(opts)

	return nil
}

func (r *Shimv2Runtime) closeIO(ctrID, execID string) error {
	_, err := r.task.CloseIO(r.ctx, &task.CloseIORequest{
		ID:     ctrID,
		ExecID: execID,
		Stdin:  true,
	})
	if err != nil {
		return errdefs.FromGRPC(err)
	}

	return nil
}

// Not supported or implemented functions at the moment by the Shimv2 runtime

func (r *Shimv2Runtime) CheckConmonRunning(ctr *Container) (bool, error) {
	return true, nil
}

func (r *Shimv2Runtime) PauseContainer(ctr *Container) error {
	return fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) UnpauseContainer(ctr *Container) error {
	return fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) HTTPAttach(ctr *Container, req *http.Request, w http.ResponseWriter, streams *HTTPAttachStreams, detachKeys *string, cancel <-chan bool, hijackDone chan<- bool, streamAttach, streamLogs bool) (deferredErr error) {
	return fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) AttachResize(ctr *Container, newSize define.TerminalSize) error {
	return fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) CheckpointContainer(ctr *Container, options ContainerCheckpointOptions) error {
	return fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) SupportsCheckpoint() bool {
	return false
}

func (r *Shimv2Runtime) SupportsJSONErrors() bool {
	return false
}

func (r *Shimv2Runtime) SupportsNoCgroups() bool {
	return false
}

func (r *Shimv2Runtime) SupportsKVM() bool {
	return false
}

func (r *Shimv2Runtime) RuntimeInfo() (*define.ConmonInfo, *define.OCIRuntimeInfo, error) {
	return nil, nil, fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) ExecContainerHTTP(ctr *Container, sessionID string, options *ExecOptions, req *http.Request, w http.ResponseWriter, streams *HTTPAttachStreams, cancel <-chan bool, hijackDone chan<- bool, holdConnOpen <-chan bool, newSize *define.TerminalSize) (int, chan error, error) {
	return 0, nil, fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) ExecContainer(c *Container, sessionID string, options *ExecOptions, streams *define.AttachStreams, newSize *define.TerminalSize) (int, chan error, error) {
	return 0, nil, fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) ExecContainerDetached(ctr *Container, sessionID string, options *ExecOptions, stdin bool) (int, error) {
	return 0, fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) ExecAttachResize(ctr *Container, sessionID string, newSize define.TerminalSize) error {
	return fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) ExecStopContainer(ctr *Container, sessionID string, timeout uint) error {
	return fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) ExecUpdateStatus(ctr *Container, sessionID string) (bool, error) {
	return false, fmt.Errorf("not supported")
}

func (r *Shimv2Runtime) ExecAttachSocketPath(ctr *Container, sessionID string) (string, error) {
	return "", fmt.Errorf("not supported")
}
