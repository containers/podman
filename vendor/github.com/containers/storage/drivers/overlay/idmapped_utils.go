//go:build linux
// +build linux

package overlay

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/containers/storage/pkg/idtools"
	"golang.org/x/sys/unix"
)

type attr struct {
	attrSet     uint64
	attrClr     uint64
	propagation uint64
	userNs      uint64
}

// openTree is a wrapper for the open_tree syscall
func openTree(path string, flags int) (fd int, err error) {
	var _p0 *byte

	if _p0, err = syscall.BytePtrFromString(path); err != nil {
		return 0, err
	}

	r, _, e1 := syscall.Syscall6(uintptr(unix.SYS_OPEN_TREE), uintptr(0), uintptr(unsafe.Pointer(_p0)),
		uintptr(flags), 0, 0, 0)
	if e1 != 0 {
		err = e1
	}
	return int(r), nil
}

// moveMount is a wrapper for the the move_mount syscall.
func moveMount(fdTree int, target string) (err error) {
	var _p0, _p1 *byte

	empty := ""

	if _p0, err = syscall.BytePtrFromString(target); err != nil {
		return err
	}
	if _p1, err = syscall.BytePtrFromString(empty); err != nil {
		return err
	}

	flags := unix.MOVE_MOUNT_F_EMPTY_PATH

	_, _, e1 := syscall.Syscall6(uintptr(unix.SYS_MOVE_MOUNT),
		uintptr(fdTree), uintptr(unsafe.Pointer(_p1)),
		0, uintptr(unsafe.Pointer(_p0)), uintptr(flags), 0)
	if e1 != 0 {
		err = e1
	}
	return
}

// mountSetAttr is a wrapper for the mount_setattr syscall
func mountSetAttr(dfd int, path string, flags uint, attr *attr, size uint) (err error) {
	var _p0 *byte

	if _p0, err = syscall.BytePtrFromString(path); err != nil {
		return err
	}

	_, _, e1 := syscall.Syscall6(uintptr(unix.SYS_MOUNT_SETATTR), uintptr(dfd), uintptr(unsafe.Pointer(_p0)),
		uintptr(flags), uintptr(unsafe.Pointer(attr)), uintptr(size), 0)
	if e1 != 0 {
		err = e1
	}
	return
}

// createIDMappedMount creates a IDMapped bind mount from SOURCE to TARGET using the user namespace
// for the PID process.
func createIDMappedMount(source, target string, pid int) error {
	path := fmt.Sprintf("/proc/%d/ns/user", pid)
	userNsFile, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("unable to get user ns file descriptor for %q: %w", path, err)
	}

	var attr attr
	attr.attrSet = unix.MOUNT_ATTR_IDMAP
	attr.attrClr = 0
	attr.propagation = 0
	attr.userNs = uint64(userNsFile.Fd())

	defer userNsFile.Close()

	targetDirFd, err := openTree(source, unix.OPEN_TREE_CLONE)
	if err != nil {
		return err
	}
	defer unix.Close(targetDirFd)

	if err := mountSetAttr(targetDirFd, "", unix.AT_EMPTY_PATH|unix.AT_RECURSIVE,
		&attr, uint(unsafe.Sizeof(attr))); err != nil {
		return err
	}
	if err := os.Mkdir(target, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	return moveMount(targetDirFd, target)
}

// createUsernsProcess forks the current process and creates a user namespace using the specified
// mappings.  It returns the pid of the new process.
func createUsernsProcess(uidMaps []idtools.IDMap, gidMaps []idtools.IDMap) (int, func(), error) {
	var pid uintptr
	var err syscall.Errno

	if runtime.GOARCH == "s390x" {
		pid, _, err = syscall.Syscall6(uintptr(unix.SYS_CLONE), 0, unix.CLONE_NEWUSER|uintptr(unix.SIGCHLD), 0, 0, 0, 0)
	} else {
		pid, _, err = syscall.Syscall6(uintptr(unix.SYS_CLONE), unix.CLONE_NEWUSER|uintptr(unix.SIGCHLD), 0, 0, 0, 0, 0)
	}
	if err != 0 {
		return -1, nil, err
	}
	if pid == 0 {
		_ = unix.Prctl(unix.PR_SET_PDEATHSIG, uintptr(unix.SIGKILL), 0, 0, 0)
		// just wait for the SIGKILL
		for {
			syscall.Pause()
		}
	}
	cleanupFunc := func() {
		unix.Kill(int(pid), unix.SIGKILL)
		_, _ = unix.Wait4(int(pid), nil, 0, nil)
	}
	writeMappings := func(fname string, idmap []idtools.IDMap) error {
		mappings := ""
		for _, m := range idmap {
			mappings = mappings + fmt.Sprintf("%d %d %d\n", m.ContainerID, m.HostID, m.Size)
		}
		return os.WriteFile(fmt.Sprintf("/proc/%d/%s", pid, fname), []byte(mappings), 0600)
	}
	if err := writeMappings("uid_map", uidMaps); err != nil {
		cleanupFunc()
		return -1, nil, err
	}
	if err := writeMappings("gid_map", gidMaps); err != nil {
		cleanupFunc()
		return -1, nil, err
	}

	return int(pid), cleanupFunc, nil
}
