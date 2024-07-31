//go:build linux

// Copyright (C) 2024 SUSE LLC. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package securejoin

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"

	"golang.org/x/sys/unix"
)

func fstat(f *os.File) (unix.Stat_t, error) {
	var stat unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &stat); err != nil {
		return stat, &os.PathError{Op: "fstat", Path: f.Name(), Err: err}
	}
	return stat, nil
}

func fstatfs(f *os.File) (unix.Statfs_t, error) {
	var statfs unix.Statfs_t
	if err := unix.Fstatfs(int(f.Fd()), &statfs); err != nil {
		return statfs, &os.PathError{Op: "fstatfs", Path: f.Name(), Err: err}
	}
	return statfs, nil
}

// The kernel guarantees that the root inode of a procfs mount has an
// f_type of PROC_SUPER_MAGIC and st_ino of PROC_ROOT_INO.
const (
	procSuperMagic = 0x9fa0 // PROC_SUPER_MAGIC
	procRootIno    = 1      // PROC_ROOT_INO
)

func verifyProcRoot(procRoot *os.File) error {
	if statfs, err := fstatfs(procRoot); err != nil {
		return err
	} else if statfs.Type != procSuperMagic {
		return fmt.Errorf("%w: incorrect procfs root filesystem type 0x%x", errUnsafeProcfs, statfs.Type)
	}
	if stat, err := fstat(procRoot); err != nil {
		return err
	} else if stat.Ino != procRootIno {
		return fmt.Errorf("%w: incorrect procfs root inode number %d", errUnsafeProcfs, stat.Ino)
	}
	return nil
}

var (
	hasNewMountApiBool bool
	hasNewMountApiOnce sync.Once
)

func hasNewMountApi() bool {
	hasNewMountApiOnce.Do(func() {
		// All of the pieces of the new mount API we use (fsopen, fsconfig,
		// fsmount, open_tree) were added together in Linux 5.1[1,2], so we can
		// just check for one of the syscalls and the others should also be
		// available.
		//
		// Just try to use open_tree(2) to open a file without OPEN_TREE_CLONE.
		// This is equivalent to openat(2), but tells us if open_tree is
		// available (and thus all of the other basic new mount API syscalls).
		// open_tree(2) is most light-weight syscall to test here.
		//
		// [1]: merge commit 400913252d09
		// [2]: <https://lore.kernel.org/lkml/153754740781.17872.7869536526927736855.stgit@warthog.procyon.org.uk/>
		fd, err := unix.OpenTree(-int(unix.EBADF), "/", unix.OPEN_TREE_CLOEXEC)
		if err == nil {
			hasNewMountApiBool = true
			_ = unix.Close(fd)
		}
	})
	return hasNewMountApiBool
}

func fsopen(fsName string, flags int) (*os.File, error) {
	// Make sure we always set O_CLOEXEC.
	flags |= unix.FSOPEN_CLOEXEC
	fd, err := unix.Fsopen(fsName, flags)
	if err != nil {
		return nil, os.NewSyscallError("fsopen "+fsName, err)
	}
	return os.NewFile(uintptr(fd), "fscontext:"+fsName), nil
}

func fsmount(ctx *os.File, flags, mountAttrs int) (*os.File, error) {
	// Make sure we always set O_CLOEXEC.
	flags |= unix.FSMOUNT_CLOEXEC
	fd, err := unix.Fsmount(int(ctx.Fd()), flags, mountAttrs)
	if err != nil {
		return nil, os.NewSyscallError("fsmount "+ctx.Name(), err)
	}
	return os.NewFile(uintptr(fd), "fsmount:"+ctx.Name()), nil
}

func newPrivateProcMount() (*os.File, error) {
	procfsCtx, err := fsopen("proc", unix.FSOPEN_CLOEXEC)
	if err != nil {
		return nil, err
	}
	defer procfsCtx.Close()

	// Try to configure hidepid=ptraceable,subset=pid if possible, but ignore errors.
	_ = unix.FsconfigSetString(int(procfsCtx.Fd()), "hidepid", "ptraceable")
	_ = unix.FsconfigSetString(int(procfsCtx.Fd()), "subset", "pid")

	// Get an actual handle.
	if err := unix.FsconfigCreate(int(procfsCtx.Fd())); err != nil {
		return nil, os.NewSyscallError("fsconfig create procfs", err)
	}
	return fsmount(procfsCtx, unix.FSMOUNT_CLOEXEC, unix.MS_RDONLY|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_NOSUID)
}

func openTree(dir *os.File, path string, flags uint) (*os.File, error) {
	dirFd := -int(unix.EBADF)
	dirName := "."
	if dir != nil {
		dirFd = int(dir.Fd())
		dirName = dir.Name()
	}
	// Make sure we always set O_CLOEXEC.
	flags |= unix.OPEN_TREE_CLOEXEC
	fd, err := unix.OpenTree(dirFd, path, flags)
	if err != nil {
		return nil, &os.PathError{Op: "open_tree", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), dirName+"/"+path), nil
}

func clonePrivateProcMount() (_ *os.File, Err error) {
	// Try to make a clone without using AT_RECURSIVE if we can. If this works,
	// we can be sure there are no over-mounts and so if the root is valid then
	// we're golden. Otherwise, we have to deal with over-mounts.
	procfsHandle, err := openTree(nil, "/proc", unix.OPEN_TREE_CLONE)
	if err != nil || testingForcePrivateProcRootOpenTreeAtRecursive(procfsHandle) {
		procfsHandle, err = openTree(nil, "/proc", unix.OPEN_TREE_CLONE|unix.AT_RECURSIVE)
	}
	if err != nil {
		return nil, fmt.Errorf("creating a detached procfs clone: %w", err)
	}
	defer func() {
		if Err != nil {
			_ = procfsHandle.Close()
		}
	}()
	if err := verifyProcRoot(procfsHandle); err != nil {
		return nil, err
	}
	return procfsHandle, nil
}

func privateProcRoot() (*os.File, error) {
	if !hasNewMountApi() || testingForceGetProcRootUnsafe() {
		return nil, fmt.Errorf("new mount api: %w", unix.ENOTSUP)
	}
	// Try to create a new procfs mount from scratch if we can. This ensures we
	// can get a procfs mount even if /proc is fake (for whatever reason).
	procRoot, err := newPrivateProcMount()
	if err != nil || testingForcePrivateProcRootOpenTree(procRoot) {
		// Try to clone /proc then...
		procRoot, err = clonePrivateProcMount()
	}
	return procRoot, err
}

var (
	procRootHandle *os.File
	procRootError  error
	procRootOnce   sync.Once

	errUnsafeProcfs = errors.New("unsafe procfs detected")
)

func unsafeHostProcRoot() (_ *os.File, Err error) {
	procRoot, err := os.OpenFile("/proc", unix.O_PATH|unix.O_NOFOLLOW|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer func() {
		if Err != nil {
			_ = procRoot.Close()
		}
	}()
	if err := verifyProcRoot(procRoot); err != nil {
		return nil, err
	}
	return procRoot, nil
}

func doGetProcRoot() (*os.File, error) {
	procRoot, err := privateProcRoot()
	if err != nil {
		// Fall back to using a /proc handle if making a private mount failed.
		// If we have openat2, at least we can avoid some kinds of over-mount
		// attacks, but without openat2 there's not much we can do.
		procRoot, err = unsafeHostProcRoot()
	}
	return procRoot, err
}

func getProcRoot() (*os.File, error) {
	procRootOnce.Do(func() {
		procRootHandle, procRootError = doGetProcRoot()
	})
	return procRootHandle, procRootError
}

var (
	haveProcThreadSelf     bool
	haveProcThreadSelfOnce sync.Once
)

type procThreadSelfCloser func()

// procThreadSelf returns a handle to /proc/thread-self/<subpath> (or an
// equivalent handle on older kernels where /proc/thread-self doesn't exist).
// Once finished with the handle, you must call the returned closer function
// (runtime.UnlockOSThread). You must not pass the returned *os.File to other
// Go threads or use the handle after calling the closer.
//
// This is similar to ProcThreadSelf from runc, but with extra hardening
// applied and using *os.File.
func procThreadSelf(procRoot *os.File, subpath string) (_ *os.File, _ procThreadSelfCloser, Err error) {
	haveProcThreadSelfOnce.Do(func() {
		// If the kernel doesn't support thread-self, it doesn't matter which
		// /proc handle we use.
		_, err := fstatatFile(procRoot, "thread-self", unix.AT_SYMLINK_NOFOLLOW)
		haveProcThreadSelf = (err == nil)
	})

	// We need to lock our thread until the caller is done with the handle
	// because between getting the handle and using it we could get interrupted
	// by the Go runtime and hit the case where the underlying thread is
	// swapped out and the original thread is killed, resulting in
	// pull-your-hair-out-hard-to-debug issues in the caller.
	runtime.LockOSThread()
	defer func() {
		if Err != nil {
			runtime.UnlockOSThread()
		}
	}()

	// Figure out what prefix we want to use.
	threadSelf := "thread-self/"
	if !haveProcThreadSelf || testingForceProcSelfTask() {
		/// Pre-3.17 kernels don't have /proc/thread-self, so do it manually.
		threadSelf = "self/task/" + strconv.Itoa(unix.Gettid()) + "/"
		if _, err := fstatatFile(procRoot, threadSelf, unix.AT_SYMLINK_NOFOLLOW); err != nil || testingForceProcSelf() {
			// In this case, we running in a pid namespace that doesn't match
			// the /proc mount we have. This can happen inside runc.
			//
			// Unfortunately, there is no nice way to get the correct TID to
			// use here because of the age of the kernel, so we have to just
			// use /proc/self and hope that it works.
			threadSelf = "self/"
		}
	}

	// Grab the handle.
	var (
		handle *os.File
		err    error
	)
	if hasOpenat2() {
		// We prefer being able to use RESOLVE_NO_XDEV if we can, to be
		// absolutely sure we are operating on a clean /proc handle that
		// doesn't have any cheeky overmounts that could trick us (including
		// symlink mounts on top of /proc/thread-self). RESOLVE_BENEATH isn't
		// stricly needed, but just use it since we have it.
		//
		// NOTE: /proc/self is technically a magic-link (the contents of the
		//       symlink are generated dynamically), but it doesn't use
		//       nd_jump_link() so RESOLVE_NO_MAGICLINKS allows it.
		//
		// NOTE: We MUST NOT use RESOLVE_IN_ROOT here, as openat2File uses
		//       procSelfFdReadlink to clean up the returned f.Name() if we use
		//       RESOLVE_IN_ROOT (which would lead to an infinite recursion).
		handle, err = openat2File(procRoot, threadSelf+subpath, &unix.OpenHow{
			Flags:   unix.O_PATH | unix.O_NOFOLLOW | unix.O_CLOEXEC,
			Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_XDEV | unix.RESOLVE_NO_MAGICLINKS,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %w", errUnsafeProcfs, err)
		}
	} else {
		handle, err = openatFile(procRoot, threadSelf+subpath, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: %w", errUnsafeProcfs, err)
		}
		defer func() {
			if Err != nil {
				_ = handle.Close()
			}
		}()
		// We can't detect bind-mounts of different parts of procfs on top of
		// /proc (a-la RESOLVE_NO_XDEV), but we can at least be sure that we
		// aren't on the wrong filesystem here.
		if statfs, err := fstatfs(handle); err != nil {
			return nil, nil, err
		} else if statfs.Type != procSuperMagic {
			return nil, nil, fmt.Errorf("%w: incorrect /proc/self/fd filesystem type 0x%x", errUnsafeProcfs, statfs.Type)
		}
	}
	return handle, runtime.UnlockOSThread, nil
}

var (
	hasStatxMountIdBool bool
	hasStatxMountIdOnce sync.Once
)

func hasStatxMountId() bool {
	hasStatxMountIdOnce.Do(func() {
		var (
			stx unix.Statx_t
			// We don't care which mount ID we get. The kernel will give us the
			// unique one if it is supported.
			wantStxMask uint32 = unix.STATX_MNT_ID_UNIQUE | unix.STATX_MNT_ID
		)
		err := unix.Statx(-int(unix.EBADF), "/", 0, int(wantStxMask), &stx)
		hasStatxMountIdBool = (err == nil && (stx.Mask&wantStxMask != 0))
	})
	return hasStatxMountIdBool
}

func getMountId(dir *os.File, path string) (uint64, error) {
	// If we don't have statx(STATX_MNT_ID*) support, we can't do anything.
	if !hasStatxMountId() {
		return 0, nil
	}

	var (
		stx unix.Statx_t
		// We don't care which mount ID we get. The kernel will give us the
		// unique one if it is supported.
		wantStxMask uint32 = unix.STATX_MNT_ID_UNIQUE | unix.STATX_MNT_ID
	)

	err := unix.Statx(int(dir.Fd()), path, unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW, int(wantStxMask), &stx)
	if stx.Mask&wantStxMask == 0 {
		// It's not a kernel limitation, for some reason we couldn't get a
		// mount ID. Assume it's some kind of attack.
		err = fmt.Errorf("%w: could not get mount id", errUnsafeProcfs)
	}
	if err != nil {
		return 0, &os.PathError{Op: "statx(STATX_MNT_ID_...)", Path: dir.Name() + "/" + path, Err: err}
	}
	return stx.Mnt_id, nil
}

func checkSymlinkOvermount(procRoot *os.File, dir *os.File, path string) error {
	// Get the mntId of our procfs handle.
	expectedMountId, err := getMountId(procRoot, "")
	if err != nil {
		return err
	}
	// Get the mntId of the target magic-link.
	gotMountId, err := getMountId(dir, path)
	if err != nil {
		return err
	}
	// As long as the directory mount is alive, even with wrapping mount IDs,
	// we would expect to see a different mount ID here. (Of course, if we're
	// using unsafeHostProcRoot() then an attaker could change this after we
	// did this check.)
	if expectedMountId != gotMountId {
		return fmt.Errorf("%w: symlink %s/%s has an overmount obscuring the real link (mount ids do not match %d != %d)", errUnsafeProcfs, dir.Name(), path, expectedMountId, gotMountId)
	}
	return nil
}

func doRawProcSelfFdReadlink(procRoot *os.File, fd int) (string, error) {
	fdPath := fmt.Sprintf("fd/%d", fd)
	procFdLink, closer, err := procThreadSelf(procRoot, fdPath)
	if err != nil {
		return "", fmt.Errorf("get safe /proc/thread-self/%s handle: %w", fdPath, err)
	}
	defer procFdLink.Close()
	defer closer()

	// Try to detect if there is a mount on top of the magic-link. Since we use the handle directly
	// provide to the closure. If the closure uses the handle directly, this
	// should be safe in general (a mount on top of the path afterwards would
	// not affect the handle itself) and will definitely be safe if we are
	// using privateProcRoot() (at least since Linux 5.12[1], when anonymous
	// mount namespaces were completely isolated from external mounts including
	// mount propagation events).
	//
	// [1]: Linux commit ee2e3f50629f ("mount: fix mounting of detached mounts
	// onto targets that reside on shared mounts").
	if err := checkSymlinkOvermount(procRoot, procFdLink, ""); err != nil {
		return "", fmt.Errorf("check safety of /proc/thread-self/fd/%d magiclink: %w", fd, err)
	}

	// readlinkat implies AT_EMPTY_PATH since Linux 2.6.39. See Linux commit
	// 65cfc6722361 ("readlinkat(), fchownat() and fstatat() with empty
	// relative pathnames").
	return readlinkatFile(procFdLink, "")
}

func rawProcSelfFdReadlink(fd int) (string, error) {
	procRoot, err := getProcRoot()
	if err != nil {
		return "", err
	}
	return doRawProcSelfFdReadlink(procRoot, fd)
}

func procSelfFdReadlink(f *os.File) (string, error) {
	return rawProcSelfFdReadlink(int(f.Fd()))
}

var (
	errPossibleBreakout = errors.New("possible breakout detected")
	errInvalidDirectory = errors.New("wandered into deleted directory")
	errDeletedInode     = errors.New("cannot verify path of deleted inode")
)

func isDeadInode(file *os.File) error {
	// If the nlink of a file drops to 0, there is an attacker deleting
	// directories during our walk, which could result in weird /proc values.
	// It's better to error out in this case.
	stat, err := fstat(file)
	if err != nil {
		return fmt.Errorf("check for dead inode: %w", err)
	}
	if stat.Nlink == 0 {
		err := errDeletedInode
		if stat.Mode&unix.S_IFMT == unix.S_IFDIR {
			err = errInvalidDirectory
		}
		return fmt.Errorf("%w %q", err, file.Name())
	}
	return nil
}

func getUmask() int {
	// umask is a per-thread property, but it is inherited by children, so we
	// need to lock our OS thread to make sure that no other goroutine runs in
	// this thread and no goroutines are spawned from this thread until we
	// revert to the old umask.
	//
	// We could parse /proc/self/status to avoid this get-set problem, but
	// /proc/thread-self requires LockOSThread anyway, so there's no real
	// benefit over just using umask(2).
	runtime.LockOSThread()
	umask := unix.Umask(0)
	unix.Umask(umask)
	runtime.UnlockOSThread()
	return umask
}

func checkProcSelfFdPath(path string, file *os.File) error {
	if err := isDeadInode(file); err != nil {
		return err
	}
	actualPath, err := procSelfFdReadlink(file)
	if err != nil {
		return fmt.Errorf("get path of handle: %w", err)
	}
	if actualPath != path {
		return fmt.Errorf("%w: handle path %q doesn't match expected path %q", errPossibleBreakout, actualPath, path)
	}
	return nil
}
