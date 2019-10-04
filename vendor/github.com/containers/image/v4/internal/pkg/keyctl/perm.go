// Copyright 2015 Jesse Sipprell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

package keyctl

import (
	"golang.org/x/sys/unix"
)

// KeyPerm represents in-kernel access control permission to keys and keyrings
// as a 32-bit integer broken up into four permission sets, one per byte.
// In MSB order, the perms are: Processor, User, Group, Other.
type KeyPerm uint32

const (
	// PermOtherAll sets all permission for Other
	PermOtherAll KeyPerm = 0x3f << (8 * iota)
	// PermGroupAll sets all permission for Group
	PermGroupAll
	// PermUserAll sets all permission for User
	PermUserAll
	// PermProcessAll sets all permission for Processor
	PermProcessAll
)

// SetPerm sets the permissions on a key or keyring.
func SetPerm(k ID, p KeyPerm) error {
	err := unix.KeyctlSetperm(int(k.ID()), uint32(p))
	return err
}
