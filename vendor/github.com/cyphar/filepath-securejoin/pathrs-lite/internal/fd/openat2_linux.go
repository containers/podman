// SPDX-License-Identifier: MPL-2.0

//go:build linux

// Copyright (C) 2024-2025 Aleksa Sarai <cyphar@cyphar.com>
// Copyright (C) 2024-2025 SUSE LLC
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package fd

import (
	"errors"
	"os"
	"runtime"

	"golang.org/x/sys/unix"

	"github.com/cyphar/filepath-securejoin/pathrs-lite/internal"
)

func scopedLookupShouldRetry(how *unix.OpenHow, err error) bool {
	// RESOLVE_IN_ROOT (and RESOLVE_BENEATH) can return -EAGAIN if we resolve
	// ".." while a mount or rename occurs anywhere on the system. This could
	// happen spuriously, or as the result of an attacker trying to mess with
	// us during lookup.
	//
	// In addition, scoped lookups have a "safety check" at the end of
	// complete_walk which will return -EXDEV if the final path is not in the
	// root.
	return how.Resolve&(unix.RESOLVE_IN_ROOT|unix.RESOLVE_BENEATH) != 0 &&
		(errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EXDEV))
}

const scopedLookupMaxRetries = 32

// Openat2 is an [Fd]-based wrapper around unix.Openat2, but with some retry
// logic in case of EAGAIN errors.
func Openat2(dir Fd, path string, how *unix.OpenHow) (*os.File, error) {
	dirFd, fullPath := prepareAt(dir, path)
	// Make sure we always set O_CLOEXEC.
	how.Flags |= unix.O_CLOEXEC
	var tries int
	for tries < scopedLookupMaxRetries {
		fd, err := unix.Openat2(dirFd, path, how)
		if err != nil {
			if scopedLookupShouldRetry(how, err) {
				// We retry a couple of times to avoid the spurious errors, and
				// if we are being attacked then returning -EAGAIN is the best
				// we can do.
				tries++
				continue
			}
			return nil, &os.PathError{Op: "openat2", Path: fullPath, Err: err}
		}
		runtime.KeepAlive(dir)
		return os.NewFile(uintptr(fd), fullPath), nil
	}
	return nil, &os.PathError{Op: "openat2", Path: fullPath, Err: internal.ErrPossibleAttack}
}
