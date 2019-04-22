// +build linux

package libpod

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/cgroups"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/libpod/utils"
	pmount "github.com/containers/storage/pkg/mount"
	"github.com/coreos/go-systemd/activation"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const unknownPackage = "Unknown"

func (r *OCIRuntime) moveConmonToCgroup(ctr *Container, cgroupParent string, cmd *exec.Cmd) error {
	if os.Geteuid() == 0 {
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
	return nil
}

// newPipe creates a unix socket pair for communication
func newPipe() (parent *os.File, child *os.File, err error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
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
			if err := os.Chmod(path, os.FileMode(st.Mode()|0111)); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateContainer creates a container in the OCI runtime
// TODO terminal support for container
// Presently just ignoring conmon opts related to it
func (r *OCIRuntime) createContainer(ctr *Container, cgroupParent string, restoreOptions *ContainerCheckpointOptions) (err error) {
	if len(ctr.config.IDMappings.UIDMap) != 0 || len(ctr.config.IDMappings.GIDMap) != 0 {
		for _, i := range []string{ctr.state.RunDir, ctr.runtime.config.TmpDir, ctr.config.StaticDir, ctr.state.Mountpoint, ctr.runtime.config.VolumePath} {
			if err := makeAccessible(i, ctr.RootUID(), ctr.RootGID()); err != nil {
				return err
			}
		}

		// if we are running a non privileged container, be sure to umount some kernel paths so they are not
		// bind mounted inside the container at all.
		if !ctr.config.Privileged && !rootless.IsRootless() {
			ch := make(chan error)
			go func() {
				runtime.LockOSThread()
				err := func() error {
					fd, err := os.Open(fmt.Sprintf("/proc/%d/task/%d/ns/mnt", os.Getpid(), unix.Gettid()))
					if err != nil {
						return err
					}
					defer fd.Close()

					// create a new mountns on the current thread
					if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
						return err
					}
					defer unix.Setns(int(fd.Fd()), unix.CLONE_NEWNS)

					// don't spread our mounts around.  We are setting only /sys to be slave
					// so that the cleanup process is still able to umount the storage and the
					// changes are propagated to the host.
					err = unix.Mount("/sys", "/sys", "none", unix.MS_REC|unix.MS_SLAVE, "")
					if err != nil {
						return errors.Wrapf(err, "cannot make /sys slave")
					}

					mounts, err := pmount.GetMounts()
					if err != nil {
						return err
					}
					for _, m := range mounts {
						if !strings.HasPrefix(m.Mountpoint, "/sys/kernel") {
							continue
						}
						err = unix.Unmount(m.Mountpoint, 0)
						if err != nil && !os.IsNotExist(err) {
							return errors.Wrapf(err, "cannot unmount %s", m.Mountpoint)
						}
					}
					return r.createOCIContainer(ctr, cgroupParent, restoreOptions)
				}()
				ch <- err
			}()
			err := <-ch
			return err
		}
	}
	return r.createOCIContainer(ctr, cgroupParent, restoreOptions)
}

func rpmVersion(path string) string {
	output := unknownPackage
	cmd := exec.Command("/usr/bin/rpm", "-q", "-f", path)
	if outp, err := cmd.Output(); err == nil {
		output = string(outp)
	}
	return strings.Trim(output, "\n")
}

func dpkgVersion(path string) string {
	output := unknownPackage
	cmd := exec.Command("/usr/bin/dpkg", "-S", path)
	if outp, err := cmd.Output(); err == nil {
		output = string(outp)
	}
	return strings.Trim(output, "\n")
}

func (r *OCIRuntime) pathPackage() string {
	if out := rpmVersion(r.path); out != unknownPackage {
		return out
	}
	return dpkgVersion(r.path)
}

func (r *OCIRuntime) conmonPackage() string {
	if out := rpmVersion(r.conmonPath); out != unknownPackage {
		return out
	}
	return dpkgVersion(r.conmonPath)
}

func (r *OCIRuntime) createOCIContainer(ctr *Container, cgroupParent string, restoreOptions *ContainerCheckpointOptions) (err error) {
	var stderrBuf bytes.Buffer

	runtimeDir, err := util.GetRootlessRuntimeDir()
	if err != nil {
		return err
	}

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

	logLevel := logrus.GetLevel()
	args = append(args, "--log-level", logLevel.String())

	if logLevel == logrus.DebugLevel {
		logrus.Debugf("%s messages will be logged to syslog", r.conmonPath)
		args = append(args, "--syslog")
	}

	if restoreOptions != nil {
		args = append(args, "--restore", ctr.CheckpointPath())
		if restoreOptions.TCPEstablished {
			args = append(args, "--restore-arg", "--tcp-established")
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

	cmd.ExtraFiles = append(cmd.ExtraFiles, childPipe, childStartPipe)
	// 0, 1 and 2 are stdin, stdout and stderr
	cmd.Env = append(r.conmonEnv, fmt.Sprintf("_OCI_SYNCPIPE=%d", 3))
	cmd.Env = append(cmd.Env, fmt.Sprintf("_OCI_STARTPIPE=%d", 4))
	cmd.Env = append(cmd.Env, fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir))
	cmd.Env = append(cmd.Env, fmt.Sprintf("_CONTAINERS_USERNS_CONFIGURED=%s", os.Getenv("_CONTAINERS_USERNS_CONFIGURED")))
	cmd.Env = append(cmd.Env, fmt.Sprintf("_CONTAINERS_ROOTLESS_UID=%s", os.Getenv("_CONTAINERS_ROOTLESS_UID")))
	cmd.Env = append(cmd.Env, fmt.Sprintf("HOME=%s", os.Getenv("HOME")))

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
		ctr.rootlessSlirpSyncR, ctr.rootlessSlirpSyncW, err = os.Pipe()
		if err != nil {
			return errors.Wrapf(err, "failed to create rootless network sync pipe")
		}
		// Leak one end in conmon, the other one will be leaked into slirp4netns
		cmd.ExtraFiles = append(cmd.ExtraFiles, ctr.rootlessSlirpSyncW)
	}

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
		var (
			plabel string
			con    selinux.Context
		)
		plabel, err = selinux.CurrentLabel()
		if err != nil {
			childPipe.Close()
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
		label.SetProcessLabel("")
		runtime.UnlockOSThread()
	} else {
		err = cmd.Start()
	}
	if err != nil {
		childPipe.Close()
		return err
	}
	defer cmd.Wait()

	// We don't need childPipe on the parent side
	childPipe.Close()
	childStartPipe.Close()

	// Move conmon to specified cgroup
	if err := r.moveConmonToCgroup(ctr, cgroupParent, cmd); err != nil {
		return err
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
		rdr := bufio.NewReader(parentPipe)
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
		ctr.state.PID = ss.si.Pid
	case <-time.After(ContainerCreateTimeout):
		return errors.Wrapf(ErrInternal, "container creation timeout")
	}
	return nil
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
		logrus.Debugf("container %s did not die within timeout %d", ctr.ID(), timeout)
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

// stopContainer stops a container, first using its given stop signal (or
// SIGTERM if no signal was specified), then using SIGKILL
// Timeout is given in seconds. If timeout is 0, the container will be
// immediately kill with SIGKILL
// Does not set finished time for container, assumes you will run updateStatus
// after to pull the exit code
func (r *OCIRuntime) stopContainer(ctr *Container, timeout uint) error {
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

	var args []string
	if rootless.IsRootless() {
		// we don't use --all for rootless containers as the OCI runtime might use
		// the cgroups to determine the PIDs, but for rootless containers there is
		// not any.
		args = []string{"kill", ctr.ID(), "KILL"}
	} else {
		args = []string{"kill", "--all", ctr.ID(), "KILL"}
	}

	runtimeDir, err := util.GetRootlessRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, args...); err != nil {
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
	runtimeDir, err := util.GetRootlessRuntimeDir()
	if err != nil {
		return err
	}
	env := []string{fmt.Sprintf("XDG_RUNTIME_DIR=%s", runtimeDir)}

	// If timeout is 0, just use SIGKILL
	if timeout > 0 {
		// Stop using SIGTERM by default
		// Use SIGSTOP after a timeout
		logrus.Debugf("Killing all processes in container %s with SIGTERM", ctr.ID())
		if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, "kill", "--all", ctr.ID(), "TERM"); err != nil {
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
	if err := utils.ExecCmdWithStdStreams(os.Stdin, os.Stdout, os.Stderr, env, r.path, "kill", "--all", ctr.ID(), "KILL"); err != nil {
		return errors.Wrapf(err, "error sending SIGKILL to container %s processes", ctr.ID())
	}

	// Give the processes a few seconds to go down
	if err := waitPidsStop(execSessions, killContainerTimeout); err != nil {
		return errors.Wrapf(err, "failed to kill container %s exec sessions", ctr.ID())
	}

	return nil
}
