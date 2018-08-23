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
	"syscall"

	"github.com/containers/storage/pkg/idtools"
	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
)

/*
extern int reexec_in_user_namespace(int ready);
extern int reexec_in_user_namespace_wait(int pid);
*/
import "C"

func runInUser() error {
	os.Setenv("_LIBPOD_USERNS_CONFIGURED", "done")
	return nil
}

// IsRootless tells us if we are running in rootless mode
func IsRootless() bool {
	return os.Getuid() != 0 || os.Getenv("_LIBPOD_USERNS_CONFIGURED") != ""
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
		return err
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
	return cmd.Run()
}

// BecomeRootInUserNS re-exec podman in a new userNS.  It returns whether podman was re-executed
// into a new user namespace and the return code from the re-executed podman process.
// If podman was re-executed the caller needs to propagate the error code returned by the child
// process.
func BecomeRootInUserNS() (bool, int, error) {
	if os.Getuid() == 0 || os.Getenv("_LIBPOD_USERNS_CONFIGURED") != "" {
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

	var uids, gids []idtools.IDMap
	username := os.Getenv("USER")
	if username == "" {
		user, err := user.LookupId(fmt.Sprintf("%d", os.Geteuid()))
		if err != nil && os.Getenv("PODMAN_ALLOW_SINGLE_ID_MAPPING_IN_USERNS") == "" {
			return false, 0, errors.Wrapf(err, "could not find user by UID nor USER env was set")
		}
		if err == nil {
			username = user.Username
		}
	}
	mappings, err := idtools.NewIDMappings(username, username)
	if err != nil && os.Getenv("PODMAN_ALLOW_SINGLE_ID_MAPPING_IN_USERNS") == "" {
		return false, -1, err
	}
	if err == nil {
		uids = mappings.UIDs()
		gids = mappings.GIDs()
	}

	uidsMapped := false
	if mappings != nil && uids != nil {
		uidsMapped = tryMappingTool("newuidmap", pid, os.Getuid(), uids) == nil
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
		gidsMapped = tryMappingTool("newgidmap", pid, os.Getgid(), gids) == nil
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
		return false, -1, errors.Wrapf(err, "error waiting for the re-exec process")
	}

	return true, int(ret), nil
}
