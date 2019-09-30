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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/containers/libpod/pkg/lookup"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/libpod/utils"
	"github.com/coreos/go-systemd/activation"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// createOCIContainer generates this container's main conmon instance and prepares it for starting
func (r *OCIRuntime) createOCIContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (err error) {
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
	if err := r.moveConmonToCgroupAndSignal(ctr, cmd, parentStartPipe, ctr.ID()); err != nil {
		return err
	}
	/* Wait for initial setup and fork, and reap child */
	err = cmd.Wait()
	if err != nil {
		return err
	}

	pid, err := readConmonPipeData(parentSyncPipe, ociLog)
	if err != nil {
		if err2 := r.deleteContainer(ctr); err2 != nil {
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
func (r *OCIRuntime) configureConmonEnv(runtimeDir string) ([]string, []*os.File, error) {
	env := make([]string, 0, 6)
	env = append(env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir))
	env = append(env, fmt.Sprintf("_CONTAINERS_USERNS_CONFIGURED=%s", os.Getenv("_CONTAINERS_USERNS_CONFIGURED")))
	env = append(env, fmt.Sprintf("_CONTAINERS_ROOTLESS_UID=%s", os.Getenv("_CONTAINERS_ROOTLESS_UID")))
	home, err := homeDir()
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
func (r *OCIRuntime) sharedConmonArgs(ctr *Container, cuuid, bundlePath, pidPath, logPath, exitDir, ociLogPath string) []string {
	// set the conmon API version to be able to use the correct sync struct keys
	args := []string{"--api-version", "1"}
	if r.cgroupManager == SystemdCgroupsManager && !ctr.config.NoCgroups {
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
func (r *OCIRuntime) moveConmonToCgroupAndSignal(ctr *Container, cmd *exec.Cmd, startFd *os.File, uuid string) error {
	mustCreateCgroup := true
	// If cgroup creation is disabled - just signal.
	if ctr.config.NoCgroups {
		mustCreateCgroup = false
	}

	if rootless.IsRootless() {
		ownsCgroup, err := cgroups.UserOwnsCurrentSystemdCgroup()
		if err != nil {
			return err
		}
		mustCreateCgroup = !ownsCgroup
	}

	if mustCreateCgroup {
		cgroupParent := ctr.CgroupParent()
		if r.cgroupManager == SystemdCgroupsManager {
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
			return -1, errors.Wrapf(ss.err, "error reading container (probably exited) json message")
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
	case <-time.After(ContainerCreateTimeout):
		return -1, errors.Wrapf(define.ErrInternal, "container creation timeout")
	}
	return data, nil
}

func getOCIRuntimeError(runtimeMsg string) error {
	r := strings.ToLower(runtimeMsg)
	if match, _ := regexp.MatchString(".*permission denied.*|.*operation not permitted.*", r); match {
		return errors.Wrapf(define.ErrOCIRuntimePermissionDenied, "%s", strings.Trim(runtimeMsg, "\n"))
	}
	if match, _ := regexp.MatchString(".*executable file not found in.*|.*no such file or directory.*", r); match {
		return errors.Wrapf(define.ErrOCIRuntimeNotFound, "%s", strings.Trim(runtimeMsg, "\n"))
	}
	return errors.Wrapf(define.ErrOCIRuntime, "%s", strings.Trim(runtimeMsg, "\n"))
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
