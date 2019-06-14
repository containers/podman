// +build linux

package chroot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/containers/buildah/bind"
	"github.com/containers/buildah/pkg/unshare"
	"github.com/containers/buildah/util"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/containers/storage/pkg/mount"
	"github.com/containers/storage/pkg/reexec"
	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sys/unix"
)

const (
	// runUsingChrootCommand is a command we use as a key for reexec
	runUsingChrootCommand = "buildah-chroot-runtime"
	// runUsingChrootExec is a command we use as a key for reexec
	runUsingChrootExecCommand = "buildah-chroot-exec"
)

var (
	rlimitsMap = map[string]int{
		"RLIMIT_AS":         unix.RLIMIT_AS,
		"RLIMIT_CORE":       unix.RLIMIT_CORE,
		"RLIMIT_CPU":        unix.RLIMIT_CPU,
		"RLIMIT_DATA":       unix.RLIMIT_DATA,
		"RLIMIT_FSIZE":      unix.RLIMIT_FSIZE,
		"RLIMIT_LOCKS":      unix.RLIMIT_LOCKS,
		"RLIMIT_MEMLOCK":    unix.RLIMIT_MEMLOCK,
		"RLIMIT_MSGQUEUE":   unix.RLIMIT_MSGQUEUE,
		"RLIMIT_NICE":       unix.RLIMIT_NICE,
		"RLIMIT_NOFILE":     unix.RLIMIT_NOFILE,
		"RLIMIT_NPROC":      unix.RLIMIT_NPROC,
		"RLIMIT_RSS":        unix.RLIMIT_RSS,
		"RLIMIT_RTPRIO":     unix.RLIMIT_RTPRIO,
		"RLIMIT_RTTIME":     unix.RLIMIT_RTTIME,
		"RLIMIT_SIGPENDING": unix.RLIMIT_SIGPENDING,
		"RLIMIT_STACK":      unix.RLIMIT_STACK,
	}
	rlimitsReverseMap = map[int]string{}
)

func init() {
	reexec.Register(runUsingChrootCommand, runUsingChrootMain)
	reexec.Register(runUsingChrootExecCommand, runUsingChrootExecMain)
	for limitName, limitNumber := range rlimitsMap {
		rlimitsReverseMap[limitNumber] = limitName
	}
}

type runUsingChrootSubprocOptions struct {
	Spec        *specs.Spec
	BundlePath  string
	UIDMappings []syscall.SysProcIDMap
	GIDMappings []syscall.SysProcIDMap
}

type runUsingChrootExecSubprocOptions struct {
	Spec       *specs.Spec
	BundlePath string
}

// RunUsingChroot runs a chrooted process, using some of the settings from the
// passed-in spec, and using the specified bundlePath to hold temporary files,
// directories, and mountpoints.
func RunUsingChroot(spec *specs.Spec, bundlePath, homeDir string, stdin io.Reader, stdout, stderr io.Writer) (err error) {
	var confwg sync.WaitGroup
	var homeFound bool
	for _, env := range spec.Process.Env {
		if strings.HasPrefix(env, "HOME=") {
			homeFound = true
			break
		}
	}
	if !homeFound {
		spec.Process.Env = append(spec.Process.Env, fmt.Sprintf("HOME=%s", homeDir))
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Write the runtime configuration, mainly for debugging.
	specbytes, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	if err = ioutils.AtomicWriteFile(filepath.Join(bundlePath, "config.json"), specbytes, 0600); err != nil {
		return errors.Wrapf(err, "error storing runtime configuration")
	}
	logrus.Debugf("config = %v", string(specbytes))

	// Default to using stdin/stdout/stderr if we weren't passed objects to use.
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	// Create a pipe for passing configuration down to the next process.
	preader, pwriter, err := os.Pipe()
	if err != nil {
		return errors.Wrapf(err, "error creating configuration pipe")
	}
	config, conferr := json.Marshal(runUsingChrootSubprocOptions{
		Spec:       spec,
		BundlePath: bundlePath,
	})
	if conferr != nil {
		return errors.Wrapf(conferr, "error encoding configuration for %q", runUsingChrootCommand)
	}

	// Set our terminal's mode to raw, to pass handling of special
	// terminal input to the terminal in the container.
	if spec.Process.Terminal && terminal.IsTerminal(unix.Stdin) {
		state, err := terminal.MakeRaw(unix.Stdin)
		if err != nil {
			logrus.Warnf("error setting terminal state: %v", err)
		} else {
			defer func() {
				if err = terminal.Restore(unix.Stdin, state); err != nil {
					logrus.Errorf("unable to restore terminal state: %v", err)
				}
			}()
		}
	}

	// Raise any resource limits that are higher than they are now, before
	// we drop any more privileges.
	if err = setRlimits(spec, false, true); err != nil {
		return err
	}

	// Start the grandparent subprocess.
	cmd := unshare.Command(runUsingChrootCommand)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	cmd.Dir = "/"
	cmd.Env = append([]string{fmt.Sprintf("LOGLEVEL=%d", logrus.GetLevel())}, os.Environ()...)

	logrus.Debugf("Running %#v in %#v", cmd.Cmd, cmd)
	confwg.Add(1)
	go func() {
		_, conferr = io.Copy(pwriter, bytes.NewReader(config))
		pwriter.Close()
		confwg.Done()
	}()
	cmd.ExtraFiles = append([]*os.File{preader}, cmd.ExtraFiles...)
	err = cmd.Run()
	confwg.Wait()
	if err == nil {
		return conferr
	}
	return err
}

// main() for grandparent subprocess.  Its main job is to shuttle stdio back
// and forth, managing a pseudo-terminal if we want one, for our child, the
// parent subprocess.
func runUsingChrootMain() {
	var options runUsingChrootSubprocOptions

	runtime.LockOSThread()

	// Set logging.
	if level := os.Getenv("LOGLEVEL"); level != "" {
		if ll, err := strconv.Atoi(level); err == nil {
			logrus.SetLevel(logrus.Level(ll))
		}
		os.Unsetenv("LOGLEVEL")
	}

	// Unpack our configuration.
	confPipe := os.NewFile(3, "confpipe")
	if confPipe == nil {
		fmt.Fprintf(os.Stderr, "error reading options pipe\n")
		os.Exit(1)
	}
	defer confPipe.Close()
	if err := json.NewDecoder(confPipe).Decode(&options); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding options: %v\n", err)
		os.Exit(1)
	}

	// Prepare to shuttle stdio back and forth.
	rootUid32, rootGid32, err := util.GetHostRootIDs(options.Spec)
	if err != nil {
		logrus.Errorf("error determining ownership for container stdio")
		os.Exit(1)
	}
	rootUid := int(rootUid32)
	rootGid := int(rootGid32)
	relays := make(map[int]int)
	closeOnceRunning := []*os.File{}
	var ctty *os.File
	var stdin io.Reader
	var stdinCopy io.WriteCloser
	var stdout io.Writer
	var stderr io.Writer
	fdDesc := make(map[int]string)
	if options.Spec.Process.Terminal {
		// Create a pseudo-terminal -- open a copy of the master side.
		ptyMasterFd, err := unix.Open("/dev/ptmx", os.O_RDWR, 0600)
		if err != nil {
			logrus.Errorf("error opening PTY master using /dev/ptmx: %v", err)
			os.Exit(1)
		}
		// Set the kernel's lock to "unlocked".
		locked := 0
		if result, _, err := unix.Syscall(unix.SYS_IOCTL, uintptr(ptyMasterFd), unix.TIOCSPTLCK, uintptr(unsafe.Pointer(&locked))); int(result) == -1 {
			logrus.Errorf("error locking PTY descriptor: %v", err)
			os.Exit(1)
		}
		// Get a handle for the other end.
		ptyFd, _, err := unix.Syscall(unix.SYS_IOCTL, uintptr(ptyMasterFd), unix.TIOCGPTPEER, unix.O_RDWR|unix.O_NOCTTY)
		if int(ptyFd) == -1 {
			if errno, isErrno := err.(syscall.Errno); !isErrno || (errno != syscall.EINVAL && errno != syscall.ENOTTY) {
				logrus.Errorf("error getting PTY descriptor: %v", err)
				os.Exit(1)
			}
			// EINVAL means the kernel's too old to understand TIOCGPTPEER.  Try TIOCGPTN.
			ptyN, err := unix.IoctlGetInt(ptyMasterFd, unix.TIOCGPTN)
			if err != nil {
				logrus.Errorf("error getting PTY number: %v", err)
				os.Exit(1)
			}
			ptyName := fmt.Sprintf("/dev/pts/%d", ptyN)
			fd, err := unix.Open(ptyName, unix.O_RDWR|unix.O_NOCTTY, 0620)
			if err != nil {
				logrus.Errorf("error opening PTY %q: %v", ptyName, err)
				os.Exit(1)
			}
			ptyFd = uintptr(fd)
		}
		// Make notes about what's going where.
		relays[ptyMasterFd] = unix.Stdout
		relays[unix.Stdin] = ptyMasterFd
		fdDesc[ptyMasterFd] = "container terminal"
		fdDesc[unix.Stdin] = "stdin"
		fdDesc[unix.Stdout] = "stdout"
		winsize := &unix.Winsize{}
		// Set the pseudoterminal's size to the configured size, or our own.
		if options.Spec.Process.ConsoleSize != nil {
			// Use configured sizes.
			winsize.Row = uint16(options.Spec.Process.ConsoleSize.Height)
			winsize.Col = uint16(options.Spec.Process.ConsoleSize.Width)
		} else {
			if terminal.IsTerminal(unix.Stdin) {
				// Use the size of our terminal.
				winsize, err = unix.IoctlGetWinsize(unix.Stdin, unix.TIOCGWINSZ)
				if err != nil {
					logrus.Debugf("error reading current terminal's size")
					winsize.Row = 0
					winsize.Col = 0
				}
			}
		}
		if winsize.Row != 0 && winsize.Col != 0 {
			if err = unix.IoctlSetWinsize(int(ptyFd), unix.TIOCSWINSZ, winsize); err != nil {
				logrus.Warnf("error setting terminal size for pty")
			}
			// FIXME - if we're connected to a terminal, we should
			// be passing the updated terminal size down when we
			// receive a SIGWINCH.
		}
		// Open an *os.File object that we can pass to our child.
		ctty = os.NewFile(ptyFd, "/dev/tty")
		// Set ownership for the PTY.
		if err = ctty.Chown(rootUid, rootGid); err != nil {
			var cttyInfo unix.Stat_t
			err2 := unix.Fstat(int(ptyFd), &cttyInfo)
			from := ""
			op := "setting"
			if err2 == nil {
				op = "changing"
				from = fmt.Sprintf("from %d/%d ", cttyInfo.Uid, cttyInfo.Gid)
			}
			logrus.Warnf("error %s ownership of container PTY %sto %d/%d: %v", op, from, rootUid, rootGid, err)
		}
		// Set permissions on the PTY.
		if err = ctty.Chmod(0620); err != nil {
			logrus.Errorf("error setting permissions of container PTY: %v", err)
			os.Exit(1)
		}
		// Make a note that our child (the parent subprocess) should
		// have the PTY connected to its stdio, and that we should
		// close it once it's running.
		stdin = ctty
		stdout = ctty
		stderr = ctty
		closeOnceRunning = append(closeOnceRunning, ctty)
	} else {
		// Create pipes for stdio.
		stdinRead, stdinWrite, err := os.Pipe()
		if err != nil {
			logrus.Errorf("error opening pipe for stdin: %v", err)
		}
		stdoutRead, stdoutWrite, err := os.Pipe()
		if err != nil {
			logrus.Errorf("error opening pipe for stdout: %v", err)
		}
		stderrRead, stderrWrite, err := os.Pipe()
		if err != nil {
			logrus.Errorf("error opening pipe for stderr: %v", err)
		}
		// Make notes about what's going where.
		relays[unix.Stdin] = int(stdinWrite.Fd())
		relays[int(stdoutRead.Fd())] = unix.Stdout
		relays[int(stderrRead.Fd())] = unix.Stderr
		fdDesc[int(stdinWrite.Fd())] = "container stdin pipe"
		fdDesc[int(stdoutRead.Fd())] = "container stdout pipe"
		fdDesc[int(stderrRead.Fd())] = "container stderr pipe"
		fdDesc[unix.Stdin] = "stdin"
		fdDesc[unix.Stdout] = "stdout"
		fdDesc[unix.Stderr] = "stderr"
		// Set ownership for the pipes.
		if err = stdinRead.Chown(rootUid, rootGid); err != nil {
			logrus.Errorf("error setting ownership of container stdin pipe: %v", err)
			os.Exit(1)
		}
		if err = stdoutWrite.Chown(rootUid, rootGid); err != nil {
			logrus.Errorf("error setting ownership of container stdout pipe: %v", err)
			os.Exit(1)
		}
		if err = stderrWrite.Chown(rootUid, rootGid); err != nil {
			logrus.Errorf("error setting ownership of container stderr pipe: %v", err)
			os.Exit(1)
		}
		// Make a note that our child (the parent subprocess) should
		// have the pipes connected to its stdio, and that we should
		// close its ends of them once it's running.
		stdin = stdinRead
		stdout = stdoutWrite
		stderr = stderrWrite
		closeOnceRunning = append(closeOnceRunning, stdinRead, stdoutWrite, stderrWrite)
		stdinCopy = stdinWrite
		defer stdoutRead.Close()
		defer stderrRead.Close()
	}
	for readFd, writeFd := range relays {
		if err := unix.SetNonblock(readFd, true); err != nil {
			logrus.Errorf("error setting descriptor %d (%s) non-blocking: %v", readFd, fdDesc[readFd], err)
			return
		}
		if err := unix.SetNonblock(writeFd, false); err != nil {
			logrus.Errorf("error setting descriptor %d (%s) blocking: %v", relays[writeFd], fdDesc[writeFd], err)
			return
		}
	}
	if err := unix.SetNonblock(relays[unix.Stdin], true); err != nil {
		logrus.Errorf("error setting %d to nonblocking: %v", relays[unix.Stdin], err)
	}
	go func() {
		buffers := make(map[int]*bytes.Buffer)
		for _, writeFd := range relays {
			buffers[writeFd] = new(bytes.Buffer)
		}
		pollTimeout := -1
		stdinClose := false
		for len(relays) > 0 {
			fds := make([]unix.PollFd, 0, len(relays))
			for fd := range relays {
				fds = append(fds, unix.PollFd{Fd: int32(fd), Events: unix.POLLIN | unix.POLLHUP})
			}
			_, err := unix.Poll(fds, pollTimeout)
			if !util.LogIfNotRetryable(err, fmt.Sprintf("poll: %v", err)) {
				return
			}
			removeFds := make(map[int]struct{})
			for _, rfd := range fds {
				if rfd.Revents&unix.POLLHUP == unix.POLLHUP {
					removeFds[int(rfd.Fd)] = struct{}{}
				}
				if rfd.Revents&unix.POLLNVAL == unix.POLLNVAL {
					logrus.Debugf("error polling descriptor %s: closed?", fdDesc[int(rfd.Fd)])
					removeFds[int(rfd.Fd)] = struct{}{}
				}
				if rfd.Revents&unix.POLLIN == 0 {
					if stdinClose && stdinCopy == nil {
						continue
					}
					continue
				}
				b := make([]byte, 8192)
				nread, err := unix.Read(int(rfd.Fd), b)
				util.LogIfNotRetryable(err, fmt.Sprintf("read %s: %v", fdDesc[int(rfd.Fd)], err))
				if nread > 0 {
					if wfd, ok := relays[int(rfd.Fd)]; ok {
						nwritten, err := buffers[wfd].Write(b[:nread])
						if err != nil {
							logrus.Debugf("buffer: %v", err)
							continue
						}
						if nwritten != nread {
							logrus.Debugf("buffer: expected to buffer %d bytes, wrote %d", nread, nwritten)
							continue
						}
					}
					// If this is the last of the data we'll be able to read
					// from this descriptor, read as much as there is to read.
					for rfd.Revents&unix.POLLHUP == unix.POLLHUP {
						nr, err := unix.Read(int(rfd.Fd), b)
						util.LogIfUnexpectedWhileDraining(err, fmt.Sprintf("read %s: %v", fdDesc[int(rfd.Fd)], err))
						if nr <= 0 {
							break
						}
						if wfd, ok := relays[int(rfd.Fd)]; ok {
							nwritten, err := buffers[wfd].Write(b[:nr])
							if err != nil {
								logrus.Debugf("buffer: %v", err)
								break
							}
							if nwritten != nr {
								logrus.Debugf("buffer: expected to buffer %d bytes, wrote %d", nr, nwritten)
								break
							}
						}
					}
				}
				if nread == 0 {
					removeFds[int(rfd.Fd)] = struct{}{}
				}
			}
			pollTimeout = -1
			for wfd, buffer := range buffers {
				if buffer.Len() > 0 {
					nwritten, err := unix.Write(wfd, buffer.Bytes())
					util.LogIfNotRetryable(err, fmt.Sprintf("write %s: %v", fdDesc[wfd], err))
					if nwritten >= 0 {
						_ = buffer.Next(nwritten)
					}
				}
				if buffer.Len() > 0 {
					pollTimeout = 100
				}
				if wfd == relays[unix.Stdin] && stdinClose && buffer.Len() == 0 {
					stdinCopy.Close()
					delete(relays, unix.Stdin)
				}
			}
			for rfd := range removeFds {
				if rfd == unix.Stdin {
					buffer, found := buffers[relays[unix.Stdin]]
					if found && buffer.Len() > 0 {
						stdinClose = true
						continue
					}
				}
				if !options.Spec.Process.Terminal && rfd == unix.Stdin {
					stdinCopy.Close()
				}
				delete(relays, rfd)
			}
		}
	}()

	// Set up mounts and namespaces, and run the parent subprocess.
	status, err := runUsingChroot(options.Spec, options.BundlePath, ctty, stdin, stdout, stderr, closeOnceRunning)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running subprocess: %v\n", err)
		os.Exit(1)
	}

	// Pass the process's exit status back to the caller by exiting with the same status.
	if status.Exited() {
		if status.ExitStatus() != 0 {
			fmt.Fprintf(os.Stderr, "subprocess exited with status %d\n", status.ExitStatus())
		}
		os.Exit(status.ExitStatus())
	} else if status.Signaled() {
		fmt.Fprintf(os.Stderr, "subprocess exited on %s\n", status.Signal())
		os.Exit(1)
	}
}

// runUsingChroot, still in the grandparent process, sets up various bind
// mounts and then runs the parent process in its own user namespace with the
// necessary ID mappings.
func runUsingChroot(spec *specs.Spec, bundlePath string, ctty *os.File, stdin io.Reader, stdout, stderr io.Writer, closeOnceRunning []*os.File) (wstatus unix.WaitStatus, err error) {
	var confwg sync.WaitGroup

	// Create a new mount namespace for ourselves and bind mount everything to a new location.
	undoIntermediates, err := bind.SetupIntermediateMountNamespace(spec, bundlePath)
	if err != nil {
		return 1, err
	}
	defer func() {
		undoIntermediates()
	}()

	// Bind mount in our filesystems.
	undoChroots, err := setupChrootBindMounts(spec, bundlePath)
	if err != nil {
		return 1, err
	}
	defer func() {
		undoChroots()
	}()

	// Create a pipe for passing configuration down to the next process.
	preader, pwriter, err := os.Pipe()
	if err != nil {
		return 1, errors.Wrapf(err, "error creating configuration pipe")
	}
	config, conferr := json.Marshal(runUsingChrootExecSubprocOptions{
		Spec:       spec,
		BundlePath: bundlePath,
	})
	if conferr != nil {
		fmt.Fprintf(os.Stderr, "error re-encoding configuration for %q", runUsingChrootExecCommand)
		os.Exit(1)
	}

	// Apologize for the namespace configuration that we're about to ignore.
	logNamespaceDiagnostics(spec)

	// If we have configured ID mappings, set them here so that they can apply to the child.
	hostUidmap, hostGidmap, err := unshare.GetHostIDMappings("")
	if err != nil {
		return 1, err
	}
	uidmap, gidmap := spec.Linux.UIDMappings, spec.Linux.GIDMappings
	if len(uidmap) == 0 {
		// No UID mappings are configured for the container.  Borrow our parent's mappings.
		uidmap = append([]specs.LinuxIDMapping{}, hostUidmap...)
		for i := range uidmap {
			uidmap[i].HostID = uidmap[i].ContainerID
		}
	}
	if len(gidmap) == 0 {
		// No GID mappings are configured for the container.  Borrow our parent's mappings.
		gidmap = append([]specs.LinuxIDMapping{}, hostGidmap...)
		for i := range gidmap {
			gidmap[i].HostID = gidmap[i].ContainerID
		}
	}

	// Start the parent subprocess.
	cmd := unshare.Command(append([]string{runUsingChrootExecCommand}, spec.Process.Args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	cmd.Dir = "/"
	cmd.Env = append([]string{fmt.Sprintf("LOGLEVEL=%d", logrus.GetLevel())}, os.Environ()...)
	cmd.UnshareFlags = syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS
	requestedUserNS := false
	for _, ns := range spec.Linux.Namespaces {
		if ns.Type == specs.LinuxNamespaceType(specs.UserNamespace) {
			requestedUserNS = true
		}
	}
	if len(spec.Linux.UIDMappings) > 0 || len(spec.Linux.GIDMappings) > 0 || requestedUserNS {
		cmd.UnshareFlags = cmd.UnshareFlags | syscall.CLONE_NEWUSER
		cmd.UidMappings = uidmap
		cmd.GidMappings = gidmap
		cmd.GidMappingsEnableSetgroups = true
	}
	if ctty != nil {
		cmd.Setsid = true
		cmd.Ctty = ctty
	}
	cmd.OOMScoreAdj = spec.Process.OOMScoreAdj
	cmd.ExtraFiles = append([]*os.File{preader}, cmd.ExtraFiles...)
	cmd.Hook = func(int) error {
		for _, f := range closeOnceRunning {
			f.Close()
		}
		return nil
	}

	logrus.Debugf("Running %#v in %#v", cmd.Cmd, cmd)
	confwg.Add(1)
	go func() {
		_, conferr = io.Copy(pwriter, bytes.NewReader(config))
		pwriter.Close()
		confwg.Done()
	}()
	err = cmd.Run()
	confwg.Wait()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if waitStatus, ok := exitError.ProcessState.Sys().(syscall.WaitStatus); ok {
				if waitStatus.Exited() {
					if waitStatus.ExitStatus() != 0 {
						fmt.Fprintf(os.Stderr, "subprocess exited with status %d\n", waitStatus.ExitStatus())
					}
					os.Exit(waitStatus.ExitStatus())
				} else if waitStatus.Signaled() {
					fmt.Fprintf(os.Stderr, "subprocess exited on %s\n", waitStatus.Signal())
					os.Exit(1)
				}
			}
		}
		fmt.Fprintf(os.Stderr, "process exited with error: %v", err)
		os.Exit(1)
	}

	return 0, nil
}

// main() for parent subprocess.  Its main job is to try to make our
// environment look like the one described by the runtime configuration blob,
// and then launch the intended command as a child.
func runUsingChrootExecMain() {
	args := os.Args[1:]
	var options runUsingChrootExecSubprocOptions
	var err error

	runtime.LockOSThread()

	// Set logging.
	if level := os.Getenv("LOGLEVEL"); level != "" {
		if ll, err := strconv.Atoi(level); err == nil {
			logrus.SetLevel(logrus.Level(ll))
		}
		os.Unsetenv("LOGLEVEL")
	}

	// Unpack our configuration.
	confPipe := os.NewFile(3, "confpipe")
	if confPipe == nil {
		fmt.Fprintf(os.Stderr, "error reading options pipe\n")
		os.Exit(1)
	}
	defer confPipe.Close()
	if err := json.NewDecoder(confPipe).Decode(&options); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding options: %v\n", err)
		os.Exit(1)
	}

	// Set the hostname.  We're already in a distinct UTS namespace and are admins in the user
	// namespace which created it, so we shouldn't get a permissions error, but seccomp policy
	// might deny our attempt to call sethostname() anyway, so log a debug message for that.
	if options.Spec.Hostname != "" {
		if err := unix.Sethostname([]byte(options.Spec.Hostname)); err != nil {
			logrus.Debugf("failed to set hostname %q for process: %v", options.Spec.Hostname, err)
		}
	}

	// Try to chroot into the root.  Do this before we potentially block the syscall via the
	// seccomp profile.
	var oldst, newst unix.Stat_t
	if err := unix.Stat(options.Spec.Root.Path, &oldst); err != nil {
		fmt.Fprintf(os.Stderr, "error stat()ing intended root directory %q: %v\n", options.Spec.Root.Path, err)
		os.Exit(1)
	}
	if err := unix.Chdir(options.Spec.Root.Path); err != nil {
		fmt.Fprintf(os.Stderr, "error chdir()ing to intended root directory %q: %v\n", options.Spec.Root.Path, err)
		os.Exit(1)
	}
	if err := unix.Chroot(options.Spec.Root.Path); err != nil {
		fmt.Fprintf(os.Stderr, "error chroot()ing into directory %q: %v\n", options.Spec.Root.Path, err)
		os.Exit(1)
	}
	if err := unix.Stat("/", &newst); err != nil {
		fmt.Fprintf(os.Stderr, "error stat()ing current root directory: %v\n", err)
		os.Exit(1)
	}
	if oldst.Dev != newst.Dev || oldst.Ino != newst.Ino {
		fmt.Fprintf(os.Stderr, "unknown error chroot()ing into directory %q: %v\n", options.Spec.Root.Path, err)
		os.Exit(1)
	}
	logrus.Debugf("chrooted into %q", options.Spec.Root.Path)

	// not doing because it's still shared: creating devices
	// not doing because it's not applicable: setting annotations
	// not doing because it's still shared: setting sysctl settings
	// not doing because cgroupfs is read only: configuring control groups
	// -> this means we can use the freezer to make sure there aren't any lingering processes
	// -> this means we ignore cgroups-based controls
	// not doing because we don't set any in the config: running hooks
	// not doing because we don't set it in the config: setting rootfs read-only
	// not doing because we don't set it in the config: setting rootfs propagation
	logrus.Debugf("setting apparmor profile")
	if err = setApparmorProfile(options.Spec); err != nil {
		fmt.Fprintf(os.Stderr, "error setting apparmor profile for process: %v\n", err)
		os.Exit(1)
	}
	if err = setSelinuxLabel(options.Spec); err != nil {
		fmt.Fprintf(os.Stderr, "error setting SELinux label for process: %v\n", err)
		os.Exit(1)
	}

	logrus.Debugf("setting resource limits")
	if err = setRlimits(options.Spec, false, false); err != nil {
		fmt.Fprintf(os.Stderr, "error setting process resource limits for process: %v\n", err)
		os.Exit(1)
	}

	// Try to change to the directory.
	cwd := options.Spec.Process.Cwd
	if !filepath.IsAbs(cwd) {
		cwd = "/" + cwd
	}
	cwd = filepath.Clean(cwd)
	if err := unix.Chdir("/"); err != nil {
		fmt.Fprintf(os.Stderr, "error chdir()ing into new root directory %q: %v\n", options.Spec.Root.Path, err)
		os.Exit(1)
	}
	if err := unix.Chdir(cwd); err != nil {
		fmt.Fprintf(os.Stderr, "error chdir()ing into directory %q under root %q: %v\n", cwd, options.Spec.Root.Path, err)
		os.Exit(1)
	}
	logrus.Debugf("changed working directory to %q", cwd)

	// Drop privileges.
	user := options.Spec.Process.User
	if len(user.AdditionalGids) > 0 {
		gids := make([]int, len(user.AdditionalGids))
		for i := range user.AdditionalGids {
			gids[i] = int(user.AdditionalGids[i])
		}
		logrus.Debugf("setting supplemental groups")
		if err = syscall.Setgroups(gids); err != nil {
			fmt.Fprintf(os.Stderr, "error setting supplemental groups list: %v", err)
			os.Exit(1)
		}
	} else {
		logrus.Debugf("clearing supplemental groups")
		if err = syscall.Setgroups([]int{}); err != nil {
			fmt.Fprintf(os.Stderr, "error clearing supplemental groups list: %v", err)
			os.Exit(1)
		}
	}

	logrus.Debugf("setting gid")
	if err = syscall.Setresgid(int(user.GID), int(user.GID), int(user.GID)); err != nil {
		fmt.Fprintf(os.Stderr, "error setting GID: %v", err)
		os.Exit(1)
	}

	if err = setSeccomp(options.Spec); err != nil {
		fmt.Fprintf(os.Stderr, "error setting seccomp filter for process: %v\n", err)
		os.Exit(1)
	}

	logrus.Debugf("setting capabilities")
	var keepCaps []string
	if user.UID != 0 {
		keepCaps = []string{"CAP_SETUID"}
	}
	if err := setCapabilities(options.Spec, keepCaps...); err != nil {
		fmt.Fprintf(os.Stderr, "error setting capabilities for process: %v\n", err)
		os.Exit(1)
	}

	logrus.Debugf("setting uid")
	if err = syscall.Setresuid(int(user.UID), int(user.UID), int(user.UID)); err != nil {
		fmt.Fprintf(os.Stderr, "error setting UID: %v", err)
		os.Exit(1)
	}

	// Actually run the specified command.
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = options.Spec.Process.Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Dir = cwd
	logrus.Debugf("Running %#v (PATH = %q)", cmd, os.Getenv("PATH"))
	if err = cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if waitStatus, ok := exitError.ProcessState.Sys().(syscall.WaitStatus); ok {
				if waitStatus.Exited() {
					if waitStatus.ExitStatus() != 0 {
						fmt.Fprintf(os.Stderr, "subprocess exited with status %d\n", waitStatus.ExitStatus())
					}
					os.Exit(waitStatus.ExitStatus())
				} else if waitStatus.Signaled() {
					fmt.Fprintf(os.Stderr, "subprocess exited on %s\n", waitStatus.Signal())
					os.Exit(1)
				}
			}
		}
		fmt.Fprintf(os.Stderr, "process exited with error: %v", err)
		os.Exit(1)
	}
}

// logNamespaceDiagnostics knows which namespaces we want to create.
// Output debug messages when that differs from what we're being asked to do.
func logNamespaceDiagnostics(spec *specs.Spec) {
	sawMountNS := false
	sawUserNS := false
	sawUTSNS := false
	for _, ns := range spec.Linux.Namespaces {
		switch ns.Type {
		case specs.CgroupNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join cgroup namespace, sorry about that")
			} else {
				logrus.Debugf("unable to create cgroup namespace, sorry about that")
			}
		case specs.IPCNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join IPC namespace, sorry about that")
			} else {
				logrus.Debugf("unable to create IPC namespace, sorry about that")
			}
		case specs.MountNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join mount namespace %q, creating a new one", ns.Path)
			}
			sawMountNS = true
		case specs.NetworkNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join network namespace, sorry about that")
			} else {
				logrus.Debugf("unable to create network namespace, sorry about that")
			}
		case specs.PIDNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join PID namespace, sorry about that")
			} else {
				logrus.Debugf("unable to create PID namespace, sorry about that")
			}
		case specs.UserNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join user namespace %q, creating a new one", ns.Path)
			}
			sawUserNS = true
		case specs.UTSNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join UTS namespace %q, creating a new one", ns.Path)
			}
			sawUTSNS = true
		}
	}
	if !sawMountNS {
		logrus.Debugf("mount namespace not requested, but creating a new one anyway")
	}
	if !sawUserNS {
		logrus.Debugf("user namespace not requested, but creating a new one anyway")
	}
	if !sawUTSNS {
		logrus.Debugf("UTS namespace not requested, but creating a new one anyway")
	}
}

// setApparmorProfile sets the apparmor profile for ourselves, and hopefully any child processes that we'll start.
func setApparmorProfile(spec *specs.Spec) error {
	if !apparmor.IsEnabled() || spec.Process.ApparmorProfile == "" {
		return nil
	}
	if err := apparmor.ApplyProfile(spec.Process.ApparmorProfile); err != nil {
		return errors.Wrapf(err, "error setting apparmor profile to %q", spec.Process.ApparmorProfile)
	}
	return nil
}

// setCapabilities sets capabilities for ourselves, to be more or less inherited by any processes that we'll start.
func setCapabilities(spec *specs.Spec, keepCaps ...string) error {
	currentCaps, err := capability.NewPid(0)
	if err != nil {
		return errors.Wrapf(err, "error reading capabilities of current process")
	}
	caps, err := capability.NewPid(0)
	if err != nil {
		return errors.Wrapf(err, "error reading capabilities of current process")
	}
	capMap := map[capability.CapType][]string{
		capability.BOUNDING:    spec.Process.Capabilities.Bounding,
		capability.EFFECTIVE:   spec.Process.Capabilities.Effective,
		capability.INHERITABLE: spec.Process.Capabilities.Inheritable,
		capability.PERMITTED:   spec.Process.Capabilities.Permitted,
		capability.AMBIENT:     spec.Process.Capabilities.Ambient,
	}
	knownCaps := capability.List()
	caps.Clear(capability.CAPS | capability.BOUNDS | capability.AMBS)
	for capType, capList := range capMap {
		for _, capToSet := range capList {
			cap := capability.CAP_LAST_CAP
			for _, c := range knownCaps {
				if strings.EqualFold("CAP_"+c.String(), capToSet) {
					cap = c
					break
				}
			}
			if cap == capability.CAP_LAST_CAP {
				return errors.Errorf("error mapping capability %q to a number", capToSet)
			}
			caps.Set(capType, cap)
		}
		for _, capToSet := range keepCaps {
			cap := capability.CAP_LAST_CAP
			for _, c := range knownCaps {
				if strings.EqualFold("CAP_"+c.String(), capToSet) {
					cap = c
					break
				}
			}
			if cap == capability.CAP_LAST_CAP {
				return errors.Errorf("error mapping capability %q to a number", capToSet)
			}
			if currentCaps.Get(capType, cap) {
				caps.Set(capType, cap)
			}
		}
	}
	if err = caps.Apply(capability.CAPS | capability.BOUNDS | capability.AMBS); err != nil {
		return errors.Wrapf(err, "error setting capabilities")
	}
	return nil
}

// parses the resource limits for ourselves and any processes that
// we'll start into a format that's more in line with the kernel APIs
func parseRlimits(spec *specs.Spec) (map[int]unix.Rlimit, error) {
	if spec.Process == nil {
		return nil, nil
	}
	parsed := make(map[int]unix.Rlimit)
	for _, limit := range spec.Process.Rlimits {
		resource, recognized := rlimitsMap[strings.ToUpper(limit.Type)]
		if !recognized {
			return nil, errors.Errorf("error parsing limit type %q", limit.Type)
		}
		parsed[resource] = unix.Rlimit{Cur: limit.Soft, Max: limit.Hard}
	}
	return parsed, nil
}

// setRlimits sets any resource limits that we want to apply to processes that
// we'll start.
func setRlimits(spec *specs.Spec, onlyLower, onlyRaise bool) error {
	limits, err := parseRlimits(spec)
	if err != nil {
		return err
	}
	for resource, desired := range limits {
		var current unix.Rlimit
		if err := unix.Getrlimit(resource, &current); err != nil {
			return errors.Wrapf(err, "error reading %q limit", rlimitsReverseMap[resource])
		}
		if desired.Max > current.Max && onlyLower {
			// this would raise a hard limit, and we're only here to lower them
			continue
		}
		if desired.Max < current.Max && onlyRaise {
			// this would lower a hard limit, and we're only here to raise them
			continue
		}
		if err := unix.Setrlimit(resource, &desired); err != nil {
			return errors.Wrapf(err, "error setting %q limit to soft=%d,hard=%d (was soft=%d,hard=%d)", rlimitsReverseMap[resource], desired.Cur, desired.Max, current.Cur, current.Max)
		}
	}
	return nil
}

func makeReadOnly(mntpoint string, flags uintptr) error {
	var fs unix.Statfs_t
	// Make sure it's read-only.
	if err := unix.Statfs(mntpoint, &fs); err != nil {
		return errors.Wrapf(err, "error checking if directory %q was bound read-only", mntpoint)
	}
	if fs.Flags&unix.ST_RDONLY == 0 {
		if err := unix.Mount(mntpoint, mntpoint, "bind", flags|unix.MS_REMOUNT, ""); err != nil {
			return errors.Wrapf(err, "error remounting %s in mount namespace read-only", mntpoint)
		}
	}
	return nil
}

// setupChrootBindMounts actually bind mounts things under the rootfs, and returns a
// callback that will clean up its work.
func setupChrootBindMounts(spec *specs.Spec, bundlePath string) (undoBinds func() error, err error) {
	var fs unix.Statfs_t
	removes := []string{}
	undoBinds = func() error {
		if err2 := bind.UnmountMountpoints(spec.Root.Path, removes); err2 != nil {
			logrus.Warnf("pkg/chroot: error unmounting %q: %v", spec.Root.Path, err2)
			if err == nil {
				err = err2
			}
		}
		return err
	}

	// Now bind mount all of those things to be under the rootfs's location in this
	// mount namespace.
	commonFlags := uintptr(unix.MS_BIND | unix.MS_REC | unix.MS_PRIVATE)
	bindFlags := commonFlags | unix.MS_NODEV
	devFlags := commonFlags | unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_RDONLY
	procFlags := devFlags | unix.MS_NODEV
	sysFlags := devFlags | unix.MS_NODEV

	// Bind /dev read-only.
	subDev := filepath.Join(spec.Root.Path, "/dev")
	if err := unix.Mount("/dev", subDev, "bind", devFlags, ""); err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(subDev, 0700)
			if err == nil {
				err = unix.Mount("/dev", subDev, "bind", devFlags, "")
			}
		}
		if err != nil {
			return undoBinds, errors.Wrapf(err, "error bind mounting /dev from host into mount namespace")
		}
	}
	// Make sure it's read-only.
	if err = unix.Statfs(subDev, &fs); err != nil {
		return undoBinds, errors.Wrapf(err, "error checking if directory %q was bound read-only", subDev)
	}
	if fs.Flags&unix.ST_RDONLY == 0 {
		if err := unix.Mount(subDev, subDev, "bind", devFlags|unix.MS_REMOUNT, ""); err != nil {
			return undoBinds, errors.Wrapf(err, "error remounting /dev in mount namespace read-only")
		}
	}
	logrus.Debugf("bind mounted %q to %q", "/dev", filepath.Join(spec.Root.Path, "/dev"))

	// Bind /proc read-only.
	subProc := filepath.Join(spec.Root.Path, "/proc")
	if err := unix.Mount("/proc", subProc, "bind", procFlags, ""); err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(subProc, 0700)
			if err == nil {
				err = unix.Mount("/proc", subProc, "bind", procFlags, "")
			}
		}
		if err != nil {
			return undoBinds, errors.Wrapf(err, "error bind mounting /proc from host into mount namespace")
		}
	}
	logrus.Debugf("bind mounted %q to %q", "/proc", filepath.Join(spec.Root.Path, "/proc"))

	// Bind /sys read-only.
	subSys := filepath.Join(spec.Root.Path, "/sys")
	if err := unix.Mount("/sys", subSys, "bind", sysFlags, ""); err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(subSys, 0700)
			if err == nil {
				err = unix.Mount("/sys", subSys, "bind", sysFlags, "")
			}
		}
		if err != nil {
			return undoBinds, errors.Wrapf(err, "error bind mounting /sys from host into mount namespace")
		}
	}
	if err := makeReadOnly(subSys, sysFlags); err != nil {
		return undoBinds, err
	}

	mnts, _ := mount.GetMounts()
	for _, m := range mnts {
		if !strings.HasPrefix(m.Mountpoint, "/sys/") &&
			m.Mountpoint != "/sys" {
			continue
		}
		subSys := filepath.Join(spec.Root.Path, m.Mountpoint)
		if err := unix.Mount(m.Mountpoint, subSys, "bind", sysFlags, ""); err != nil {
			return undoBinds, errors.Wrapf(err, "error bind mounting /sys from host into mount namespace")
		}
		if err := makeReadOnly(subSys, sysFlags); err != nil {
			return undoBinds, err
		}
	}
	logrus.Debugf("bind mounted %q to %q", "/sys", filepath.Join(spec.Root.Path, "/sys"))

	// Add /sys/fs/selinux to the set of masked paths, to ensure that we don't have processes
	// attempting to interact with labeling, when they aren't allowed to do so.
	spec.Linux.MaskedPaths = append(spec.Linux.MaskedPaths, "/sys/fs/selinux")
	// Bind mount in everything we've been asked to mount.
	for _, m := range spec.Mounts {
		// Skip anything that we just mounted.
		switch m.Destination {
		case "/dev", "/proc", "/sys":
			logrus.Debugf("already bind mounted %q on %q", m.Destination, filepath.Join(spec.Root.Path, m.Destination))
			continue
		default:
			if strings.HasPrefix(m.Destination, "/dev/") {
				continue
			}
			if strings.HasPrefix(m.Destination, "/proc/") {
				continue
			}
			if strings.HasPrefix(m.Destination, "/sys/") {
				continue
			}
		}
		// Skip anything that isn't a bind or tmpfs mount.
		if m.Type != "bind" && m.Type != "tmpfs" && m.Type != "overlay" {
			logrus.Debugf("skipping mount of type %q on %q", m.Type, m.Destination)
			continue
		}
		// If the target is there, we can just mount it.
		var srcinfo os.FileInfo
		switch m.Type {
		case "bind":
			srcinfo, err = os.Stat(m.Source)
			if err != nil {
				return undoBinds, errors.Wrapf(err, "error examining %q for mounting in mount namespace", m.Source)
			}
		case "overlay":
			fallthrough
		case "tmpfs":
			srcinfo, err = os.Stat("/")
			if err != nil {
				return undoBinds, errors.Wrapf(err, "error examining / to use as a template for a %s", m.Type)
			}
		}
		target := filepath.Join(spec.Root.Path, m.Destination)
		if _, err := os.Stat(target); err != nil {
			// If the target can't be stat()ted, check the error.
			if !os.IsNotExist(err) {
				return undoBinds, errors.Wrapf(err, "error examining %q for mounting in mount namespace", target)
			}
			// The target isn't there yet, so create it, and make a
			// note to remove it later.
			if srcinfo.IsDir() {
				if err = os.MkdirAll(target, 0111); err != nil {
					return undoBinds, errors.Wrapf(err, "error creating mountpoint %q in mount namespace", target)
				}
				removes = append(removes, target)
			} else {
				if err = os.MkdirAll(filepath.Dir(target), 0111); err != nil {
					return undoBinds, errors.Wrapf(err, "error ensuring parent of mountpoint %q (%q) is present in mount namespace", target, filepath.Dir(target))
				}
				var file *os.File
				if file, err = os.OpenFile(target, os.O_WRONLY|os.O_CREATE, 0); err != nil {
					return undoBinds, errors.Wrapf(err, "error creating mountpoint %q in mount namespace", target)
				}
				file.Close()
				removes = append(removes, target)
			}
		}
		requestFlags := bindFlags
		expectedFlags := uintptr(0)
		if util.StringInSlice("nodev", m.Options) {
			requestFlags |= unix.MS_NODEV
			expectedFlags |= unix.ST_NODEV
		}
		if util.StringInSlice("noexec", m.Options) {
			requestFlags |= unix.MS_NOEXEC
			expectedFlags |= unix.ST_NOEXEC
		}
		if util.StringInSlice("nosuid", m.Options) {
			requestFlags |= unix.MS_NOSUID
			expectedFlags |= unix.ST_NOSUID
		}
		if util.StringInSlice("ro", m.Options) {
			requestFlags |= unix.MS_RDONLY
			expectedFlags |= unix.ST_RDONLY
		}
		switch m.Type {
		case "bind":
			// Do the bind mount.
			if err := unix.Mount(m.Source, target, "", requestFlags, ""); err != nil {
				return undoBinds, errors.Wrapf(err, "error bind mounting %q from host to %q in mount namespace (%q)", m.Source, m.Destination, target)
			}
			logrus.Debugf("bind mounted %q to %q", m.Source, target)
		case "tmpfs":
			// Mount a tmpfs.
			if err := mount.Mount(m.Source, target, m.Type, strings.Join(append(m.Options, "private"), ",")); err != nil {
				return undoBinds, errors.Wrapf(err, "error mounting tmpfs to %q in mount namespace (%q, %q)", m.Destination, target, strings.Join(m.Options, ","))
			}
			logrus.Debugf("mounted a tmpfs to %q", target)
		case "overlay":
			// Mount a overlay.
			if err := mount.Mount(m.Source, target, m.Type, strings.Join(append(m.Options, "private"), ",")); err != nil {
				return undoBinds, errors.Wrapf(err, "error mounting overlay to %q in mount namespace (%q, %q)", m.Destination, target, strings.Join(m.Options, ","))
			}
			logrus.Debugf("mounted a overlay to %q", target)
		}
		if err = unix.Statfs(target, &fs); err != nil {
			return undoBinds, errors.Wrapf(err, "error checking if directory %q was bound read-only", target)
		}
		if uintptr(fs.Flags)&expectedFlags != expectedFlags {
			if err := unix.Mount(target, target, "bind", requestFlags|unix.MS_REMOUNT, ""); err != nil {
				return undoBinds, errors.Wrapf(err, "error remounting %q in mount namespace with expected flags", target)
			}
		}
	}

	// Set up any read-only paths that we need to.  If we're running inside
	// of a container, some of these locations will already be read-only.
	for _, roPath := range spec.Linux.ReadonlyPaths {
		r := filepath.Join(spec.Root.Path, roPath)
		target, err := filepath.EvalSymlinks(r)
		if err != nil {
			if os.IsNotExist(err) {
				// No target, no problem.
				continue
			}
			return undoBinds, errors.Wrapf(err, "error checking %q for symlinks before marking it read-only", r)
		}
		// Check if the location is already read-only.
		var fs unix.Statfs_t
		if err = unix.Statfs(target, &fs); err != nil {
			if os.IsNotExist(err) {
				// No target, no problem.
				continue
			}
			return undoBinds, errors.Wrapf(err, "error checking if directory %q is already read-only", target)
		}
		if fs.Flags&unix.ST_RDONLY != 0 {
			continue
		}
		// Mount the location over itself, so that we can remount it as read-only.
		roFlags := uintptr(unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_RDONLY)
		if err := unix.Mount(target, target, "", roFlags|unix.MS_BIND|unix.MS_REC, ""); err != nil {
			if os.IsNotExist(err) {
				// No target, no problem.
				continue
			}
			return undoBinds, errors.Wrapf(err, "error bind mounting %q onto itself in preparation for making it read-only", target)
		}
		// Remount the location read-only.
		if err = unix.Statfs(target, &fs); err != nil {
			return undoBinds, errors.Wrapf(err, "error checking if directory %q was bound read-only", target)
		}
		if fs.Flags&unix.ST_RDONLY == 0 {
			if err := unix.Mount(target, target, "", roFlags|unix.MS_BIND|unix.MS_REMOUNT, ""); err != nil {
				return undoBinds, errors.Wrapf(err, "error remounting %q in mount namespace read-only", target)
			}
		}
		// Check again.
		if err = unix.Statfs(target, &fs); err != nil {
			return undoBinds, errors.Wrapf(err, "error checking if directory %q was remounted read-only", target)
		}
		if fs.Flags&unix.ST_RDONLY == 0 {
			return undoBinds, errors.Wrapf(err, "error verifying that %q in mount namespace was remounted read-only", target)
		}
	}

	// Create an empty directory for to use for masking directories.
	roEmptyDir := filepath.Join(bundlePath, "empty")
	if len(spec.Linux.MaskedPaths) > 0 {
		if err := os.Mkdir(roEmptyDir, 0700); err != nil {
			return undoBinds, errors.Wrapf(err, "error creating empty directory %q", roEmptyDir)
		}
		removes = append(removes, roEmptyDir)
	}

	// Set up any masked paths that we need to.  If we're running inside of
	// a container, some of these locations will already be read-only tmpfs
	// filesystems or bind mounted to os.DevNull.  If we're not running
	// inside of a container, and nobody else has done that, we'll do it.
	for _, masked := range spec.Linux.MaskedPaths {
		t := filepath.Join(spec.Root.Path, masked)
		target, err := filepath.EvalSymlinks(t)
		if err != nil {
			target = t
		}
		// Get some info about the null device.
		nullinfo, err := os.Stat(os.DevNull)
		if err != nil {
			return undoBinds, errors.Wrapf(err, "error examining %q for masking in mount namespace", os.DevNull)
		}
		// Get some info about the target.
		targetinfo, err := os.Stat(target)
		if err != nil {
			if os.IsNotExist(err) {
				// No target, no problem.
				continue
			}
			return undoBinds, errors.Wrapf(err, "error examining %q for masking in mount namespace", target)
		}
		if targetinfo.IsDir() {
			// The target's a directory.  Check if it's a read-only filesystem.
			var statfs unix.Statfs_t
			if err = unix.Statfs(target, &statfs); err != nil {
				return undoBinds, errors.Wrapf(err, "error checking if directory %q is a mountpoint", target)
			}
			isReadOnly := statfs.Flags&unix.MS_RDONLY != 0
			// Check if any of the IDs we're mapping could read it.
			isAccessible := true
			var stat unix.Stat_t
			if err = unix.Stat(target, &stat); err != nil {
				return undoBinds, errors.Wrapf(err, "error checking permissions on directory %q", target)
			}
			isAccessible = false
			if stat.Mode&unix.S_IROTH|unix.S_IXOTH != 0 {
				isAccessible = true
			}
			if !isAccessible && stat.Mode&unix.S_IROTH|unix.S_IXOTH != 0 {
				if len(spec.Linux.GIDMappings) > 0 {
					for _, mapping := range spec.Linux.GIDMappings {
						if stat.Gid >= mapping.ContainerID && stat.Gid < mapping.ContainerID+mapping.Size {
							isAccessible = true
							break
						}
					}
				}
			}
			if !isAccessible && stat.Mode&unix.S_IRUSR|unix.S_IXUSR != 0 {
				if len(spec.Linux.UIDMappings) > 0 {
					for _, mapping := range spec.Linux.UIDMappings {
						if stat.Uid >= mapping.ContainerID && stat.Uid < mapping.ContainerID+mapping.Size {
							isAccessible = true
							break
						}
					}
				}
			}
			// Check if it's empty.
			hasContent := false
			directory, err := os.Open(target)
			if err != nil {
				if !os.IsPermission(err) {
					return undoBinds, errors.Wrapf(err, "error opening directory %q", target)
				}
			} else {
				names, err := directory.Readdirnames(0)
				directory.Close()
				if err != nil {
					return undoBinds, errors.Wrapf(err, "error reading contents of directory %q", target)
				}
				hasContent = false
				for _, name := range names {
					switch name {
					case ".", "..":
						continue
					default:
						hasContent = true
					}
					if hasContent {
						break
					}
				}
			}
			// The target's a directory, so read-only bind mount an empty directory on it.
			roFlags := uintptr(syscall.MS_BIND | syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_RDONLY)
			if !isReadOnly || (hasContent && isAccessible) {
				if err = unix.Mount(roEmptyDir, target, "bind", roFlags, ""); err != nil {
					return undoBinds, errors.Wrapf(err, "error masking directory %q in mount namespace", target)
				}
				if err = unix.Statfs(target, &fs); err != nil {
					return undoBinds, errors.Wrapf(err, "error checking if directory %q was mounted read-only in mount namespace", target)
				}
				if fs.Flags&unix.ST_RDONLY == 0 {
					if err = unix.Mount(target, target, "", roFlags|syscall.MS_REMOUNT, ""); err != nil {
						return undoBinds, errors.Wrapf(err, "error making sure directory %q in mount namespace is read only", target)
					}
				}
			}
		} else {
			// The target's not a directory, so bind mount os.DevNull over it, unless it's already os.DevNull.
			if !os.SameFile(nullinfo, targetinfo) {
				if err = unix.Mount(os.DevNull, target, "", uintptr(syscall.MS_BIND|syscall.MS_RDONLY|syscall.MS_PRIVATE), ""); err != nil {
					return undoBinds, errors.Wrapf(err, "error masking non-directory %q in mount namespace", target)
				}
			}
		}
	}
	return undoBinds, nil
}
