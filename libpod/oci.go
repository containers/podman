package libpod

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/util"
	"github.com/cri-o/ocicni/pkg/ocicni"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	// TODO import these functions into libpod and remove the import
	// Trying to keep libpod from depending on CRI-O code
	"github.com/containers/libpod/utils"
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
	reservePorts  bool
	supportsJSON  bool
}

// ociError is used to parse the OCI runtime JSON log.  It is not part of the
// OCI runtime specifications, it follows what runc does
type ociError struct {
	Level string `json:"level,omitempty"`
	Time  string `json:"time,omitempty"`
	Msg   string `json:"msg,omitempty"`
}

// Make a new OCI runtime with provided options.
// The first path that points to a valid executable will be used.
func newOCIRuntime(name string, paths []string, conmonPath string, runtimeCfg *RuntimeConfig, supportsJSON bool) (*OCIRuntime, error) {
	if name == "" {
		return nil, errors.Wrapf(define.ErrInvalidArg, "the OCI runtime must be provided a non-empty name")
	}

	runtime := new(OCIRuntime)
	runtime.name = name
	runtime.conmonPath = conmonPath

	runtime.conmonEnv = runtimeCfg.ConmonEnvVars
	runtime.cgroupManager = runtimeCfg.CgroupManager
	runtime.tmpDir = runtimeCfg.TmpDir
	runtime.logSizeMax = runtimeCfg.MaxLogSize
	runtime.noPivot = runtimeCfg.NoPivotRoot
	runtime.reservePorts = runtimeCfg.EnablePortReservation

	// TODO: probe OCI runtime for feature and enable automatically if
	// available.
	runtime.supportsJSON = supportsJSON

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
		break
	}
	if !foundPath {
		return nil, errors.Wrapf(define.ErrInvalidArg, "no valid executable found for OCI runtime %s", name)
	}

	runtime.exitsDir = filepath.Join(runtime.tmpDir, "exits")
	runtime.socketsDir = filepath.Join(runtime.tmpDir, "socket")

	if runtime.cgroupManager != CgroupfsCgroupsManager && runtime.cgroupManager != SystemdCgroupsManager {
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

// Create systemd unit name for cgroup scopes
func createUnitName(prefix string, name string) string {
	return fmt.Sprintf("%s-%s.scope", prefix, name)
}

func bindPorts(ports []ocicni.PortMapping) ([]*os.File, error) {
	var files []*os.File
	notifySCTP := false
	for _, i := range ports {
		switch i.Protocol {
		case "udp":
			addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", i.HostIP, i.HostPort))
			if err != nil {
				return nil, errors.Wrapf(err, "cannot resolve the UDP address")
			}

			server, err := net.ListenUDP("udp", addr)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot listen on the UDP port")
			}
			f, err := server.File()
			if err != nil {
				return nil, errors.Wrapf(err, "cannot get file for UDP socket")
			}
			files = append(files, f)

		case "tcp":
			addr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", i.HostIP, i.HostPort))
			if err != nil {
				return nil, errors.Wrapf(err, "cannot resolve the TCP address")
			}

			server, err := net.ListenTCP("tcp4", addr)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot listen on the TCP port")
			}
			f, err := server.File()
			if err != nil {
				return nil, errors.Wrapf(err, "cannot get file for TCP socket")
			}
			files = append(files, f)
		case "sctp":
			if !notifySCTP {
				notifySCTP = true
				logrus.Warnf("port reservation for SCTP is not supported")
			}
		default:
			return nil, fmt.Errorf("unknown protocol %s", i.Protocol)

		}
	}
	return files, nil
}

// updateContainerStatus retrieves the current status of the container from the
// runtime. It updates the container's state but does not save it.
// If useRunc is false, we will not directly hit runc to see the container's
// status, but will instead only check for the existence of the conmon exit file
// and update state to stopped if it exists.
func (r *OCIRuntime) updateContainerStatus(ctr *Container, useRuntime bool) error {
	exitFile := ctr.exitFilePath()

	runtimeDir, err := util.GetRootlessRuntimeDir()
	if err != nil {
		return err
	}

	// If not using the OCI runtime, we don't need to do most of this.
	if !useRuntime {
		// If the container's not running, nothing to do.
		if ctr.state.State != define.ContainerStateRunning && ctr.state.State != define.ContainerStatePaused {
			return nil
		}

		// Check for the exit file conmon makes
		info, err := os.Stat(exitFile)
		if err != nil {
			if os.IsNotExist(err) {
				// Container is still running, no error
				return nil
			}

			return errors.Wrapf(err, "error running stat on container %s exit file", ctr.ID())
		}

		// Alright, it exists. Transition to Stopped state.
		ctr.state.State = define.ContainerStateStopped
		ctr.state.PID = 0
		ctr.state.ConmonPID = 0

		// Read the exit file to get our stopped time and exit code.
		return ctr.handleExitFile(exitFile, info)
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

// startContainer starts the given container
// Sets time the container was started, but does not save it.
func (r *OCIRuntime) startContainer(ctr *Container) error {
	// TODO: streams should probably *not* be our STDIN/OUT/ERR - redirect to buffers?
	runtimeDir, err := util.GetRootlessRuntimeDir()
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

// killContainer sends the given signal to the given container
func (r *OCIRuntime) killContainer(ctr *Container, signal uint) error {
	logrus.Debugf("Sending signal %d to container %s", signal, ctr.ID())
	runtimeDir, err := util.GetRootlessRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, "kill", ctr.ID(), fmt.Sprintf("%d", signal)); err != nil {
		return errors.Wrapf(err, "error sending signal to container %s", ctr.ID())
	}

	return nil
}

// deleteContainer deletes a container from the OCI runtime
func (r *OCIRuntime) deleteContainer(ctr *Container) error {
	runtimeDir, err := util.GetRootlessRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, "delete", "--force", ctr.ID())
}

// pauseContainer pauses the given container
func (r *OCIRuntime) pauseContainer(ctr *Container) error {
	runtimeDir, err := util.GetRootlessRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, "pause", ctr.ID())
}

// unpauseContainer unpauses the given container
func (r *OCIRuntime) unpauseContainer(ctr *Container) error {
	runtimeDir, err := util.GetRootlessRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, "resume", ctr.ID())
}

// checkpointContainer checkpoints the given container
func (r *OCIRuntime) checkpointContainer(ctr *Container, options ContainerCheckpointOptions) error {
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

func (r *OCIRuntime) featureCheckCheckpointing() bool {
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
