//go:build !windows
// +build !windows

package copier

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var canChroot = os.Getuid() == 0

func chroot(root string) (bool, error) {
	if canChroot {
		if err := os.Chdir(root); err != nil {
			return false, fmt.Errorf("changing to intended-new-root directory %q: %w", root, err)
		}
		if err := unix.Chroot(root); err != nil {
			return false, fmt.Errorf("chrooting to directory %q: %w", root, err)
		}
		if err := os.Chdir(string(os.PathSeparator)); err != nil {
			return false, fmt.Errorf("changing to just-became-root directory %q: %w", root, err)
		}
		return true, nil
	}
	return false, nil
}

func chrMode(mode os.FileMode) uint32 {
	return uint32(unix.S_IFCHR | mode)
}

func blkMode(mode os.FileMode) uint32 {
	return uint32(unix.S_IFBLK | mode)
}

func mkdev(major, minor uint32) uint64 {
	return unix.Mkdev(major, minor)
}

func mkfifo(path string, mode uint32) error {
	return unix.Mkfifo(path, mode)
}

func chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

func chown(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}

func lchown(path string, uid, gid int) error {
	return os.Lchown(path, uid, gid)
}

func lutimes(isSymlink bool, path string, atime, mtime time.Time) error {
	if atime.IsZero() || mtime.IsZero() {
		now := time.Now()
		if atime.IsZero() {
			atime = now
		}
		if mtime.IsZero() {
			mtime = now
		}
	}
	return unix.Lutimes(path, []unix.Timeval{unix.NsecToTimeval(atime.UnixNano()), unix.NsecToTimeval(mtime.UnixNano())})
}

// sameDevice returns true unless we're sure that they're not on the same device
func sameDevice(a, b os.FileInfo) bool {
	aSys := a.Sys()
	bSys := b.Sys()
	if aSys == nil || bSys == nil {
		return true
	}
	au, aok := aSys.(*syscall.Stat_t)
	bu, bok := bSys.(*syscall.Stat_t)
	if !aok || !bok {
		return true
	}
	return au.Dev == bu.Dev
}

const (
	testModeMask           = int64(os.ModePerm)
	testIgnoreSymlinkDates = false
)
