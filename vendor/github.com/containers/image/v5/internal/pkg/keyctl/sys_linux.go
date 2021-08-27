// Copyright 2015 Jesse Sipprell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build linux
// +build linux

package keyctl

import (
	"golang.org/x/sys/unix"
)

type keyID int32

func newKeyring(id keyID) (*keyring, error) {
	r1, err := unix.KeyctlGetKeyringID(int(id), true)
	if err != nil {
		return nil, err
	}

	if id < 0 {
		r1 = int(id)
	}
	return &keyring{id: keyID(r1)}, nil
}
