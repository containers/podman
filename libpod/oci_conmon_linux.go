package libpod

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/utils"
	pmount "github.com/containers/storage/pkg/mount"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func (r *ConmonOCIRuntime) createRootlessContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (int64, error) {
	type result struct {
		restoreDuration int64
		err             error
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
			return nil, fmt.Errorf("error creating OCI runtime exit files directory: %w", err)
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
			defer errorhandling.CloseQuiet(fd)

			// create a new mountns on the current thread
			if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
				return 0, err
			}
			defer func() {
				if err := unix.Setns(int(fd.Fd()), unix.CLONE_NEWNS); err != nil {
					logrus.Errorf("Unable to clone new namespace: %q", err)
				}
			}()

			// don't spread our mounts around.  We are setting only /sys to be slave
			// so that the cleanup process is still able to umount the storage and the
			// changes are propagated to the host.
			err = unix.Mount("/sys", "/sys", "none", unix.MS_REC|unix.MS_SLAVE, "")
			if err != nil {
				return 0, fmt.Errorf("cannot make /sys slave: %w", err)
			}

			mounts, err := pmount.GetMounts()
			if err != nil {
				return 0, err
			}
			for _, m := range mounts {
				if !strings.HasPrefix(m.Mountpoint, "/sys/kernel") {
					continue
				}
				err = unix.Unmount(m.Mountpoint, 0)
				if err != nil && !os.IsNotExist(err) {
					return 0, fmt.Errorf("cannot unmount %s: %w", m.Mountpoint, err)
				}
			}
			return r.createOCIContainer(ctr, restoreOptions)
		}()
		ch <- result{
			restoreDuration: restoreDuration,
			err:             err,
		}
	}()
	res := <-ch
	return res.restoreDuration, res.err
}

// Run the closure with the container's socket label set
func (r *ConmonOCIRuntime) withContainerSocketLabel(ctr *Container, closure func() error) error {
	runtime.LockOSThread()
	if err := label.SetSocketLabel(ctr.ProcessLabel()); err != nil {
		return err
	}
	err := closure()
	// Ignore error returned from SetSocketLabel("") call,
	// can't recover.
	if labelErr := label.SetSocketLabel(""); labelErr == nil {
		// Unlock the thread only if the process label could be restored
		// successfully.  Otherwise leave the thread locked and the Go runtime
		// will terminate it once it returns to the threads pool.
		runtime.UnlockOSThread()
	} else {
		logrus.Errorf("Unable to reset socket label: %q", labelErr)
	}

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
		return false, fmt.Errorf("error pinging container %s conmon with signal 0: %w", ctr.ID(), err)
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
	runtimePackage := packageVersion(r.path)
	conmonPackage := packageVersion(r.conmonPath)
	runtimeVersion, err := r.getOCIRuntimeVersion()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting version of OCI runtime %s: %w", r.name, err)
	}
	conmonVersion, err := r.getConmonVersion()
	if err != nil {
		return nil, nil, fmt.Errorf("error getting conmon version: %w", err)
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
					logrus.Errorf("Pinging PID %d with signal 0: %v", pid, err)
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
		return fmt.Errorf("given PIDs did not die within timeout")
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
		return 0, fmt.Errorf("error creating socket pair: %w", err)
	}
	defer errorhandling.CloseQuiet(parentSyncPipe)

	childStartPipe, parentStartPipe, err := newPipe()
	if err != nil {
		return 0, fmt.Errorf("error creating socket pair for start pipe: %w", err)
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

	if ctr.config.SdNotifyMode == define.SdNotifyModeContainer && ctr.notifySocket != "" {
		args = append(args, fmt.Sprintf("--sdnotify-socket=%s", ctr.notifySocket))
	}

	if ctr.config.Spec.Process.Terminal {
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
	if ctr.config.Spec.Process.Terminal {
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
	conf, ok := os.LookupEnv("CONTAINERS_CONF")
	if ok {
		env = append(env, fmt.Sprintf("CONTAINERS_CONF=%s", conf))
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
	default: //nolint:stylecheck,gocritic
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

	if logLevel == logrus.DebugLevel {
		logrus.Debugf("%s messages will be logged to syslog", r.conmonPath)
		args = append(args, "--syslog")
	}

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
	// Make sure to unset the NOTIFY_SOCKET and reset if afterwards if needed.
	// NOTE: going to have to wire this into conmon-rs as well
	switch ctr.config.SdNotifyMode {
	case define.SdNotifyModeContainer, define.SdNotifyModeIgnore:
		if ctr.notifySocket != "" {
			if err := os.Unsetenv("NOTIFY_SOCKET"); err != nil {
				logrus.Warnf("Error unsetting NOTIFY_SOCKET %v", err)
			}

			defer func() {
				if err := os.Setenv("NOTIFY_SOCKET", ctr.notifySocket); err != nil {
					logrus.Errorf("Resetting NOTIFY_SOCKET=%s", ctr.notifySocket)
				}
			}()
		}
	}

	return cmd.Start()
}

// moveConmonToCgroupAndSignal gets a container's cgroupParent and moves the conmon process to that cgroup
// it then signals for conmon to start by sending nonce data down the start fd
func (r *ConmonOCIRuntime) moveConmonToCgroupAndSignal(ctr *Container, cmd *exec.Cmd, startFd *os.File) error {
	mustCreateCgroup := true

	if ctr.config.NoCgroups {
		mustCreateCgroup = false
	}

	// If cgroup creation is disabled - just signal.
	switch ctr.config.CgroupsMode {
	case "disabled", "no-conmon", cgroupSplit:
		mustCreateCgroup = false
	}

	// $INVOCATION_ID is set by systemd when running as a service.
	if ctr.runtime.RemoteURI() == "" && os.Getenv("INVOCATION_ID") != "" {
		mustCreateCgroup = false
	}

	if mustCreateCgroup {
		// Usually rootless users are not allowed to configure cgroupfs.
		// There are cases though, where it is allowed, e.g. if the cgroup
		// is manually configured and chowned).  Avoid detecting all
		// such cases and simply use a lower log level.
		logLevel := logrus.WarnLevel
		if rootless.IsRootless() {
			logLevel = logrus.InfoLevel
		}
		// TODO: This should be a switch - we are not guaranteed that
		// there are only 2 valid cgroup managers
		cgroupParent := ctr.CgroupParent()
		cgroupPath := filepath.Join(ctr.config.CgroupParent, "conmon")
		cgroupResources, err := GetLimits(ctr.LinuxResources())
		if err != nil {
			logrus.StandardLogger().Log(logLevel, "Could not get ctr resources")
		}
		if ctr.CgroupManager() == config.SystemdCgroupsManager {
			unitName := createUnitName("libpod-conmon", ctr.ID())
			realCgroupParent := cgroupParent
			splitParent := strings.Split(cgroupParent, "/")
			if strings.HasSuffix(cgroupParent, ".slice") && len(splitParent) > 1 {
				realCgroupParent = splitParent[len(splitParent)-1]
			}

			logrus.Infof("Running conmon under slice %s and unitName %s", realCgroupParent, unitName)
			if err := utils.RunUnderSystemdScope(cmd.Process.Pid, realCgroupParent, unitName); err != nil {
				logrus.StandardLogger().Logf(logLevel, "Failed to add conmon to systemd sandbox cgroup: %v", err)
			}
		} else {
			control, err := cgroups.New(cgroupPath, &cgroupResources)
			if err != nil {
				logrus.StandardLogger().Logf(logLevel, "Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
			} else if err := control.AddPid(cmd.Process.Pid); err != nil {
				// we need to remove this defer and delete the cgroup once conmon exits
				// maybe need a conmon monitor?
				logrus.StandardLogger().Logf(logLevel, "Failed to add conmon to cgroupfs sandbox cgroup: %v", err)
			}
		}
	}

	/* We set the cgroup, now the child can start creating children */
	if err := writeConmonPipeData(startFd); err != nil {
		return err
	}
	return nil
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

	data := -1 //nolint: wastedassign
	select {
	case ss := <-ch:
		if ss.err != nil {
			if ociLog != "" {
				ociLogData, err := ioutil.ReadFile(ociLog)
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
				ociLogData, err := ioutil.ReadFile(ociLog)
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
