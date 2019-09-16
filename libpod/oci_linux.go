// +build linux

package libpod

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/libpod/utils"
	pmount "github.com/containers/storage/pkg/mount"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"k8s.io/client-go/tools/remotecommand"
)

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

// CreateContainer creates a container in the OCI runtime
// TODO terminal support for container
// Presently just ignoring conmon opts related to it
func (r *OCIRuntime) createContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) (err error) {
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
					defer errorhandling.CloseQuiet(fd)

					// create a new mountns on the current thread
					if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
						return err
					}
					defer func() {
						if err := unix.Setns(int(fd.Fd()), unix.CLONE_NEWNS); err != nil {
							logrus.Errorf("unable to clone new namespace: %q", err)
						}
					}()

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
					return r.createOCIContainer(ctr, restoreOptions)
				}()
				ch <- err
			}()
			err := <-ch
			return err
		}
	}
	return r.createOCIContainer(ctr, restoreOptions)
}

func (r *OCIRuntime) pathPackage() string {
	return packageVersion(r.path)
}

func (r *OCIRuntime) conmonPackage() string {
	return packageVersion(r.conmonPath)
}

// execContainer executes a command in a running container
// TODO: Add --detach support
// TODO: Convert to use conmon
// TODO: add --pid-file and use that to generate exec session tracking
func (r *OCIRuntime) execContainer(c *Container, cmd, capAdd, env []string, tty bool, cwd, user, sessionID string, streams *AttachStreams, preserveFDs int, resize chan remotecommand.TerminalSize, detachKeys string) (int, chan error, error) {
	if len(cmd) == 0 {
		return -1, nil, errors.Wrapf(define.ErrInvalidArg, "must provide a command to execute")
	}

	if sessionID == "" {
		return -1, nil, errors.Wrapf(define.ErrEmptyID, "must provide a session ID for exec")
	}

	// create sync pipe to receive the pid
	parentSyncPipe, childSyncPipe, err := newPipe()
	if err != nil {
		return -1, nil, errors.Wrapf(err, "error creating socket pair")
	}

	defer errorhandling.CloseQuiet(parentSyncPipe)

	// create start pipe to set the cgroup before running
	// attachToExec is responsible for closing parentStartPipe
	childStartPipe, parentStartPipe, err := newPipe()
	if err != nil {
		return -1, nil, errors.Wrapf(err, "error creating socket pair")
	}

	// We want to make sure we close the parent{Start,Attach}Pipes if we fail
	// but also don't want to close them after attach to exec is called
	attachToExecCalled := false

	defer func() {
		if !attachToExecCalled {
			errorhandling.CloseQuiet(parentStartPipe)
		}
	}()

	// create the attach pipe to allow attach socket to be created before
	// $RUNTIME exec starts running. This is to make sure we can capture all output
	// from the process through that socket, rather than half reading the log, half attaching to the socket
	// attachToExec is responsible for closing parentAttachPipe
	parentAttachPipe, childAttachPipe, err := newPipe()
	if err != nil {
		return -1, nil, errors.Wrapf(err, "error creating socket pair")
	}

	defer func() {
		if !attachToExecCalled {
			errorhandling.CloseQuiet(parentAttachPipe)
		}
	}()

	childrenClosed := false
	defer func() {
		if !childrenClosed {
			errorhandling.CloseQuiet(childSyncPipe)
			errorhandling.CloseQuiet(childAttachPipe)
			errorhandling.CloseQuiet(childStartPipe)
		}
	}()

	runtimeDir, err := util.GetRuntimeDir()
	if err != nil {
		return -1, nil, err
	}

	processFile, err := prepareProcessExec(c, cmd, env, tty, cwd, user, sessionID)
	if err != nil {
		return -1, nil, err
	}

	var ociLog string
	if logrus.GetLevel() != logrus.DebugLevel && r.supportsJSON {
		ociLog = c.execOCILog(sessionID)
	}
	args := r.sharedConmonArgs(c, sessionID, c.execBundlePath(sessionID), c.execPidPath(sessionID), c.execLogPath(sessionID), c.execExitFileDir(sessionID), ociLog)

	if preserveFDs > 0 {
		args = append(args, formatRuntimeOpts("--preserve-fds", strconv.Itoa(preserveFDs))...)
	}

	for _, capability := range capAdd {
		args = append(args, formatRuntimeOpts("--cap", capability)...)
	}

	if tty {
		args = append(args, "-t")
	}

	// Append container ID and command
	args = append(args, "-e")
	// TODO make this optional when we can detach
	args = append(args, "--exec-attach")
	args = append(args, "--exec-process-spec", processFile.Name())

	logrus.WithFields(logrus.Fields{
		"args": args,
	}).Debugf("running conmon: %s", r.conmonPath)
	execCmd := exec.Command(r.conmonPath, args...)

	if streams.AttachInput {
		execCmd.Stdin = streams.InputStream
	}
	if streams.AttachOutput {
		execCmd.Stdout = streams.OutputStream
	}
	if streams.AttachError {
		execCmd.Stderr = streams.ErrorStream
	}

	conmonEnv, extraFiles, err := r.configureConmonEnv(runtimeDir)
	if err != nil {
		return -1, nil, err
	}

	if preserveFDs > 0 {
		for fd := 3; fd < 3+preserveFDs; fd++ {
			execCmd.ExtraFiles = append(execCmd.ExtraFiles, os.NewFile(uintptr(fd), fmt.Sprintf("fd-%d", fd)))
		}
	}

	// we don't want to step on users fds they asked to preserve
	// Since 0-2 are used for stdio, start the fds we pass in at preserveFDs+3
	execCmd.Env = append(r.conmonEnv, fmt.Sprintf("_OCI_SYNCPIPE=%d", preserveFDs+3), fmt.Sprintf("_OCI_STARTPIPE=%d", preserveFDs+4), fmt.Sprintf("_OCI_ATTACHPIPE=%d", preserveFDs+5))
	execCmd.Env = append(execCmd.Env, conmonEnv...)

	execCmd.ExtraFiles = append(execCmd.ExtraFiles, childSyncPipe, childStartPipe, childAttachPipe)
	execCmd.ExtraFiles = append(execCmd.ExtraFiles, extraFiles...)
	execCmd.Dir = c.execBundlePath(sessionID)
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	err = startCommandGivenSelinux(execCmd)

	// We don't need children pipes  on the parent side
	errorhandling.CloseQuiet(childSyncPipe)
	errorhandling.CloseQuiet(childAttachPipe)
	errorhandling.CloseQuiet(childStartPipe)
	childrenClosed = true

	if err != nil {
		return -1, nil, errors.Wrapf(err, "cannot start container %s", c.ID())
	}
	if err := r.moveConmonToCgroupAndSignal(c, execCmd, parentStartPipe, sessionID); err != nil {
		return -1, nil, err
	}

	if preserveFDs > 0 {
		for fd := 3; fd < 3+preserveFDs; fd++ {
			// These fds were passed down to the runtime.  Close them
			// and not interfere
			if err := os.NewFile(uintptr(fd), fmt.Sprintf("fd-%d", fd)).Close(); err != nil {
				logrus.Debugf("unable to close file fd-%d", fd)
			}
		}
	}

	// TODO Only create if !detach
	// Attach to the container before starting it
	attachChan := make(chan error)
	go func() {
		// attachToExec is responsible for closing pipes
		attachChan <- c.attachToExec(streams, detachKeys, resize, sessionID, parentStartPipe, parentAttachPipe)
		close(attachChan)
	}()
	attachToExecCalled = true

	pid, err := readConmonPipeData(parentSyncPipe, ociLog)

	return pid, attachChan, err
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
	if rootless.IsRootless() || ctr.config.NoCgroups {
		// we don't use --all for rootless containers as the OCI runtime might use
		// the cgroups to determine the PIDs, but for rootless containers there is
		// not any.
		// Same logic for NoCgroups - we can't use cgroups as the user
		// explicitly requested none be created.
		args = []string{"kill", ctr.ID(), "KILL"}
	} else {
		args = []string{"kill", "--all", ctr.ID(), "KILL"}
	}

	runtimeDir, err := util.GetRuntimeDir()
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
	runtimeDir, err := util.GetRuntimeDir()
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
