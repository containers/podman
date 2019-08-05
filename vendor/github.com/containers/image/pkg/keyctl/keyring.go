// Copyright 2015 Jesse Sipprell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

// Package keyctl is a Go interface to linux kernel keyrings (keyctl interface)
//
// Deprecated: Most callers should use either golang.org/x/sys/unix directly,
// or the original (and more extensive) github.com/jsipprell/keyctl .
package keyctl

import (
	"golang.org/x/sys/unix"
)

// Keyring is the basic interface to a linux keyctl keyring.
type Keyring interface {
	ID
	Add(string, []byte) (*Key, error)
	Search(string) (*Key, error)
}

type keyring struct {
	id keyID
}

// ID is unique 32-bit serial number identifiers for all Keys and Keyrings have.
type ID interface {
	ID() int32
}

// Add a new key to a keyring. The key can be searched for later by name.
func (kr *keyring) Add(name string, key []byte) (*Key, error) {
	r, err := unix.AddKey("user", name, key, int(kr.id))
	if err == nil {
		key := &Key{Name: name, id: keyID(r), ring: kr.id}
		return key, nil
	}
	return nil, err
}

// Search for a key by name, this also searches child keyrings linked to this
// one. The key, if found, is linked to the top keyring that Search() was called
// from.
func (kr *keyring) Search(name string) (*Key, error) {
	id, err := unix.KeyctlSearch(int(kr.id), "user", name, 0)
	if err == nil {
		return &Key{Name: name, id: keyID(id), ring: kr.id}, nil
	}
	return nil, err
}

// ID returns the 32-bit kernel identifier of a keyring
func (kr *keyring) ID() int32 {
	return int32(kr.id)
}

// SessionKeyring returns the current login session keyring
func SessionKeyring() (Keyring, error) {
	return newKeyring(unix.KEY_SPEC_SESSION_KEYRING)
}

// UserKeyring  returns the keyring specific to the current user.
func UserKeyring() (Keyring, error) {
	return newKeyring(unix.KEY_SPEC_USER_KEYRING)
}

// Unlink an object from a keyring
func Unlink(parent Keyring, child ID) error {
	_, err := unix.KeyctlInt(unix.KEYCTL_UNLINK, int(child.ID()), int(parent.ID()), 0, 0)
	return err
}

// Link a key into a keyring
func Link(parent Keyring, child ID) error {
	_, err := unix.KeyctlInt(unix.KEYCTL_LINK, int(child.ID()), int(parent.ID()), 0, 0)
	return err
}
