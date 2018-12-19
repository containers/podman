// +build linux

package rootless

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	gosignal "os/signal"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/containers/storage/pkg/idtools"
	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

/*
extern int reexec_in_user_namespace(int ready);
extern int reexec_in_user_namespace_wait(int pid);
extern int reexec_userns_join(int userns, int mountns);
*/
import "C"

func runInUser() error {
	os.Setenv("_LIBPOD_USERNS_CONFIGURED", "done")
	return nil
}

var (
	isRootlessOnce sync.Once
	isRootless     bool
)

// IsRootless tells us if we are running in rootless mode
func IsRootless() bool {
	isRootlessOnce.Do(func() {
		isRootless = os.Geteuid() != 0 || os.Getenv("_LIBPOD_USERNS_CONFIGURED") != ""
	})
	return isRootless
}

var (
	skipStorageSetup = false
)

// SetSkipStorageSetup tells the runtime to not setup containers/storage
func SetSkipStorageSetup(v bool) {
	skipStorageSetup = v
}

// SkipStorageSetup tells if we should skip the containers/storage setup
func SkipStorageSetup() bool {
	return skipStorageSetup
}

// GetRootlessUID returns the UID of the user in the parent userNS
func GetRootlessUID() int {
	uidEnv := os.Getenv("_LIBPOD_ROOTLESS_UID")
	if uidEnv != "" {
		u, _ := strconv.Atoi(uidEnv)
		return u
	}
	return os.Getuid()
}

func tryMappingTool(tool string, pid int, hostID int, mappings []idtools.IDMap) error {
	path, err := exec.LookPath(tool)
	if err != nil {
		return errors.Wrapf(err, "cannot find %s", tool)
	}

	appendTriplet := func(l []string, a, b, c int) []string {
		return append(l, fmt.Sprintf("%d", a), fmt.Sprintf("%d", b), fmt.Sprintf("%d", c))
	}

	args := []string{path, fmt.Sprintf("%d", pid)}
	args = appendTriplet(args, 0, hostID, 1)
	if mappings != nil {
		for _, i := range mappings {
			args = appendTriplet(args, i.ContainerID+1, i.HostID, i.Size)
		}
	}
	cmd := exec.Cmd{
		Path: path,
		Args: args,
	}

	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "cannot setup namespace using %s", tool)
	}
	return nil
}

// JoinNS re-exec podman in a new userNS and join the user namespace of the specified
// PID.
func JoinNS(pid uint) (bool, int, error) {
	if os.Geteuid() == 0 || os.Getenv("_LIBPOD_USERNS_CONFIGURED") != "" {
		return false, -1, nil
	}

	userNS, err := getUserNSForPid(pid)
	if err != nil {
		return false, -1, err
	}
	defer userNS.Close()

	pidC := C.reexec_userns_join(C.int(userNS.Fd()), -1)
	if int(pidC) < 0 {
		return false, -1, errors.Errorf("cannot re-exec process")
	}

	ret := C.reexec_in_user_namespace_wait(pidC)
	if ret < 0 {
		return false, -1, errors.New("error waiting for the re-exec process")
	}

	return true, int(ret), nil
}

// JoinDirectUserAndMountNS re-exec podman in a new userNS and join the user and mount
// namespace of the specified PID without looking up its parent.  Useful to join directly
// the conmon process.
func JoinDirectUserAndMountNS(pid uint) (bool, int, error) {
	if os.Geteuid() == 0 || os.Getenv("_LIBPOD_USERNS_CONFIGURED") != "" {
		return false, -1, nil
	}

	userNS, err := os.Open(fmt.Sprintf("/proc/%d/ns/user", pid))
	if err != nil {
		return false, -1, err
	}
	defer userNS.Close()

	mountNS, err := os.Open(fmt.Sprintf("/proc/%d/ns/mnt", pid))
	if err != nil {
		return false, -1, err
	}
	defer userNS.Close()

	pidC := C.reexec_userns_join(C.int(userNS.Fd()), C.int(mountNS.Fd()))
	if int(pidC) < 0 {
		return false, -1, errors.Errorf("cannot re-exec process")
	}

	ret := C.reexec_in_user_namespace_wait(pidC)
	if ret < 0 {
		return false, -1, errors.New("error waiting for the re-exec process")
	}

	return true, int(ret), nil
}

// JoinNSPath re-exec podman in a new userNS and join the owner user namespace of the
// specified path.
func JoinNSPath(path string) (bool, int, error) {
	if os.Geteuid() == 0 || os.Getenv("_LIBPOD_USERNS_CONFIGURED") != "" {
		return false, -1, nil
	}

	userNS, err := getUserNSForPath(path)
	if err != nil {
		return false, -1, err
	}
	defer userNS.Close()

	pidC := C.reexec_userns_join(C.int(userNS.Fd()), -1)
	if int(pidC) < 0 {
		return false, -1, errors.Errorf("cannot re-exec process")
	}

	ret := C.reexec_in_user_namespace_wait(pidC)
	if ret < 0 {
		return false, -1, errors.New("error waiting for the re-exec process")
	}

	return true, int(ret), nil
}

const defaultMinimumMappings = 65536

func getMinimumIDs(p string) int {
	content, err := ioutil.ReadFile(p)
	if err != nil {
		logrus.Debugf("error reading data from %q, use a default value of %d", p, defaultMinimumMappings)
		return defaultMinimumMappings
	}
	ret, err := strconv.Atoi(strings.TrimSuffix(string(content), "\n"))
	if err != nil {
		logrus.Debugf("error reading data from %q, use a default value of %d", p, defaultMinimumMappings)
		return defaultMinimumMappings
	}
	return ret + 1
}

// BecomeRootInUserNS re-exec podman in a new userNS.  It returns whether podman was re-executed
// into a new user namespace and the return code from the re-executed podman process.
// If podman was re-executed the caller needs to propagate the error code returned by the child
// process.
func BecomeRootInUserNS() (bool, int, error) {
	if os.Geteuid() == 0 || os.Getenv("_LIBPOD_USERNS_CONFIGURED") != "" {
		if os.Getenv("_LIBPOD_USERNS_CONFIGURED") == "init" {
			return false, 0, runInUser()
		}
		return false, 0, nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	r, w, err := os.Pipe()
	if err != nil {
		return false, -1, err
	}
	defer r.Close()
	defer w.Close()

	pidC := C.reexec_in_user_namespace(C.int(r.Fd()))
	pid := int(pidC)
	if pid < 0 {
		return false, -1, errors.Errorf("cannot re-exec process")
	}

	allowSingleIDMapping := os.Getenv("PODMAN_ALLOW_SINGLE_ID_MAPPING_IN_USERNS") != ""

	var uids, gids []idtools.IDMap
	username := os.Getenv("USER")
	if username == "" {
		user, err := user.LookupId(fmt.Sprintf("%d", os.Getuid()))
		if err != nil && !allowSingleIDMapping {
			if os.IsNotExist(err) {
				return false, 0, errors.Wrapf(err, "/etc/subuid or /etc/subgid does not exist, see subuid/subgid man pages for information on these files")
			}
			return false, 0, errors.Wrapf(err, "could not find user by UID nor USER env was set")
		}
		if err == nil {
			username = user.Username
		}
	}
	mappings, err := idtools.NewIDMappings(username, username)
	if !allowSingleIDMapping {
		if err != nil {
			return false, -1, err
		}

		availableGIDs, availableUIDs := 0, 0
		for _, i := range mappings.UIDs() {
			availableUIDs += i.Size
		}

		minUIDs := getMinimumIDs("/proc/sys/kernel/overflowuid")
		if availableUIDs < minUIDs {
			return false, 0, fmt.Errorf("not enough UIDs available for the user, at least %d are needed", minUIDs)
		}

		for _, i := range mappings.GIDs() {
			availableGIDs += i.Size
		}
		minGIDs := getMinimumIDs("/proc/sys/kernel/overflowgid")
		if availableGIDs < minGIDs {
			return false, 0, fmt.Errorf("not enough GIDs available for the user, at least %d are needed", minGIDs)
		}
	}
	if err == nil {
		uids = mappings.UIDs()
		gids = mappings.GIDs()
	}

	uidsMapped := false
	if mappings != nil && uids != nil {
		err := tryMappingTool("newuidmap", pid, os.Getuid(), uids)
		if !allowSingleIDMapping && err != nil {
			return false, 0, err
		}
		uidsMapped = err == nil
	}
	if !uidsMapped {
		setgroups := fmt.Sprintf("/proc/%d/setgroups", pid)
		err = ioutil.WriteFile(setgroups, []byte("deny\n"), 0666)
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot write setgroups file")
		}

		uidMap := fmt.Sprintf("/proc/%d/uid_map", pid)
		err = ioutil.WriteFile(uidMap, []byte(fmt.Sprintf("%d %d 1\n", 0, os.Getuid())), 0666)
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot write uid_map")
		}
	}

	gidsMapped := false
	if mappings != nil && gids != nil {
		err := tryMappingTool("newgidmap", pid, os.Getgid(), gids)
		if !allowSingleIDMapping && err != nil {
			return false, 0, err
		}
		gidsMapped = err == nil
	}
	if !gidsMapped {
		gidMap := fmt.Sprintf("/proc/%d/gid_map", pid)
		err = ioutil.WriteFile(gidMap, []byte(fmt.Sprintf("%d %d 1\n", 0, os.Getgid())), 0666)
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot write gid_map")
		}
	}

	_, err = w.Write([]byte("1"))
	if err != nil {
		return false, -1, errors.Wrapf(err, "write to sync pipe")
	}

	c := make(chan os.Signal, 1)

	gosignal.Notify(c)
	defer gosignal.Reset()
	go func() {
		for s := range c {
			if s == signal.SIGCHLD || s == signal.SIGPIPE {
				continue
			}

			syscall.Kill(int(pidC), s.(syscall.Signal))
		}
	}()

	ret := C.reexec_in_user_namespace_wait(pidC)
	if ret < 0 {
		return false, -1, errors.New("error waiting for the re-exec process")
	}

	return true, int(ret), nil
}

func readUserNs(path string) (string, error) {
	b := make([]byte, 256)
	_, err := syscall.Readlink(path, b)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func readUserNsFd(fd uintptr) (string, error) {
	return readUserNs(fmt.Sprintf("/proc/self/fd/%d", fd))
}

func getOwner(fd uintptr) (uintptr, error) {
	const nsGetUserns = 0xb701
	ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(nsGetUserns), 0)
	if errno != 0 {
		return 0, errno
	}
	return (uintptr)(unsafe.Pointer(ret)), nil
}

func getParentUserNs(fd uintptr) (uintptr, error) {
	const nsGetParent = 0xb702
	ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(nsGetParent), 0)
	if errno != 0 {
		return 0, errno
	}
	return (uintptr)(unsafe.Pointer(ret)), nil
}

func getUserNSForPath(path string) (*os.File, error) {
	u, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open %s", path)
	}
	defer u.Close()
	fd, err := getOwner(u.Fd())
	if err != nil {
		return nil, err
	}

	return getUserNSFirstChild(fd)
}

func getUserNSForPid(pid uint) (*os.File, error) {
	path := fmt.Sprintf("/proc/%d/ns/user", pid)
	u, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open %s", path)
	}

	return getUserNSFirstChild(u.Fd())
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
			return nil, errors.Wrapf(err, "cannot get parent user namespace")
		}

		ns, err = readUserNsFd(nextFd)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot read user namespace")
		}

		if ns == currentNS {
			syscall.Close(int(nextFd))

			// Drop O_CLOEXEC for the fd.
			_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFD, 0)
			if errno != 0 {
				syscall.Close(int(fd))
				return nil, errno
			}

			return os.NewFile(fd, "userns child"), nil
		}
		syscall.Close(int(fd))
		fd = nextFd
	}
}
