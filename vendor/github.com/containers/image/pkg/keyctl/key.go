// Copyright 2015 Jesse Sipprell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

package keyctl

import (
	"golang.org/x/sys/unix"
)

// Key represents a single key linked to one or more kernel keyrings.
type Key struct {
	Name string

	id, ring keyID
	size     int
}

// ID returns the 32-bit kernel identifier for a specific key
func (k *Key) ID() int32 {
	return int32(k.id)
}

// Get the key's value as a byte slice
func (k *Key) Get() ([]byte, error) {
	var (
		b        []byte
		err      error
		sizeRead int
	)

	if k.size == 0 {
		k.size = 512
	}

	size := k.size

	b = make([]byte, int(size))
	sizeRead = size + 1
	for sizeRead > size {
		r1, err := unix.KeyctlBuffer(unix.KEYCTL_READ, int(k.id), b, size)
		if err != nil {
			return nil, err
		}

		if sizeRead = int(r1); sizeRead > size {
			b = make([]byte, sizeRead)
			size = sizeRead
			sizeRead = size + 1
		} else {
			k.size = sizeRead
		}
	}
	return b[:k.size], err
}

// Unlink a key from the keyring it was loaded from (or added to). If the key
// is not linked to any other keyrings, it is destroyed.
func (k *Key) Unlink() error {
	_, err := unix.KeyctlInt(unix.KEYCTL_UNLINK, int(k.id), int(k.ring), 0, 0)
	return err
}
