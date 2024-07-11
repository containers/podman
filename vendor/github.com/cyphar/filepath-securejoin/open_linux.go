//go:build linux

// Copyright (C) 2024 SUSE LLC. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package securejoin

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// OpenatInRoot is equivalent to OpenInRoot, except that the root is provided
// using an *os.File handle, to ensure that the correct root directory is used.
func OpenatInRoot(root *os.File, unsafePath string) (*os.File, error) {
	handle, remainingPath, err := partialLookupInRoot(root, unsafePath)
	if err != nil {
		return nil, err
	}
	if remainingPath != "" {
		_ = handle.Close()
		return nil, &os.PathError{Op: "securejoin.OpenInRoot", Path: unsafePath, Err: unix.ENOENT}
	}
	return handle, nil
}

// OpenInRoot safely opens the provided unsafePath within the root.
// Effectively, OpenInRoot(root, unsafePath) is equivalent to
//
//	path, _ := securejoin.SecureJoin(root, unsafePath)
//	handle, err := os.OpenFile(path, unix.O_PATH|unix.O_CLOEXEC)
//
// But is much safer. The above implementation is unsafe because if an attacker
// can modify the filesystem tree between SecureJoin and OpenFile, it is
// possible for the returned file to be outside of the root.
//
// Note that the returned handle is an O_PATH handle, meaning that only a very
// limited set of operations will work on the handle. This is done to avoid
// accidentally opening an untrusted file that could cause issues (such as a
// disconnected TTY that could cause a DoS, or some other issue). In order to
// use the returned handle, you can "upgrade" it to a proper handle using
// Reopen.
func OpenInRoot(root, unsafePath string) (*os.File, error) {
	rootDir, err := os.OpenFile(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer rootDir.Close()
	return OpenatInRoot(rootDir, unsafePath)
}

// Reopen takes an *os.File handle and re-opens it through /proc/self/fd.
// Reopen(file, flags) is effectively equivalent to
//
//	fdPath := fmt.Sprintf("/proc/self/fd/%d", file.Fd())
//	os.OpenFile(fdPath, flags|unix.O_CLOEXEC)
//
// But with some extra hardenings to ensure that we are not tricked by a
// maliciously-configured /proc mount. While this attack scenario is not
// common, in container runtimes it is possible for higher-level runtimes to be
// tricked into configuring an unsafe /proc that can be used to attack file
// operations. See CVE-2019-19921 for more details.
func Reopen(handle *os.File, flags int) (*os.File, error) {
	procRoot, err := getProcRoot()
	if err != nil {
		return nil, err
	}

	flags |= unix.O_CLOEXEC
	fdPath := fmt.Sprintf("fd/%d", handle.Fd())
	return doProcSelfMagiclink(procRoot, fdPath, func(procDirHandle *os.File, base string) (*os.File, error) {
		// Rather than just wrapping openatFile, open-code it so we can copy
		// handle.Name().
		reopenFd, err := unix.Openat(int(procDirHandle.Fd()), base, flags, 0)
		if err != nil {
			return nil, fmt.Errorf("reopen fd %d: %w", handle.Fd(), err)
		}
		return os.NewFile(uintptr(reopenFd), handle.Name()), nil
	})
}
