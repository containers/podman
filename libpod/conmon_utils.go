//go:build linux
// +build linux

package libpod

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"

	"net"
	//"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	//"sync"
	"syscall"
	"time"

	"github.com/containers/common/pkg/config"
	//cutil "github.com/containers/common/pkg/util"
	"github.com/containers/common/pkg/resize"
	"github.com/containers/podman/v4/libpod/define"
	//"github.com/containers/podman/v4/libpod/logs"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	runcconfig "github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// writeConmonPipeData writes nonce data to a pipe
func writeConmonPipeData(pipe *os.File) error {
	someData := []byte{0}
	_, err := pipe.Write(someData)
	return err
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

// GetLimits converts spec resource limits to cgroup consumable limits
func GetLimits(resource *spec.LinuxResources) (runcconfig.Resources, error) {
	if resource == nil {
		resource = &spec.LinuxResources{}
	}
	final := &runcconfig.Resources{}
	devs := []*devices.Rule{}

	// Devices
	for _, entry := range resource.Devices {
		if entry.Major == nil || entry.Minor == nil {
			continue
		}
		runeType := 'a'
		switch entry.Type {
		case "b":
			runeType = 'b'
		case "c":
			runeType = 'c'
		}

		devs = append(devs, &devices.Rule{
			Type:        devices.Type(runeType),
			Major:       *entry.Major,
			Minor:       *entry.Minor,
			Permissions: devices.Permissions(entry.Access),
			Allow:       entry.Allow,
		})
	}
	final.Devices = devs

	// HugepageLimits
	pageLimits := []*runcconfig.HugepageLimit{}
	for _, entry := range resource.HugepageLimits {
		pageLimits = append(pageLimits, &runcconfig.HugepageLimit{
			Pagesize: entry.Pagesize,
			Limit:    entry.Limit,
		})
	}
	final.HugetlbLimit = pageLimits

	// Networking
	netPriorities := []*runcconfig.IfPrioMap{}
	if resource.Network != nil {
		for _, entry := range resource.Network.Priorities {
			netPriorities = append(netPriorities, &runcconfig.IfPrioMap{
				Interface: entry.Name,
				Priority:  int64(entry.Priority),
			})
		}
	}
	final.NetPrioIfpriomap = netPriorities
	rdma := make(map[string]runcconfig.LinuxRdma)
	for name, entry := range resource.Rdma {
		rdma[name] = runcconfig.LinuxRdma{HcaHandles: entry.HcaHandles, HcaObjects: entry.HcaObjects}
	}
	final.Rdma = rdma

	// Memory
	if resource.Memory != nil {
		if resource.Memory.Limit != nil {
			final.Memory = *resource.Memory.Limit
		}
		if resource.Memory.Reservation != nil {
			final.MemoryReservation = *resource.Memory.Reservation
		}
		if resource.Memory.Swap != nil {
			final.MemorySwap = *resource.Memory.Swap
		}
		if resource.Memory.Swappiness != nil {
			final.MemorySwappiness = resource.Memory.Swappiness
		}
	}

	// CPU
	if resource.CPU != nil {
		if resource.CPU.Period != nil {
			final.CpuPeriod = *resource.CPU.Period
		}
		if resource.CPU.Quota != nil {
			final.CpuQuota = *resource.CPU.Quota
		}
		if resource.CPU.RealtimePeriod != nil {
			final.CpuRtPeriod = *resource.CPU.RealtimePeriod
		}
		if resource.CPU.RealtimeRuntime != nil {
			final.CpuRtRuntime = *resource.CPU.RealtimeRuntime
		}
		if resource.CPU.Shares != nil {
			final.CpuShares = *resource.CPU.Shares
		}
		final.CpusetCpus = resource.CPU.Cpus
		final.CpusetMems = resource.CPU.Mems
	}

	// BlkIO
	if resource.BlockIO != nil {
		if len(resource.BlockIO.ThrottleReadBpsDevice) > 0 {
			for _, entry := range resource.BlockIO.ThrottleReadBpsDevice {
				throttle := &runcconfig.ThrottleDevice{}
				dev := &runcconfig.BlockIODevice{
					Major: entry.Major,
					Minor: entry.Minor,
				}
				throttle.BlockIODevice = *dev
				throttle.Rate = entry.Rate
				final.BlkioThrottleReadBpsDevice = append(final.BlkioThrottleReadBpsDevice, throttle)
			}
		}
		if len(resource.BlockIO.ThrottleWriteBpsDevice) > 0 {
			for _, entry := range resource.BlockIO.ThrottleWriteBpsDevice {
				throttle := &runcconfig.ThrottleDevice{}
				dev := &runcconfig.BlockIODevice{
					Major: entry.Major,
					Minor: entry.Minor,
				}
				throttle.BlockIODevice = *dev
				throttle.Rate = entry.Rate
				final.BlkioThrottleWriteBpsDevice = append(final.BlkioThrottleWriteBpsDevice, throttle)
			}
		}
		if len(resource.BlockIO.ThrottleReadIOPSDevice) > 0 {
			for _, entry := range resource.BlockIO.ThrottleReadIOPSDevice {
				throttle := &runcconfig.ThrottleDevice{}
				dev := &runcconfig.BlockIODevice{
					Major: entry.Major,
					Minor: entry.Minor,
				}
				throttle.BlockIODevice = *dev
				throttle.Rate = entry.Rate
				final.BlkioThrottleReadIOPSDevice = append(final.BlkioThrottleReadIOPSDevice, throttle)
			}
		}
		if len(resource.BlockIO.ThrottleWriteIOPSDevice) > 0 {
			for _, entry := range resource.BlockIO.ThrottleWriteIOPSDevice {
				throttle := &runcconfig.ThrottleDevice{}
				dev := &runcconfig.BlockIODevice{
					Major: entry.Major,
					Minor: entry.Minor,
				}
				throttle.BlockIODevice = *dev
				throttle.Rate = entry.Rate
				final.BlkioThrottleWriteIOPSDevice = append(final.BlkioThrottleWriteIOPSDevice, throttle)
			}
		}
		if resource.BlockIO.LeafWeight != nil {
			final.BlkioLeafWeight = *resource.BlockIO.LeafWeight
		}
		if resource.BlockIO.Weight != nil {
			final.BlkioWeight = *resource.BlockIO.Weight
		}
		if len(resource.BlockIO.WeightDevice) > 0 {
			for _, entry := range resource.BlockIO.WeightDevice {
				weight := &runcconfig.WeightDevice{}
				dev := &runcconfig.BlockIODevice{
					Major: entry.Major,
					Minor: entry.Minor,
				}
				if entry.Weight != nil {
					weight.Weight = *entry.Weight
				}
				if entry.LeafWeight != nil {
					weight.LeafWeight = *entry.LeafWeight
				}
				weight.BlockIODevice = *dev
				final.BlkioWeightDevice = append(final.BlkioWeightDevice, weight)
			}
		}
	}

	// Pids
	if resource.Pids != nil {
		final.PidsLimit = resource.Pids.Limit
	}

	// Networking
	if resource.Network != nil {
		if resource.Network.ClassID != nil {
			final.NetClsClassid = *resource.Network.ClassID
		}
	}

	// Unified state
	final.Unified = resource.Unified

	return *final, nil
}

// startContainer is an internal implementation of the OCI Runtime's StartContainer function.
// This is shared between the ConmonOCIRuntime and ConmonRsOCIRuntime types.
func startContainer(ctr *Container, path string, runtimeFlags []string) error {
	// TODO: streams should probably *not* be our STDIN/OUT/ERR - redirect to buffers?
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	if path, ok := os.LookupEnv("PATH"); ok {
		env = append(env, fmt.Sprintf("PATH=%s", path))
	}

	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, path, append(runtimeFlags, "start", ctr.ID())...); err != nil {
		return err
	}

	ctr.state.StartedTime = time.Now()

	return nil
}

// stopContainer is an internal implementation of the OCI Runtime's StopContainer function.
// This is shared between the ConmonOCIRuntime and ConmonRsOCIRuntime types.
func stopContainer(ctr *Container, timeout uint, all bool, path string, runtimeFlags []string) error {
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
		err = killContainer(ctr, stopSignal, all, path, runtimeFlags)
		if err != nil {
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
			logrus.Debugf("Timed out stopping container %s with %s, resorting to SIGKILL: %v", ctr.ID(), unix.SignalName(syscall.Signal(stopSignal)), err)
			logrus.Warnf("StopSignal %s failed to stop container %s in %d seconds, resorting to SIGKILL", unix.SignalName(syscall.Signal(stopSignal)), ctr.Name(), timeout)
		} else {
			// No error, the container is dead
			return nil
		}
	}

	err = killContainer(ctr, stopSignal, all, path, runtimeFlags)

	if err != nil {
		// Again, check if the container is gone. If it is, exit cleanly.
		err := unix.Kill(ctr.state.PID, 0)
		if err == unix.ESRCH {
			return nil
		}

		return fmt.Errorf("error sending SIGKILL to container %s: %w", ctr.ID(), err)
	}

	// Give runtime a few seconds to make it happen
	if err := waitContainerStop(ctr, killContainerTimeout); err != nil {
		return err
	}

	return nil
}

// killContainer is an internal implementation of the OCI Runtime's KillContainer function.
// This is shared between the ConmonOCIRuntime and ConmonRsOCIRuntime types.
func killContainer(ctr *Container, signal uint, all bool, path string, runtimeFlags []string) error {
	logrus.Debugf("Sending signal %d to container %s", signal, ctr.ID())
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	var args []string

	args = append(args, runtimeFlags...)
	if all {
		args = append(args, "kill", "--all", ctr.ID(), fmt.Sprintf("%d", signal))
	} else {
		args = append(args, "kill", ctr.ID(), fmt.Sprintf("%d", signal))
	}
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, path, args...); err != nil {
		// Update container state - there's a chance we failed because
		// the container exited in the meantime.
		var err2 error
		err2 = updateContainerStatus(ctr, path)
		if err2 != nil {
			logrus.Infof("Error updating status for container %s: %v", ctr.ID(), err2)
		}
		if ctr.ensureState(define.ContainerStateStopped, define.ContainerStateExited) {
			return define.ErrCtrStateInvalid
		}
		return fmt.Errorf("error sending signal to container %s: %w", ctr.ID(), err)
	}

	return nil
}

// updateContainer is an internal implementation of the OCI Runtime's UpdateContainer function.
// This is shared between the ConmonOCIRuntime and ConmonRsOCIRuntime types.
func updateContainerStatus(ctr *Container, path string) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}

	// Store old state so we know if we were already stopped
	oldState := ctr.state.State

	state := new(spec.State)

	cmd := exec.Command(path, "state", ctr.ID())
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
		out, err2 := ioutil.ReadAll(errPipe)
		if err2 != nil {
			return fmt.Errorf("error getting container %s state: %w", ctr.ID(), err)
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
		return fmt.Errorf("error getting container %s state. stderr/out: %s: %w", ctr.ID(), out, err)
	}
	defer func() {
		_ = cmd.Wait()
	}()

	if err := errPipe.Close(); err != nil {
		return err
	}
	out, err := ioutil.ReadAll(outPipe)
	if err != nil {
		return fmt.Errorf("error reading stdout: %s: %w", ctr.ID(), err)
	}
	if err := json.NewDecoder(bytes.NewBuffer(out)).Decode(state); err != nil {
		return fmt.Errorf("error decoding container status for container %s: %w", ctr.ID(), err)
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

	// Only grab exit status if we were not already stopped
	// If we were, it should already be in the database
	if ctr.state.State == define.ContainerStateStopped && oldState != define.ContainerStateStopped {
		if _, err := ctr.Wait(context.Background()); err != nil {
			logrus.Errorf("Waiting for container %s to exit: %v", ctr.ID(), err)
		}
		return nil
	}

	// Handle ContainerStateStopping - keep it unless the container
	// transitioned to no longer running.
	if oldState == define.ContainerStateStopping && (ctr.state.State == define.ContainerStatePaused || ctr.state.State == define.ContainerStateRunning) {
		ctr.state.State = define.ContainerStateStopping
	}

	return nil
}

// exitFilePath is an internal implementation of the OCI Runtime's ExitFilePath function.
// This is shared between the ConmonOCIRuntime and ConmonRsOCIRuntime types.
func exitFilePath(ctr *Container, exitsDir string) (string, error) {
	if ctr == nil {
		return "", fmt.Errorf("must provide a valid container to get exit file path: %w", define.ErrInvalidArg)
	}
	return filepath.Join(exitsDir, ctr.ID()), nil
}

// pauseContainer is an internal implementation of the OCI Runtime's PauseContainer function.
// This is shared between the ConmonOCIRuntime and ConmonRsOCIRuntime types.
func pauseContainer(ctr *Container, path string, runtimeFlags []string) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, path, append(runtimeFlags, "pause", ctr.ID())...)
}

// unpauseContainer is an internal implementation of the OCI Runtime's UnpauseContainer function.
// This is shared between the ConmonOCIRuntime and ConmonRsOCIRuntime types.
func unpauseContainer(ctr *Container, path string, runtimeFlags []string) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, path, append(runtimeFlags, "resume", ctr.ID())...)
}

// deleteContainer is an internal implementation of the OCI Runtime's DeleteContainer function.
// This is shared between the ConmonOCIRuntime and ConmonRsOCIRuntime types.
func deleteContainer(ctr *Container, path string, runtimeFlags []string) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, path, append(runtimeFlags, "delete", "--force", ctr.ID())...)
}

func attach(c *Container, params *AttachOptions) error {
	passthrough := c.LogDriver() == define.PassthroughLogging

	if params == nil || params.Streams == nil {
		return fmt.Errorf("must provide parameters to Attach: %w", define.ErrInternal)
	}

	if !params.Streams.AttachOutput && !params.Streams.AttachError && !params.Streams.AttachInput && !passthrough {
		return fmt.Errorf("must provide at least one stream to attach to: %w", define.ErrInvalidArg)
	}
	if params.Start && params.Started == nil {
		return fmt.Errorf("started chan not passed when startContainer set: %w", define.ErrInternal)
	}

	keys := config.DefaultDetachKeys
	if params.DetachKeys != nil {
		keys = *params.DetachKeys
	}

	detachKeys, err := processDetachKeys(keys)
	if err != nil {
		return err
	}

	var conn *net.UnixConn
	if !passthrough {
		logrus.Debugf("Attaching to container %s", c.ID())

		// If we have a resize, do it.
		if params.InitialSize != nil {
			if err := attachResize(c, *params.InitialSize); err != nil {
				return err
			}
		}

		attachSock, err := c.AttachSocketPath()
		if err != nil {
			return err
		}

		logrus.Warnf("current attach socket path: %s", attachSock)

		conn, err = openUnixSocket(attachSock)
		if err != nil {
			return fmt.Errorf("failed to connect to container's attach socket: %v: %w", attachSock, err)
		}
		defer func() {
			if err := conn.Close(); err != nil {
				logrus.Errorf("unable to close socket: %q", err)
			}
		}()
	}

	// If starting was requested, start the container and notify when that's
	// done.
	if params.Start {
		if err := c.start(); err != nil {
			return err
		}
		params.Started <- true
	}

	if passthrough {
		return nil
	}

	receiveStdoutError, stdinDone := setupStdioChannels(params.Streams, conn, detachKeys)
	if params.AttachReady != nil {
		params.AttachReady <- true
	}
	return readStdio(conn, params.Streams, receiveStdoutError, stdinDone)
}

func attachResize(ctr *Container, newSize resize.TerminalSize) error {
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

func attachSocketPath(ctr *Container) (string, error) {
	if ctr == nil {
		return "", fmt.Errorf("must provide a valid container to get attach socket path: %w", define.ErrInvalidArg)
	}

	return filepath.Join(ctr.bundlePath(), "attach"), nil
}

func checkConmonRunning(ctr *Container) (bool, error) {
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
