// Copyright 2015 Jesse Sipprell. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

// Package keyctl is a Go interface to linux kernel keyrings (keyctl interface)
package keyctl

import (
	"unsafe"

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

// ReadUserKeyring reads user keyring and returns slice of key with id(key_serial_t) representing the IDs of all the keys that are linked to it
func ReadUserKeyring() ([]*Key, error) {
	var (
		b        []byte
		err      error
		sizeRead int
	)
	krSize := 4
	size := krSize
	b = make([]byte, size)
	sizeRead = size + 1
	for sizeRead > size {
		r1, err := unix.KeyctlBuffer(unix.KEYCTL_READ, unix.KEY_SPEC_USER_KEYRING, b, size)
		if err != nil {
			return nil, err
		}

		if sizeRead = int(r1); sizeRead > size {
			b = make([]byte, sizeRead)
			size = sizeRead
			sizeRead = size + 1
		} else {
			krSize = sizeRead
		}
	}
	keyIDs := getKeyIDsFromByte(b[:krSize])
	return keyIDs, err
}

func getKeyIDsFromByte(byteKeyIDs []byte) []*Key {
	idSize := 4
	var keys []*Key
	for idx := 0; idx+idSize <= len(byteKeyIDs); idx = idx + idSize {
		tempID := *(*int32)(unsafe.Pointer(&byteKeyIDs[idx]))
		keys = append(keys, &Key{id: keyID(tempID)})
	}
	return keys
}
