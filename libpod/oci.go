package libpod

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/containerd/cgroups"
	spec "github.com/opencontainers/runtime-spec/specs-go"
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

// CreateContainer creates a container in the OCI runtime
// TODO terminal support for container
// Presently just ignoring conmon opts related to it
func (r *OCIRuntime) createContainer(ctr *Container, cgroupParent string) error {
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
	// TODO container log location should be configurable
	// The default also likely shouldn't be this
	args = append(args, "-l", filepath.Join(ctr.config.StaticDir, "ctr.log"))
	args = append(args, "--exit-dir", r.exitsDir)
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
	logrus.WithFields(logrus.Fields{
		"args": args,
	}).Debugf("running conmon: %s", r.conmonPath)

	cmd := exec.Command(r.conmonPath, args...)
	cmd.Dir = ctr.state.RunDir
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
		logrus.Infof("Running conmon under slice %s and unitName %s", cgroupParent, createUnitName("libpod-conmon", ctr.ID()))
		if err = utils.RunUnderSystemdScope(cmd.Process.Pid, cgroupParent, createUnitName("libpod-conmon", ctr.ID())); err != nil {
			logrus.Warnf("Failed to add conmon to systemd sandbox cgroup: %v", err)
		}
	} else {
		control, err := cgroups.New(cgroups.V1, cgroups.StaticPath(filepath.Join(cgroupParent, "/libpod-conmon-"+ctr.ID())), &spec.LinuxResources{})
		if err != nil {
			logrus.Warnf("Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
		} else {
			// XXX: this defer does nothing as the cgroup can't be deleted cause
			// it contains the conmon pid in tasks
			// we need to remove this defer and delete the cgroup once conmon exits
			// maybe need a conmon monitor?
			defer control.Delete()
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

	// TODO should do a defer r.deleteContainer(ctr) here if err != nil
	// Need deleteContainer to be working first, though...

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

	out, err := exec.Command(r.path, "state", ctr.ID()).CombinedOutput()
	if err != nil {
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
		return errors.Wrapf(ErrInternal, "unrecognized status returned by runc for container %s: %s",
			ctr.ID(), state.Status)
	}

	if ctr.state.State == ContainerStateStopped {
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
			return errors.Wrapf(err, "no exit file for container %s found", ctr.ID())
		}

		ctr.state.FinishedTime = getFinishedTime(fi)
		statusCodeStr, err := ioutil.ReadFile(exitFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read exit file for container %s", ctr.ID())
		}
		statusCode, err := strconv.Atoi(string(statusCodeStr))
		if err != nil {
			return errors.Wrapf(err, "error convertaing exit status code for container %s to int",
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
