//go:build linux
// +build linux

package libpod

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	//"net/http"

	"path/filepath"

	//"sync"

	//cutil "github.com/containers/common/pkg/util"
	"github.com/containers/common/pkg/resize"
	cutil "github.com/containers/common/pkg/util"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/logs"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"

	//"github.com/containers/podman/v4/libpod/logs"

	runcconfig "github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func unpauseContainer(ctr *Container, path string, runtimeFlags []string) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, path, append(runtimeFlags, "resume", ctr.ID())...)
}

func pauseContainer(ctr *Container, path string, runtimeFlags []string) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, path, append(runtimeFlags, "pause", ctr.ID())...)
}

func httpAttach(attachSock string, ctr *Container, req *http.Request, w http.ResponseWriter, streams *HTTPAttachStreams, detachKeys *string, cancel <-chan bool, hijackDone chan<- bool, streamAttach, streamLogs bool) (deferredErr error) {
	isTerminal := ctr.Terminal()

	if streams != nil {
		if !streams.Stdin && !streams.Stdout && !streams.Stderr {
			return fmt.Errorf("must specify at least one stream to attach to: %w", define.ErrInvalidArg)
		}
	}

	var conn *net.UnixConn
	if streamAttach {
		newConn, err := openUnixSocket(attachSock)
		if err != nil {
			return fmt.Errorf("failed to connect to container's attach socket: %v: %w", attachSock, err)
		}
		conn = newConn
		defer func() {
			if err := conn.Close(); err != nil {
				logrus.Errorf("Unable to close container %s attach socket: %q", ctr.ID(), err)
			}
		}()

		logrus.Debugf("Successfully connected to container %s attach socket %s", ctr.ID(), attachSock)
	}

	detachString := ctr.runtime.config.Engine.DetachKeys
	if detachKeys != nil {
		detachString = *detachKeys
	}
	detach, err := processDetachKeys(detachString)
	if err != nil {
		return err
	}

	attachStdout := true
	attachStderr := true
	attachStdin := true
	if streams != nil {
		attachStdout = streams.Stdout
		attachStderr = streams.Stderr
		attachStdin = streams.Stdin
	}

	logrus.Debugf("Going to hijack container %s attach connection", ctr.ID())

	// Alright, let's hijack.
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return fmt.Errorf("unable to hijack connection")
	}

	httpCon, httpBuf, err := hijacker.Hijack()
	if err != nil {
		return fmt.Errorf("hijacking connection: %w", err)
	}

	reader := io.NopCloser(httpBuf)

	hijackDone <- true

	writeHijackHeader(req, httpBuf)

	// Force a flush after the header is written.
	if err := httpBuf.Flush(); err != nil {
		return fmt.Errorf("flushing HTTP hijack header: %w", err)
	}

	defer func() {
		hijackWriteErrorAndClose(deferredErr, ctr.ID(), isTerminal, httpCon, httpBuf)
	}()

	logrus.Debugf("Hijack for container %s attach session done, ready to stream", ctr.ID())

	// TODO: This is gross. Really, really gross.
	// I want to say we should read all the logs into an array before
	// calling this, in container_api.go, but that could take a lot of
	// memory...
	// On the whole, we need to figure out a better way of doing this,
	// though.
	logSize := 0
	if streamLogs {
		logrus.Debugf("Will stream logs for container %s attach session", ctr.ID())

		// Get all logs for the container
		logChan := make(chan *logs.LogLine)
		logOpts := new(logs.LogOptions)
		logOpts.Tail = -1
		logOpts.WaitGroup = new(sync.WaitGroup)
		errChan := make(chan error)
		go func() {
			var err error
			// In non-terminal mode we need to prepend with the
			// stream header.
			logrus.Debugf("Writing logs for container %s to HTTP attach", ctr.ID())
			for logLine := range logChan {
				if !isTerminal {
					device := logLine.Device
					var header []byte
					headerLen := uint32(len(logLine.Msg))
					logSize += len(logLine.Msg)
					switch strings.ToLower(device) {
					case "stdin":
						header = makeHTTPAttachHeader(0, headerLen)
					case "stdout":
						header = makeHTTPAttachHeader(1, headerLen)
					case "stderr":
						header = makeHTTPAttachHeader(2, headerLen)
					default:
						logrus.Errorf("Unknown device for log line: %s", device)
						header = makeHTTPAttachHeader(1, headerLen)
					}
					_, err = httpBuf.Write(header)
					if err != nil {
						break
					}
				}
				_, err = httpBuf.Write([]byte(logLine.Msg))
				if err != nil {
					break
				}
				if !logLine.Partial() {
					_, err = httpBuf.Write([]byte("\n"))
					if err != nil {
						break
					}
				}
				err = httpBuf.Flush()
				if err != nil {
					break
				}
			}
			errChan <- err
		}()
		if err := ctr.ReadLog(context.Background(), logOpts, logChan, 0); err != nil {
			return err
		}
		go func() {
			logOpts.WaitGroup.Wait()
			close(logChan)
		}()
		logrus.Debugf("Done reading logs for container %s, %d bytes", ctr.ID(), logSize)
		if err := <-errChan; err != nil {
			return err
		}
	}
	if !streamAttach {
		logrus.Debugf("Done streaming logs for container %s attach, exiting as attach streaming not requested", ctr.ID())
		return nil
	}

	logrus.Debugf("Forwarding attach output for container %s", ctr.ID())

	stdoutChan := make(chan error)
	stdinChan := make(chan error)

	// Handle STDOUT/STDERR
	go func() {
		var err error
		if isTerminal {
			// Hack: return immediately if attachStdout not set to
			// emulate Docker.
			// Basically, when terminal is set, STDERR goes nowhere.
			// Everything does over STDOUT.
			// Therefore, if not attaching STDOUT - we'll never copy
			// anything from here.
			logrus.Debugf("Performing terminal HTTP attach for container %s", ctr.ID())
			if attachStdout {
				err = httpAttachTerminalCopy(conn, httpBuf, ctr.ID())
			}
		} else {
			logrus.Debugf("Performing non-terminal HTTP attach for container %s", ctr.ID())
			err = httpAttachNonTerminalCopy(conn, httpBuf, ctr.ID(), attachStdin, attachStdout, attachStderr)
		}
		stdoutChan <- err
		logrus.Debugf("STDOUT/ERR copy completed")
	}()
	// Next, STDIN. Avoid entirely if attachStdin unset.
	if attachStdin {
		go func() {
			_, err := cutil.CopyDetachable(conn, reader, detach)
			logrus.Debugf("STDIN copy completed")
			stdinChan <- err
		}()
	}

	for {
		select {
		case err := <-stdoutChan:
			if err != nil {
				return err
			}

			return nil
		case err := <-stdinChan:
			if err != nil {
				return err
			}
			// copy stdin is done, close it
			if connErr := socketCloseWrite(conn); connErr != nil {
				logrus.Errorf("Unable to close conn: %v", connErr)
			}
		case <-cancel:
			return nil
		}
	}
}

func stopContainer(ctr *Container, all bool, timeout uint, runtimeFlags []string, path string) error {
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
		if err := killContainer(all, ctr, stopSignal, runtimeFlags, path); err != nil {
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

	if err := killContainer(all, ctr, uint(unix.SIGKILL), runtimeFlags, path); err != nil {
		// Again, check if the container is gone. If it is, exit cleanly.
		if aliveErr := unix.Kill(ctr.state.PID, 0); errors.Is(aliveErr, unix.ESRCH) {
			return nil
		}
		return fmt.Errorf("sending SIGKILL to container %s: %w", ctr.ID(), err)
	}

	// Give runtime a few seconds to make it happen
	if err := waitContainerStop(ctr, killContainerTimeout); err != nil {
		return err
	}

	return nil
}

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

func updateContainerStatus(path string, ctr *Container) error {
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
		out, err2 := io.ReadAll(errPipe)
		if err2 != nil {
			return fmt.Errorf("getting container %s state: %w", ctr.ID(), err)
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
		return fmt.Errorf("getting container %s state. stderr/out: %s: %w", ctr.ID(), out, err)
	}
	defer func() {
		_ = cmd.Wait()
	}()

	if err := errPipe.Close(); err != nil {
		return err
	}
	out, err := io.ReadAll(outPipe)
	if err != nil {
		return fmt.Errorf("reading stdout: %s: %w", ctr.ID(), err)
	}
	if err := json.NewDecoder(bytes.NewBuffer(out)).Decode(state); err != nil {
		return fmt.Errorf("decoding container status for container %s: %w", ctr.ID(), err)
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

	// Handle ContainerStateStopping - keep it unless the container
	// transitioned to no longer running.
	if oldState == define.ContainerStateStopping && (ctr.state.State == define.ContainerStatePaused || ctr.state.State == define.ContainerStateRunning) {
		ctr.state.State = define.ContainerStateStopping
	}

	return nil
}

func deleteContainer(ctr *Container, path string, runtimeFlags []string) error {
	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	return utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, path, append(runtimeFlags, "delete", "--force", ctr.ID())...)
}

func killContainer(all bool, ctr *Container, signal uint, runtimeFlags []string, path string) error {
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
		if err2 := updateContainerStatus(path, ctr); err2 != nil {
			logrus.Infof("Error updating status for container %s: %v", ctr.ID(), err2)
		}
		if ctr.ensureState(define.ContainerStateStopped, define.ContainerStateExited) {
			return fmt.Errorf("%w: %s", define.ErrCtrStateInvalid, ctr.state.State)
		}
		return fmt.Errorf("sending signal to container %s: %w", ctr.ID(), err)
	}

	return nil
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

// exitFilePath is an internal implementation of the OCI Runtime's ExitFilePath function.
// This is shared between the ConmonOCIRuntime and ConmonRsOCIRuntime types.
func exitFilePath(ctr *Container, exitsDir string) (string, error) {
	if ctr == nil {
		return "", fmt.Errorf("must provide a valid container to get exit file path: %w", define.ErrInvalidArg)
	}
	return filepath.Join(exitsDir, ctr.ID()), nil
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

// getOCIRuntimeVersion returns a string representation of the OCI runtime's
// version.
func getOCIRuntimeVersion(path string) (string, error) {
	output, err := utils.ExecCmd(path, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(output, "\n"), nil
}

// getConmonVersion returns a string representation of the conmon version.
func getConmonVersion(conmonPath string) (string, error) {
	output, err := utils.ExecCmd(conmonPath, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(strings.Replace(output, "\n", ", ", 1), "\n"), nil
}
