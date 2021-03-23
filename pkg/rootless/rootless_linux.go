// +build linux,cgo

package rootless

import (
	"bufio"
	"bytes"
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

	"github.com/containers/podman/v3/pkg/errorhandling"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/unshare"
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
extern int reexec_userns_join(int pid, char *pause_pid_file_path);
extern int is_fd_inherited(int fd);
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
		if !isRootless {
			hasCapSysAdmin, err := unshare.HasCapSysAdmin()
			if err != nil {
				logrus.Warnf("failed to read CAP_SYS_ADMIN presence for the current process")
			}
			if err == nil && !hasCapSysAdmin {
				isRootless = true
			}
		}
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

func tryMappingTool(uid bool, pid int, hostID int, mappings []idtools.IDMap) error {
	var tool = "newuidmap"
	if !uid {
		tool = "newgidmap"
	}
	path, err := exec.LookPath(tool)
	if err != nil {
		return errors.Wrapf(err, "command required for rootless mode with multiple IDs")
	}

	appendTriplet := func(l []string, a, b, c int) []string {
		return append(l, strconv.Itoa(a), strconv.Itoa(b), strconv.Itoa(c))
	}

	args := []string{path, fmt.Sprintf("%d", pid)}
	args = appendTriplet(args, 0, hostID, 1)
	for _, i := range mappings {
		if hostID >= i.HostID && hostID < i.HostID+i.Size {
			what := "UID"
			where := "/etc/subuid"
			if !uid {
				what = "GID"
				where = "/etc/subgid"
			}
			return errors.Errorf("invalid configuration: the specified mapping %d:%d in %q includes the user %s", i.HostID, i.Size, where, what)
		}
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

// joinUserAndMountNS re-exec podman in a new userNS and join the user and mount
// namespace of the specified PID without looking up its parent.  Useful to join directly
// the conmon process.
func joinUserAndMountNS(pid uint, pausePid string) (bool, int, error) {
	hasCapSysAdmin, err := unshare.HasCapSysAdmin()
	if err != nil {
		return false, 0, err
	}
	if hasCapSysAdmin || os.Getenv("_CONTAINERS_USERNS_CONFIGURED") != "" {
		return false, 0, nil
	}

	cPausePid := C.CString(pausePid)
	defer C.free(unsafe.Pointer(cPausePid))

	pidC := C.reexec_userns_join(C.int(pid), cPausePid)
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
		logLevel := logrus.ErrorLevel
		if os.Geteuid() == 0 && GetRootlessUID() == 0 {
			logLevel = logrus.DebugLevel
		}
		logrus.StandardLogger().Logf(logLevel, "cannot find UID/GID for user %s: %v - check rootless mode in man pages.", username, err)
	} else {
		uids = mappings.UIDs()
		gids = mappings.GIDs()
	}
	return uids, gids, nil
}

func copyMappings(from, to string) error {
	content, err := ioutil.ReadFile(from)
	if err != nil {
		return err
	}
	// Both runc and crun check whether the current process is in a user namespace
	// by looking up 4294967295 in /proc/self/uid_map.  If the mappings would be
	// copied as they are, the check in the OCI runtimes would fail.  So just split
	// it in two different ranges.
	if bytes.Contains(content, []byte("4294967295")) {
		content = []byte("0 0 1\n1 1 4294967294\n")
	}
	return ioutil.WriteFile(to, content, 0600)
}

func becomeRootInUserNS(pausePid, fileToRead string, fileOutput *os.File) (_ bool, _ int, retErr error) {
	hasCapSysAdmin, err := unshare.HasCapSysAdmin()
	if err != nil {
		return false, 0, err
	}

	if hasCapSysAdmin || os.Getenv("_CONTAINERS_USERNS_CONFIGURED") != "" {
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

	var pid int

	defer errorhandling.CloseQuiet(r)
	defer errorhandling.CloseQuiet(w)
	defer func() {
		toWrite := []byte("0")
		if retErr != nil {
			toWrite = []byte("1")
		}
		if _, err := w.Write(toWrite); err != nil {
			logrus.Errorf("failed to write byte 0: %q", err)
		}
		if retErr != nil && pid > 0 {
			if err := unix.Kill(pid, unix.SIGKILL); err != nil {
				logrus.Errorf("failed to kill %d", pid)
			}
			C.reexec_in_user_namespace_wait(C.int(pid), 0)
		}
	}()

	pidC := C.reexec_in_user_namespace(C.int(r.Fd()), cPausePid, cFileToRead, fileOutputFD)
	pid = int(pidC)
	if pid < 0 {
		return false, -1, errors.Errorf("cannot re-exec process")
	}

	uids, gids, err := GetConfiguredMappings()
	if err != nil {
		return false, -1, err
	}

	uidMap := fmt.Sprintf("/proc/%d/uid_map", pid)
	gidMap := fmt.Sprintf("/proc/%d/gid_map", pid)

	uidsMapped := false

	if err := copyMappings("/proc/self/uid_map", uidMap); err == nil {
		uidsMapped = true
	}

	if uids != nil && !uidsMapped {
		err := tryMappingTool(true, pid, os.Geteuid(), uids)
		// If some mappings were specified, do not ignore the error
		if err != nil && len(uids) > 0 {
			return false, -1, err
		}
		uidsMapped = err == nil
	}
	if !uidsMapped {
		logrus.Warnf("using rootless single mapping into the namespace. This might break some images. Check /etc/subuid and /etc/subgid for adding sub*ids")
		setgroups := fmt.Sprintf("/proc/%d/setgroups", pid)
		err = ioutil.WriteFile(setgroups, []byte("deny\n"), 0666)
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot write setgroups file")
		}
		logrus.Debugf("write setgroups file exited with 0")

		err = ioutil.WriteFile(uidMap, []byte(fmt.Sprintf("%d %d 1\n", 0, os.Geteuid())), 0666)
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot write uid_map")
		}
		logrus.Debugf("write uid_map exited with 0")
	}

	gidsMapped := false
	if err := copyMappings("/proc/self/gid_map", gidMap); err == nil {
		gidsMapped = true
	}
	if gids != nil && !gidsMapped {
		err := tryMappingTool(false, pid, os.Getegid(), gids)
		// If some mappings were specified, do not ignore the error
		if err != nil && len(gids) > 0 {
			return false, -1, err
		}
		gidsMapped = err == nil
	}
	if !gidsMapped {
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
		ret := C.reexec_in_user_namespace_wait(pidC, 0)
		if ret < 0 {
			return false, -1, errors.New("error waiting for the re-exec process")
		}

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
	foundProcess := false

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
			r, w, err := os.Pipe()
			if err != nil {
				lastErr = err
				continue
			}

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
			if err == nil && unix.Kill(pausePid, 0) == nil {
				foundProcess = true
				lastErr = nil
				break
			}
		}
	}
	if !foundProcess && pausePidPath != "" {
		return BecomeRootInUserNS(pausePidPath)
	}
	if lastErr != nil {
		return false, 0, lastErr
	}

	return joinUserAndMountNS(uint(pausePid), pausePidPath)
}

// ReadMappingsProc parses and returns the ID mappings at the specified path.
func ReadMappingsProc(path string) ([]idtools.IDMap, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
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

// IsFdInherited checks whether the fd is opened and valid to use
func IsFdInherited(fd int) bool {
	return int(C.is_fd_inherited(C.int(fd))) > 0
}
