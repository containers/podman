// +build linux,cgo

package rootless

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	gosignal "os/signal"
	"os/user"
	"runtime"
	"strconv"
	"sync"
	"unsafe"

	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/containers/storage/pkg/idtools"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

/*
#cgo remoteclient CFLAGS: -Wall -Werror -DDISABLE_JOIN_SHORTCUT
#include <stdlib.h>
#include <sys/types.h>
extern uid_t rootless_uid();
extern uid_t rootless_gid();
extern int reexec_in_user_namespace(int ready, char *pause_pid_file_path, char *file_to_read, int fd);
extern int reexec_in_user_namespace_wait(int pid, int options);
extern int reexec_userns_join(int userns, int mountns, char *pause_pid_file_path);
*/
import "C"

const (
	numSig = 65 // max number of signals
)

func runInUser() error {
	return os.Setenv("_CONTAINERS_USERNS_CONFIGURED", "done")
}

var (
	isRootlessOnce sync.Once
	isRootless     bool
)

// IsRootless tells us if we are running in rootless mode
func IsRootless() bool {
	isRootlessOnce.Do(func() {
		rootlessUIDInit := int(C.rootless_uid())
		rootlessGIDInit := int(C.rootless_gid())
		if rootlessUIDInit != 0 {
			// This happens if we joined the user+mount namespace as part of
			if err := os.Setenv("_CONTAINERS_USERNS_CONFIGURED", "done"); err != nil {
				logrus.Errorf("failed to set environment variable %s as %s", "_CONTAINERS_USERNS_CONFIGURED", "done")
			}
			if err := os.Setenv("_CONTAINERS_ROOTLESS_UID", fmt.Sprintf("%d", rootlessUIDInit)); err != nil {
				logrus.Errorf("failed to set environment variable %s as %d", "_CONTAINERS_ROOTLESS_UID", rootlessUIDInit)
			}
			if err := os.Setenv("_CONTAINERS_ROOTLESS_GID", fmt.Sprintf("%d", rootlessGIDInit)); err != nil {
				logrus.Errorf("failed to set environment variable %s as %d", "_CONTAINERS_ROOTLESS_GID", rootlessGIDInit)
			}
		}
		isRootless = os.Geteuid() != 0 || os.Getenv("_CONTAINERS_USERNS_CONFIGURED") != ""
	})
	return isRootless
}

// GetRootlessUID returns the UID of the user in the parent userNS
func GetRootlessUID() int {
	uidEnv := os.Getenv("_CONTAINERS_ROOTLESS_UID")
	if uidEnv != "" {
		u, _ := strconv.Atoi(uidEnv)
		return u
	}
	return os.Geteuid()
}

// GetRootlessGID returns the GID of the user in the parent userNS
func GetRootlessGID() int {
	gidEnv := os.Getenv("_CONTAINERS_ROOTLESS_GID")
	if gidEnv != "" {
		u, _ := strconv.Atoi(gidEnv)
		return u
	}

	/* If the _CONTAINERS_ROOTLESS_UID is set, assume the gid==uid.  */
	uidEnv := os.Getenv("_CONTAINERS_ROOTLESS_UID")
	if uidEnv != "" {
		u, _ := strconv.Atoi(uidEnv)
		return u
	}
	return os.Getegid()
}

func tryMappingTool(tool string, pid int, hostID int, mappings []idtools.IDMap) error {
	path, err := exec.LookPath(tool)
	if err != nil {
		return errors.Wrapf(err, "cannot find %s", tool)
	}

	appendTriplet := func(l []string, a, b, c int) []string {
		return append(l, strconv.Itoa(a), strconv.Itoa(b), strconv.Itoa(c))
	}

	args := []string{path, fmt.Sprintf("%d", pid)}
	args = appendTriplet(args, 0, hostID, 1)
	for _, i := range mappings {
		args = appendTriplet(args, i.ContainerID+1, i.HostID, i.Size)
	}
	cmd := exec.Cmd{
		Path: path,
		Args: args,
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		logrus.Debugf("error from %s: %s", tool, output)
		return errors.Wrapf(err, "cannot setup namespace using %s", tool)
	}
	return nil
}

func readUserNs(path string) (string, error) {
	b := make([]byte, 256)
	_, err := unix.Readlink(path, b)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func readUserNsFd(fd uintptr) (string, error) {
	return readUserNs(fmt.Sprintf("/proc/self/fd/%d", fd))
}

func getParentUserNs(fd uintptr) (uintptr, error) {
	const nsGetParent = 0xb702
	ret, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, uintptr(nsGetParent), 0)
	if errno != 0 {
		return 0, errno
	}
	return (uintptr)(unsafe.Pointer(ret)), nil
}

// getUserNSFirstChild returns an open FD for the first direct child user namespace that created the process
// Each container creates a new user namespace where the runtime runs.  The current process in the container
// might have created new user namespaces that are child of the initial namespace we created.
// This function finds the initial namespace created for the container that is a child of the current namespace.
//
//                                     current ns
//                                       /     \
//                           TARGET ->  a   [other containers]
//                                     /
//                                    b
//                                   /
//        NS READ USING THE PID ->  c
func getUserNSFirstChild(fd uintptr) (*os.File, error) {
	currentNS, err := readUserNs("/proc/self/ns/user")
	if err != nil {
		return nil, err
	}

	ns, err := readUserNsFd(fd)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read user namespace")
	}
	if ns == currentNS {
		return nil, errors.New("process running in the same user namespace")
	}

	for {
		nextFd, err := getParentUserNs(fd)
		if err != nil {
			if err == unix.ENOTTY {
				return os.NewFile(fd, "userns child"), nil
			}
			return nil, errors.Wrapf(err, "cannot get parent user namespace")
		}

		ns, err = readUserNsFd(nextFd)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read user namespace")
		}

		if ns == currentNS {
			if err := unix.Close(int(nextFd)); err != nil {
				return nil, err
			}

			// Drop O_CLOEXEC for the fd.
			_, _, errno := unix.Syscall(unix.SYS_FCNTL, fd, unix.F_SETFD, 0)
			if errno != 0 {
				if err := unix.Close(int(fd)); err != nil {
					logrus.Errorf("failed to close file descriptor %d", fd)
				}
				return nil, errno
			}

			return os.NewFile(fd, "userns child"), nil
		}
		if err := unix.Close(int(fd)); err != nil {
			return nil, err
		}
		fd = nextFd
	}
}

// joinUserAndMountNS re-exec podman in a new userNS and join the user and mount
// namespace of the specified PID without looking up its parent.  Useful to join directly
// the conmon process.
func joinUserAndMountNS(pid uint, pausePid string) (bool, int, error) {
	if os.Geteuid() == 0 || os.Getenv("_CONTAINERS_USERNS_CONFIGURED") != "" {
		return false, -1, nil
	}

	cPausePid := C.CString(pausePid)
	defer C.free(unsafe.Pointer(cPausePid))

	userNS, err := os.Open(fmt.Sprintf("/proc/%d/ns/user", pid))
	if err != nil {
		return false, -1, err
	}
	defer func() {
		if err := userNS.Close(); err != nil {
			logrus.Errorf("unable to close namespace: %q", err)
		}
	}()

	mountNS, err := os.Open(fmt.Sprintf("/proc/%d/ns/mnt", pid))
	if err != nil {
		return false, -1, err
	}
	defer func() {
		if err := mountNS.Close(); err != nil {
			logrus.Errorf("unable to close namespace: %q", err)
		}
	}()

	fd, err := getUserNSFirstChild(userNS.Fd())
	if err != nil {
		return false, -1, err
	}
	pidC := C.reexec_userns_join(C.int(fd.Fd()), C.int(mountNS.Fd()), cPausePid)
	if int(pidC) < 0 {
		return false, -1, errors.Errorf("cannot re-exec process")
	}

	ret := C.reexec_in_user_namespace_wait(pidC, 0)
	if ret < 0 {
		return false, -1, errors.New("error waiting for the re-exec process")
	}

	return true, int(ret), nil
}

// GetConfiguredMappings returns the additional IDs configured for the current user.
func GetConfiguredMappings() ([]idtools.IDMap, []idtools.IDMap, error) {
	var uids, gids []idtools.IDMap
	username := os.Getenv("USER")
	if username == "" {
		var id string
		if os.Geteuid() == 0 {
			id = strconv.Itoa(GetRootlessUID())
		} else {
			id = strconv.Itoa(os.Geteuid())
		}
		userID, err := user.LookupId(id)
		if err == nil {
			username = userID.Username
		}
	}
	mappings, err := idtools.NewIDMappings(username, username)
	if err != nil {
		logrus.Errorf("cannot find mappings for user %s: %v", username, err)
	} else {
		uids = mappings.UIDs()
		gids = mappings.GIDs()
	}
	return uids, gids, nil
}

func becomeRootInUserNS(pausePid, fileToRead string, fileOutput *os.File) (bool, int, error) {
	if os.Geteuid() == 0 || os.Getenv("_CONTAINERS_USERNS_CONFIGURED") != "" {
		if os.Getenv("_CONTAINERS_USERNS_CONFIGURED") == "init" {
			return false, 0, runInUser()
		}
		return false, 0, nil
	}

	cPausePid := C.CString(pausePid)
	defer C.free(unsafe.Pointer(cPausePid))

	cFileToRead := C.CString(fileToRead)
	defer C.free(unsafe.Pointer(cFileToRead))
	var fileOutputFD C.int
	if fileOutput != nil {
		fileOutputFD = C.int(fileOutput.Fd())
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
	if err != nil {
		return false, -1, err
	}
	r, w := os.NewFile(uintptr(fds[0]), "sync host"), os.NewFile(uintptr(fds[1]), "sync child")

	defer errorhandling.CloseQuiet(r)
	defer errorhandling.CloseQuiet(w)
	defer func() {
		if _, err := w.Write([]byte("0")); err != nil {
			logrus.Errorf("failed to write byte 0: %q", err)
		}
	}()

	pidC := C.reexec_in_user_namespace(C.int(r.Fd()), cPausePid, cFileToRead, fileOutputFD)
	pid := int(pidC)
	if pid < 0 {
		return false, -1, errors.Errorf("cannot re-exec process")
	}

	uids, gids, err := GetConfiguredMappings()
	if err != nil {
		return false, -1, err
	}

	uidsMapped := false
	if uids != nil {
		err := tryMappingTool("newuidmap", pid, os.Geteuid(), uids)
		uidsMapped = err == nil
	}
	if !uidsMapped {
		logrus.Warnf("using rootless single mapping into the namespace. This might break some images. Check /etc/subuid and /etc/subgid for adding subids")
		setgroups := fmt.Sprintf("/proc/%d/setgroups", pid)
		err = ioutil.WriteFile(setgroups, []byte("deny\n"), 0666)
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot write setgroups file")
		}
		logrus.Debugf("write setgroups file exited with 0")

		uidMap := fmt.Sprintf("/proc/%d/uid_map", pid)
		err = ioutil.WriteFile(uidMap, []byte(fmt.Sprintf("%d %d 1\n", 0, os.Geteuid())), 0666)
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot write uid_map")
		}
		logrus.Debugf("write uid_map exited with 0")
	}

	gidsMapped := false
	if gids != nil {
		err := tryMappingTool("newgidmap", pid, os.Getegid(), gids)
		gidsMapped = err == nil
	}
	if !gidsMapped {
		gidMap := fmt.Sprintf("/proc/%d/gid_map", pid)
		err = ioutil.WriteFile(gidMap, []byte(fmt.Sprintf("%d %d 1\n", 0, os.Getegid())), 0666)
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot write gid_map")
		}
	}

	_, err = w.Write([]byte("0"))
	if err != nil {
		return false, -1, errors.Wrapf(err, "write to sync pipe")
	}

	b := make([]byte, 1)
	_, err = w.Read(b)
	if err != nil {
		return false, -1, errors.Wrapf(err, "read from sync pipe")
	}

	if fileOutput != nil {
		return true, 0, nil
	}

	if b[0] == '2' {
		// We have lost the race for writing the PID file, as probably another
		// process created a namespace and wrote the PID.
		// Try to join it.
		data, err := ioutil.ReadFile(pausePid)
		if err == nil {
			pid, err := strconv.ParseUint(string(data), 10, 0)
			if err == nil {
				return joinUserAndMountNS(uint(pid), "")
			}
		}
		return false, -1, errors.Wrapf(err, "error setting up the process")
	}

	if b[0] != '0' {
		return false, -1, errors.Wrapf(err, "error setting up the process")
	}

	c := make(chan os.Signal, 1)

	signals := []os.Signal{}
	for sig := 0; sig < numSig; sig++ {
		if sig == int(unix.SIGTSTP) {
			continue
		}
		signals = append(signals, unix.Signal(sig))
	}

	gosignal.Notify(c, signals...)
	defer gosignal.Reset()
	go func() {
		for s := range c {
			if s == unix.SIGCHLD || s == unix.SIGPIPE {
				continue
			}

			if err := unix.Kill(int(pidC), s.(unix.Signal)); err != nil {
				logrus.Errorf("failed to kill %d", int(pidC))
			}
		}
	}()

	ret := C.reexec_in_user_namespace_wait(pidC, 0)
	if ret < 0 {
		return false, -1, errors.New("error waiting for the re-exec process")
	}

	return true, int(ret), nil
}

// BecomeRootInUserNS re-exec podman in a new userNS.  It returns whether podman was re-executed
// into a new user namespace and the return code from the re-executed podman process.
// If podman was re-executed the caller needs to propagate the error code returned by the child
// process.
func BecomeRootInUserNS(pausePid string) (bool, int, error) {
	return becomeRootInUserNS(pausePid, "", nil)
}

// TryJoinFromFilePaths attempts to join the namespaces of the pid files in paths.
// This is useful when there are already running containers and we
// don't have a pause process yet.  We can use the paths to the conmon
// processes to attempt joining their namespaces.
// If needNewNamespace is set, the file is read from a temporary user
// namespace, this is useful for containers that are running with a
// different uidmap and the unprivileged user has no way to read the
// file owned by the root in the container.
func TryJoinFromFilePaths(pausePidPath string, needNewNamespace bool, paths []string) (bool, int, error) {
	if len(paths) == 0 {
		return BecomeRootInUserNS(pausePidPath)
	}

	var lastErr error
	var pausePid int

	for _, path := range paths {
		if !needNewNamespace {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				lastErr = err
				continue
			}

			pausePid, err = strconv.Atoi(string(data))
			if err != nil {
				lastErr = errors.Wrapf(err, "cannot parse file %s", path)
				continue
			}

			lastErr = nil
			break
		} else {
			fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM, 0)
			if err != nil {
				lastErr = err
				continue
			}

			r, w := os.NewFile(uintptr(fds[0]), "read file"), os.NewFile(uintptr(fds[1]), "write file")

			defer errorhandling.CloseQuiet(r)

			if _, _, err := becomeRootInUserNS("", path, w); err != nil {
				w.Close()
				lastErr = err
				continue
			}

			if err := w.Close(); err != nil {
				return false, 0, err
			}
			defer func() {
				C.reexec_in_user_namespace_wait(-1, 0)
			}()

			b := make([]byte, 32)

			n, err := r.Read(b)
			if err != nil {
				lastErr = errors.Wrapf(err, "cannot read %s\n", path)
				continue
			}

			pausePid, err = strconv.Atoi(string(b[:n]))
			if err == nil {
				lastErr = nil
				break
			}
		}
	}
	if lastErr != nil {
		return false, 0, lastErr
	}

	return joinUserAndMountNS(uint(pausePid), pausePidPath)
}
func ReadMappingsProc(path string) ([]idtools.IDMap, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open %s", path)
	}
	defer file.Close()

	mappings := []idtools.IDMap{}

	buf := bufio.NewReader(file)
	for {
		line, _, err := buf.ReadLine()
		if err != nil {
			if err == io.EOF {
				return mappings, nil
			}
			return nil, errors.Wrapf(err, "cannot read line from %s", path)
		}
		if line == nil {
			return mappings, nil
		}

		containerID, hostID, size := 0, 0, 0
		if _, err := fmt.Sscanf(string(line), "%d %d %d", &containerID, &hostID, &size); err != nil {
			return nil, errors.Wrapf(err, "cannot parse %s", string(line))
		}
		mappings = append(mappings, idtools.IDMap{ContainerID: containerID, HostID: hostID, Size: size})
	}
}

func matches(id int, configuredIDs []idtools.IDMap, currentIDs []idtools.IDMap) bool {
	// The first mapping is the host user, handle it separately.
	if currentIDs[0].HostID != id || currentIDs[0].Size != 1 {
		return false
	}

	currentIDs = currentIDs[1:]
	if len(currentIDs) != len(configuredIDs) {
		return false
	}

	// It is fine to iterate sequentially as both slices are sorted.
	for i := range currentIDs {
		if currentIDs[i].HostID != configuredIDs[i].HostID {
			return false
		}
		if currentIDs[i].Size != configuredIDs[i].Size {
			return false
		}
	}

	return true
}

// ConfigurationMatches checks whether the additional uids/gids configured for the user
// match the current user namespace.
func ConfigurationMatches() (bool, error) {
	if !IsRootless() || os.Geteuid() != 0 {
		return true, nil
	}

	uids, gids, err := GetConfiguredMappings()
	if err != nil {
		return false, err
	}

	currentUIDs, err := ReadMappingsProc("/proc/self/uid_map")
	if err != nil {
		return false, err
	}

	if !matches(GetRootlessUID(), uids, currentUIDs) {
		return false, err
	}

	currentGIDs, err := ReadMappingsProc("/proc/self/gid_map")
	if err != nil {
		return false, err
	}

	return matches(GetRootlessGID(), gids, currentGIDs), nil
}
