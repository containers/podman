//go:build linux

// Copyright (C) 2024 SUSE LLC. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package securejoin

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sys/unix"
)

var (
	errInvalidMode    = errors.New("invalid permission mode")
	errPossibleAttack = errors.New("possible attack detected")
)

// MkdirAllHandle is equivalent to MkdirAll, except that it is safer to use in
// two respects:
//
//   - The caller provides the root directory as an *os.File (preferably O_PATH)
//     handle. This means that the caller can be sure which root directory is
//     being used. Note that this can be emulated by using /proc/self/fd/... as
//     the root path with MkdirAll.
//
//   - Once all of the directories have been created, an *os.File (O_PATH) handle
//     to the directory at unsafePath is returned to the caller. This is done in
//     an effectively-race-free way (an attacker would only be able to swap the
//     final directory component), which is not possible to emulate with
//     MkdirAll.
//
// In addition, the returned handle is obtained far more efficiently than doing
// a brand new lookup of unsafePath (such as with SecureJoin or openat2) after
// doing MkdirAll. If you intend to open the directory after creating it, you
// should use MkdirAllHandle.
func MkdirAllHandle(root *os.File, unsafePath string, mode int) (_ *os.File, Err error) {
	// Make sure there are no os.FileMode bits set.
	if mode&^0o7777 != 0 {
		return nil, fmt.Errorf("%w for mkdir 0o%.3o", errInvalidMode, mode)
	}

	// Try to open as much of the path as possible.
	currentDir, remainingPath, err := partialLookupInRoot(root, unsafePath)
	defer func() {
		if Err != nil {
			_ = currentDir.Close()
		}
	}()
	if err != nil && !errors.Is(err, unix.ENOENT) {
		return nil, fmt.Errorf("find existing subpath of %q: %w", unsafePath, err)
	}

	// If there is an attacker deleting directories as we walk into them,
	// detect this proactively. Note this is guaranteed to detect if the
	// attacker deleted any part of the tree up to currentDir.
	//
	// Once we walk into a dead directory, partialLookupInRoot would not be
	// able to walk further down the tree (directories must be empty before
	// they are deleted), and if the attacker has removed the entire tree we
	// can be sure that anything that was originally inside a dead directory
	// must also be deleted and thus is a dead directory in its own right.
	//
	// This is mostly a quality-of-life check, because mkdir will simply fail
	// later if the attacker deletes the tree after this check.
	if err := isDeadInode(currentDir); err != nil {
		return nil, fmt.Errorf("finding existing subpath of %q: %w", unsafePath, err)
	}

	// Re-open the path to match the O_DIRECTORY reopen loop later (so that we
	// always return a non-O_PATH handle). We also check that we actually got a
	// directory.
	if reopenDir, err := Reopen(currentDir, unix.O_DIRECTORY|unix.O_CLOEXEC); errors.Is(err, unix.ENOTDIR) {
		return nil, fmt.Errorf("cannot create subdirectories in %q: %w", currentDir.Name(), unix.ENOTDIR)
	} else if err != nil {
		return nil, fmt.Errorf("re-opening handle to %q: %w", currentDir.Name(), err)
	} else {
		_ = currentDir.Close()
		currentDir = reopenDir
	}

	remainingParts := strings.Split(remainingPath, string(filepath.Separator))
	if slices.Contains(remainingParts, "..") {
		// The path contained ".." components after the end of the "real"
		// components. We could try to safely resolve ".." here but that would
		// add a bunch of extra logic for something that it's not clear even
		// needs to be supported. So just return an error.
		//
		// If we do filepath.Clean(remainingPath) then we end up with the
		// problem that ".." can erase a trailing dangling symlink and produce
		// a path that doesn't quite match what the user asked for.
		return nil, fmt.Errorf("%w: yet-to-be-created path %q contains '..' components", unix.ENOENT, remainingPath)
	}

	// Make sure the mode doesn't have any type bits.
	mode &^= unix.S_IFMT
	// What properties do we expect any newly created directories to have?
	var (
		// While umask(2) is a per-thread property, and thus this value could
		// vary between threads, a functioning Go program would LockOSThread
		// threads with different umasks and so we don't need to LockOSThread
		// for this entire mkdirat loop (if we are in the locked thread with a
		// different umask, we are already locked and there's nothing for us to
		// do -- and if not then it doesn't matter which thread we run on and
		// there's nothing for us to do).
		expectedMode = uint32(unix.S_IFDIR | (mode &^ getUmask()))

		// We would want to get the fs[ug]id here, but we can't access those
		// from userspace. In practice, nobody uses setfs[ug]id() anymore, so
		// just use the effective [ug]id (which is equivalent to the fs[ug]id
		// for programs that don't use setfs[ug]id).
		expectedUid = uint32(unix.Geteuid())
		expectedGid = uint32(unix.Getegid())
	)

	// Create the remaining components.
	for _, part := range remainingParts {
		switch part {
		case "", ".":
			// Skip over no-op paths.
			continue
		}

		// NOTE: mkdir(2) will not follow trailing symlinks, so we can safely
		// create the finaly component without worrying about symlink-exchange
		// attacks.
		if err := unix.Mkdirat(int(currentDir.Fd()), part, uint32(mode)); err != nil {
			err = &os.PathError{Op: "mkdirat", Path: currentDir.Name() + "/" + part, Err: err}
			// Make the error a bit nicer if the directory is dead.
			if err2 := isDeadInode(currentDir); err2 != nil {
				err = fmt.Errorf("%w (%w)", err, err2)
			}
			return nil, err
		}

		// Get a handle to the next component. O_DIRECTORY means we don't need
		// to use O_PATH.
		var nextDir *os.File
		if hasOpenat2() {
			nextDir, err = openat2File(currentDir, part, &unix.OpenHow{
				Flags:   unix.O_NOFOLLOW | unix.O_DIRECTORY | unix.O_CLOEXEC,
				Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_XDEV,
			})
		} else {
			nextDir, err = openatFile(currentDir, part, unix.O_NOFOLLOW|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		}
		if err != nil {
			return nil, err
		}
		_ = currentDir.Close()
		currentDir = nextDir

		// Make sure that the directory matches what we expect. An attacker
		// could have swapped the directory between us making it and opening
		// it. There's no way for us to be sure that the directory is
		// _precisely_ the same as the directory we created, but if we are in
		// an empty directory with the same owner and mode as the one we
		// created then there is nothing the attacker could do with this new
		// directory that they couldn't do with the old one.
		if stat, err := fstat(currentDir); err != nil {
			return nil, fmt.Errorf("check newly created directory: %w", err)
		} else {
			if stat.Mode != expectedMode {
				return nil, fmt.Errorf("%w: newly created directory %q has incorrect mode 0o%.3o (expected 0o%.3o)", errPossibleAttack, currentDir.Name(), stat.Mode, expectedMode)
			}
			if stat.Uid != expectedUid || stat.Gid != expectedGid {
				return nil, fmt.Errorf("%w: newly created directory %q has incorrect owner %d:%d (expected %d:%d)", errPossibleAttack, currentDir.Name(), stat.Uid, stat.Gid, expectedUid, expectedGid)
			}
			// Check that the directory is empty. We only need to check for
			// a single entry, and we should get EOF if the directory is
			// empty.
			_, err := currentDir.Readdirnames(1)
			if !errors.Is(err, io.EOF) {
				if err == nil {
					err = fmt.Errorf("%w: newly created directory %q is non-empty", errPossibleAttack, currentDir.Name())
				}
				return nil, fmt.Errorf("check if newly created directory %q is empty: %w", currentDir.Name(), err)
			}
			// Reset the offset.
			_, _ = currentDir.Seek(0, unix.SEEK_SET)
		}
	}
	return currentDir, nil
}

// MkdirAll is a race-safe alternative to the Go stdlib's os.MkdirAll function,
// where the new directory is guaranteed to be within the root directory (if an
// attacker can move directories from inside the root to outside the root, the
// created directory tree might be outside of the root but the key constraint
// is that at no point will we walk outside of the directory tree we are
// creating).
//
// Effectively, MkdirAll(root, unsafePath, mode) is equivalent to
//
//	path, _ := securejoin.SecureJoin(root, unsafePath)
//	err := os.MkdirAll(path, mode)
//
// But is much safer. The above implementation is unsafe because if an attacker
// can modify the filesystem tree between SecureJoin and MkdirAll, it is
// possible for MkdirAll to resolve unsafe symlink components and create
// directories outside of the root.
//
// If you plan to open the directory after you have created it or want to use
// an open directory handle as the root, you should use MkdirAllHandle instead.
// This function is a wrapper around MkdirAllHandle.
//
// NOTE: The mode argument must be set the unix mode bits (unix.S_I...), not
// the Go generic mode bits (os.Mode...).
func MkdirAll(root, unsafePath string, mode int) error {
	rootDir, err := os.OpenFile(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer rootDir.Close()

	f, err := MkdirAllHandle(rootDir, unsafePath, mode)
	if err != nil {
		return err
	}
	_ = f.Close()
	return nil
}
