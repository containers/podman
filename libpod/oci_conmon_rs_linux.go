//go:build linux
// +build linux

package libpod

import (
	// "bufio"
	"bytes"
	// "context"
	"fmt"
	// "io"
	// "io/ioutil"
	// "net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	// "sync"
	// "syscall"
	"text/template"
	"time"

	"github.com/containers/common/pkg/cgroups"
	"github.com/containers/common/pkg/config"
	conmonClient "github.com/containers/conmon-rs/pkg/client"
	// conmonConfig "github.com/containers/conmon/runner/config"
	"github.com/containers/common/pkg/resize"
	"github.com/containers/podman/v4/libpod/define"
	// "github.com/containers/podman/v4/libpod/logs"
	// "github.com/containers/podman/v4/pkg/checkpoint/crutils"
	"github.com/containers/podman/v4/pkg/errorhandling"
	"github.com/containers/podman/v4/pkg/rootless"
	// "github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage/pkg/homedir"
	pmount "github.com/containers/storage/pkg/mount"
	// spec "github.com/opencontainers/runtime-spec/specs-go"
	// "github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
)

// ConmonRsOCIRuntime is an OCI runtime managed by Conmon-rs.
type ConmonRsOCIRuntime struct {
	name              string
	path              string
	conmonPath        string
	conmonEnv         []string //nolint:unused
	tmpDir            string   //nolint:unused
	exitsDir          string
	logSizeMax        int64 //nolint:unused
	noPivot           bool  //nolint:unused
	reservePorts      bool
	runtimeFlags      []string
	supportsJSON      bool //nolint:unused
	supportsKVM       bool //nolint:unused
	supportsNoCgroups bool //nolint:unused
	enableKeyring     bool //nolint:unused
}

// Make a new Conmon-based OCI runtime with the given options.
// Conmon will wrap the given OCI runtime, which can be `runc`, `crun`, or
// any runtime with a runc-compatible CLI.
// The first path that points to a valid executable will be used.
// Deliberately private. Someone should not be able to construct this outside of
// libpod.
func newConmonRsOCIRuntime(name string, paths []string, conmonPath string, runtimeFlags []string, runtimeCfg *config.Config) (OCIRuntime, error) { //nolint:deadcode, unused
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

	runtime := new(ConmonRsOCIRuntime)
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
		logrus.Tracef("found runtime %q", runtime.path)
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
func (r *ConmonRsOCIRuntime) Name() string {
	return r.name
}

// Path returns the path of the OCI runtime being wrapped by Conmon.
func (r *ConmonRsOCIRuntime) Path() string {
	return r.path
}

// CreateContainer creates a container.
func (r *ConmonRsOCIRuntime) CreateContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (int64, error) {
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

		// if we are running a non-privileged container, be sure to unmount some kernel paths so they are not
		// bind mounted inside the container at all.
		if !ctr.config.Privileged && !rootless.IsRootless() {
			type result struct {
				restoreDuration int64
				err             error
			}
			ch := make(chan result)
			go func() {
				runtime.LockOSThread()
				restoreDuration, err := func() (int64, error) {
					fd, err := os.Open(fmt.Sprintf("/proc/%d/task/%d/ns/mnt", os.Getpid(), unix.Gettid()))
					if err != nil {
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
			r := <-ch
			return r.restoreDuration, r.err
		}
	}
	return r.createOCIContainer(ctr, restoreOptions)
}

func (r *ConmonRsOCIRuntime) getLogTag(ctr *Container) (string, error) { //nolint:unused
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
func (r *ConmonRsOCIRuntime) createOCIContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (int64, error) {
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

	if ctr.config.CgroupsMode == cgroupSplit {
		if err := utils.MoveUnderCgroupSubtree("runtime"); err != nil {
			return 0, err
		}
	}

	// NOTE: not used currently
	// pidfile := ctr.config.PidFile
	// if pidfile == "" {
	// pidfile = filepath.Join(ctr.state.RunDir, "pidfile")
	// }

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

	// FIXME: the log level warning is not supported, so its hardcoded as "debug"
	// logLevel := logrus.GetLevel().String()
	// if logLevel == logrus.PanicLevel.String() || logLevel == logrus.FatalLevel.String() {
	// logLevel = "off"
	// }

	client, err := conmonClient.New(&conmonClient.ConmonServerConfig{
		ConmonServerPath: r.conmonPath,
		LogLevel:         "debug",
		Runtime:          r.path,
		ServerRunDir:     ctr.state.RunDir,
		LogDriver:        conmonClient.LogDriverStdout,
	})
	if err != nil {
		return 0, fmt.Errorf("error creating conmon-rs client: %w", err)
	}

	// most arguments are going to go into here or the ConmonServerConfig. keep note of the ones that don't do anything at this point. indicative of missing features
	createConfig := &conmonClient.CreateContainerConfig{
		ID:         ctr.ID(),
		BundlePath: ctr.bundlePath(),
		ExitPaths:  []string{filepath.Join(r.exitsDir, ctr.ID())},
		LogDrivers: []conmonClient.LogDriver{
			{
				Type: conmonClient.LogDriverTypeContainerRuntimeInterface,
				Path: ctr.LogPath(),
			},
		},
	}

	var filesToClose []*os.File
	if preserveFDs > 0 {
		for fd := 3; fd < int(3+preserveFDs); fd++ {
			f := os.NewFile(uintptr(fd), fmt.Sprintf("fd-%d", fd))
			filesToClose = append(filesToClose, f)
		}
	}

	if r.reservePorts && !rootless.IsRootless() && !ctr.config.NetMode.IsSlirp4netns() {
		ports, err := bindPorts(ctr.convertPortMappings())
		if err != nil {
			return 0, err
		}
		filesToClose = append(filesToClose, ports...)
	}

	// NOTE: probably going to need some significant adaptation. just test things as root. rootless is going to be pretty broken
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

		if ctr.rootlessPortSyncW != nil {
			defer errorhandling.CloseQuiet(ctr.rootlessPortSyncW)
		}
	}
	var runtimeRestoreStarted time.Time
	if restoreOptions != nil {
		runtimeRestoreStarted = time.Now()
	}
	resp, err := client.CreateContainer(context.Background(), createConfig)
	if err != nil {
		return 0, fmt.Errorf("unable to start conmon instance: %w", err)
	}

	// regardless of whether we errored or not, we no longer need the children pipes
	childSyncPipe.Close()
	childStartPipe.Close()
	if err != nil {
		return 0, err
	}
	ctr.state.PID = int(resp.PID)

	logrus.Infof("Got Conmon PID as %d", client.PID())
	ctr.state.ConmonPID = int(client.PID())

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

// sharedConmonArgs takes common arguments for exec and create/restore and formats them for the conmon CLI
func (r *ConmonRsOCIRuntime) sharedConmonArgs(ctr *Container, cuuid, bundlePath, pidPath, logPath, exitDir, ociLogPath, logDriver, logTag string) []string { //nolint:unused
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

// DeleteContainer deletes a container from the OCI runtime.
func (r *ConmonRsOCIRuntime) DeleteContainer(ctr *Container) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, append(r.runtimeFlags, "delete", "--force", ctr.ID())...)
}

// moveConmonToCgroupAndSignal gets a container's cgroupParent and moves the conmon process to that cgroup
// it then signals for conmon to start by sending nonce data down the start fd
func (r *ConmonRsOCIRuntime) moveConmonToCgroupAndSignal(ctr *Container, cmd *exec.Cmd, startFd *os.File) error { //nolint:unused
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
		cgroupPath := filepath.Join(ctr.config.CgroupParent, "conmonrs")
		Resource := ctr.Spec().Linux.Resources
		cgroupResources, err := GetLimits(Resource)
		if err != nil {
			logrus.StandardLogger().Log(logLevel, "Could not get ctr resources")
		}
		if ctr.CgroupManager() == config.SystemdCgroupsManager {
			unitName := createUnitName("libpod-conmonrs", ctr.ID())
			realCgroupParent := cgroupParent
			splitParent := strings.Split(cgroupParent, "/")
			if strings.HasSuffix(cgroupParent, ".slice") && len(splitParent) > 1 {
				realCgroupParent = splitParent[len(splitParent)-1]
			}

			logrus.Infof("Running conmon-rs under slice %s and unitName %s", realCgroupParent, unitName)
			if err := utils.RunUnderSystemdScope(cmd.Process.Pid, realCgroupParent, unitName); err != nil {
				logrus.StandardLogger().Logf(logLevel, "Failed to add conmon-rs to systemd sandbox cgroup: %v", err)
			}
		} else {
			control, err := cgroups.New(cgroupPath, &cgroupResources)
			if err != nil {
				logrus.StandardLogger().Logf(logLevel, "Failed to add conmon-rs to cgroupfs sandbox cgroup: %v", err)
			} else if err := control.AddPid(cmd.Process.Pid); err != nil {
				// we need to remove this defer and delete the cgroup once conmon exits
				// maybe need a conmon monitor?
				logrus.StandardLogger().Logf(logLevel, "Failed to add conmon-rs to cgroupfs sandbox cgroup: %v", err)
			}
		}
	}

	/* We set the cgroup, now the child can start creating children */
	if err := writeConmonPipeData(startFd); err != nil {
		return err
	}
	return nil
}

// configureConmonEnv gets the environment values to add to conmon's exec struct
// TODO this may want to be less hardcoded/more configurable in the future
func (r *ConmonRsOCIRuntime) configureConmonEnv(runtimeDir string) []string { //nolint:unused
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

// UpdateContainerStatus updates the status of the given container.
func (r *ConmonRsOCIRuntime) UpdateContainerStatus(ctr *Container) error { return nil }

// StartContainer starts the given container.
func (r *ConmonRsOCIRuntime) StartContainer(ctr *Container) error { return nil }

// KillContainer sends the given signal to the given container.
// If all is set, all processes in the container will be signalled;
// otherwise, only init will be signalled.
func (r *ConmonRsOCIRuntime) KillContainer(ctr *Container, signal uint, all bool) error { return nil }

// StopContainer stops the given container.
// The container's stop signal (or SIGTERM if unspecified) will be sent
// first.
// After the given timeout, SIGKILL will be sent.
// If the given timeout is 0, SIGKILL will be sent immediately, and the
// stop signal will be omitted.
// If all is set, we will attempt to use the --all flag will `kill` in
// the OCI runtime to kill all processes in the container, including
// exec sessions. This is only supported if the container has cgroups.
func (r *ConmonRsOCIRuntime) StopContainer(ctr *Container, timeout uint, all bool) error { return nil }

// PauseContainer pauses the given container.
func (r *ConmonRsOCIRuntime) PauseContainer(ctr *Container) error { return nil }

// UnpauseContainer unpauses the given container.
func (r *ConmonRsOCIRuntime) UnpauseContainer(ctr *Container) error { return nil }

// Attach to a container.
func (r *ConmonRsOCIRuntime) Attach(ctr *Container, params *AttachOptions) error { return nil }

// HTTPAttach performs an attach intended to be transported over HTTP.
// For terminal attach, the container's output will be directly streamed
// to output; otherwise, STDOUT and STDERR will be multiplexed, with
// a header prepended as follows: 1-byte STREAM (0, 1, 2 for STDIN,
// STDOUT, STDERR), 3 null (0x00) bytes, 4-byte big endian length.
// If a cancel channel is provided, it can be used to asynchronously
// terminate the attach session. Detach keys, if given, will also cause
// the attach session to be terminated if provided via the STDIN
// channel. If they are not provided, the default detach keys will be
// used instead. Detach keys of "" will disable detaching via keyboard.
// The streams parameter will determine which streams to forward to the
// client.
func (r *ConmonRsOCIRuntime) HTTPAttach(ctr *Container, req *http.Request, w http.ResponseWriter, streams *HTTPAttachStreams, detachKeys *string, cancel <-chan bool, hijackDone chan<- bool, streamAttach, streamLogs bool) error {
	return nil
}

// AttachResize resizes the terminal in use by the given container.
func (r *ConmonRsOCIRuntime) AttachResize(ctr *Container, newSize resize.TerminalSize) error {
	return nil
}

// ExecContainer executes a command in a running container.
// Returns an int (PID of exec session), error channel (errors from
// attach), and error (errors that occurred attempting to start the exec
// session). This returns once the exec session is running - not once it
// has completed, as one might expect. The attach session will remain
// running, in a goroutine that will return via the chan error in the
// return signature.
// newSize resizes the tty to this size before the process is started, must be nil if the exec session has no tty
func (r *ConmonRsOCIRuntime) ExecContainer(ctr *Container, sessionID string, options *ExecOptions, streams *define.AttachStreams, newSize *resize.TerminalSize) (int, chan error, error) {
	return 0, make(chan error), nil
}

// ExecContainerHTTP executes a command in a running container and
// attaches its standard streams to a provided hijacked HTTP session.
// Maintains the same invariants as ExecContainer (returns on session
// start, with a goroutine running in the background to handle attach).
// The HTTP attach itself maintains the same invariants as HTTPAttach.
// newSize resizes the tty to this size before the process is started, must be nil if the exec session has no tty
func (r *ConmonRsOCIRuntime) ExecContainerHTTP(ctr *Container, sessionID string, options *ExecOptions, req *http.Request, w http.ResponseWriter,
	streams *HTTPAttachStreams, cancel <-chan bool, hijackDone chan<- bool, holdConnOpen <-chan bool, newSize *resize.TerminalSize) (int, chan error, error) {
	return 0, make(chan error), nil
}

// ExecContainerDetached executes a command in a running container, but
// does not attach to it. Returns the PID of the exec session and an
// error (if starting the exec session failed)
func (r *ConmonRsOCIRuntime) ExecContainerDetached(ctr *Container, sessionID string, options *ExecOptions, stdin bool) (int, error) {
	return 0, nil
}

// ExecAttachResize resizes the terminal of a running exec session. Only
// allowed with sessions that were created with a TTY.
func (r *ConmonRsOCIRuntime) ExecAttachResize(ctr *Container, sessionID string, newSize resize.TerminalSize) error {
	return nil
}

// ExecStopContainer stops a given exec session in a running container.
// SIGTERM with be sent initially, then SIGKILL after the given timeout.
// If timeout is 0, SIGKILL will be sent immediately, and SIGTERM will
// be omitted.
func (r *ConmonRsOCIRuntime) ExecStopContainer(ctr *Container, sessionID string, timeout uint) error {
	return nil
}

// ExecUpdateStatus checks the status of a given exec session.
// Returns true if the session is still running, or false if it exited.
func (r *ConmonRsOCIRuntime) ExecUpdateStatus(ctr *Container, sessionID string) (bool, error) {
	return false, nil
}

// CheckpointContainer checkpoints the given container.
// Some OCI runtimes may not support this - if SupportsCheckpoint()
// returns false, this is not implemented, and will always return an
// error. If CheckpointOptions.PrintStats is true the first return parameter
// contains the number of microseconds the runtime needed to checkpoint
// the given container.
func (r *ConmonRsOCIRuntime) CheckpointContainer(ctr *Container, options ContainerCheckpointOptions) (int64, error) {
	return 0, nil
}

// CheckConmonRunning verifies that the given container's Conmon
// instance is still running. Runtimes without Conmon, or systems where
// the PID of conmon is not available, should mock this as True.
// True indicates that Conmon for the instance is running, False
// indicates it is not.
func (r *ConmonRsOCIRuntime) CheckConmonRunning(ctr *Container) (bool, error) { return false, nil }

// SupportsCheckpoint returns whether this OCI runtime
// implementation supports the CheckpointContainer() operation.
func (r *ConmonRsOCIRuntime) SupportsCheckpoint() bool { return false }

// SupportsJSONErrors is whether the runtime can return JSON-formatted
// error messages.
func (r *ConmonRsOCIRuntime) SupportsJSONErrors() bool { return false }

// SupportsNoCgroups is whether the runtime supports running containers
// without cgroups.
func (r *ConmonRsOCIRuntime) SupportsNoCgroups() bool { return false }

// SupportsKVM os whether the OCI runtime supports running containers
// without KVM separation
func (r *ConmonRsOCIRuntime) SupportsKVM() bool { return false }

// AttachSocketPath is the path to the socket to attach to a given
// container.
// TODO: If we move Attach code in here, this should be made internal.
// We don't want to force all runtimes to share the same attach
// implementation.
func (r *ConmonRsOCIRuntime) AttachSocketPath(ctr *Container) (string, error) { return "", nil }

// ExecAttachSocketPath is the path to the socket to attach to a given
// exec session in the given container.
// TODO: Probably should be made internal.
func (r *ConmonRsOCIRuntime) ExecAttachSocketPath(ctr *Container, sessionID string) (string, error) {
	return "", nil
}

// ExitFilePath is the path to a container's exit file.
// All runtime implementations must create an exit file when containers
// exit, containing the exit code of the container (as a string).
// This is the path to that file for a given container.
func (r *ConmonRsOCIRuntime) ExitFilePath(ctr *Container) (string, error) { return "", nil }

// RuntimeInfo returns verbose information about the runtime.
func (r *ConmonRsOCIRuntime) RuntimeInfo() (*define.ConmonInfo, *define.OCIRuntimeInfo, error) {
	return nil, nil, nil
}
