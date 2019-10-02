// Copyright 2016 PLUMgrid
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bcc

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"unsafe"
)

/*
#cgo CFLAGS: -I/usr/include/bcc/compat
#cgo LDFLAGS: -lbcc
#include <bcc/bcc_common.h>
#include <bcc/libbpf.h>
*/
import "C"

var errIterationFailed = errors.New("table.Iter: leaf for next key not found")

// Table references a BPF table.  The zero value cannot be used.
type Table struct {
	id     C.size_t
	module *Module
}

// New tables returns a refernce to a BPF table.
func NewTable(id C.size_t, module *Module) *Table {
	return &Table{
		id:     id,
		module: module,
	}
}

// ID returns the table id.
func (table *Table) ID() string {
	return C.GoString(C.bpf_table_name(table.module.p, table.id))
}

// Name returns the table name.
func (table *Table) Name() string {
	return C.GoString(C.bpf_table_name(table.module.p, table.id))
}

// Config returns the table properties (name, fd, ...).
func (table *Table) Config() map[string]interface{} {
	mod := table.module.p
	return map[string]interface{}{
		"name":      C.GoString(C.bpf_table_name(mod, table.id)),
		"fd":        int(C.bpf_table_fd_id(mod, table.id)),
		"key_size":  uint64(C.bpf_table_key_size_id(mod, table.id)),
		"leaf_size": uint64(C.bpf_table_leaf_size_id(mod, table.id)),
		"key_desc":  C.GoString(C.bpf_table_key_desc_id(mod, table.id)),
		"leaf_desc": C.GoString(C.bpf_table_leaf_desc_id(mod, table.id)),
	}
}

func (table *Table) LeafStrToBytes(leafStr string) ([]byte, error) {
	mod := table.module.p

	leafSize := C.bpf_table_leaf_size_id(mod, table.id)
	leaf := make([]byte, leafSize)
	leafP := unsafe.Pointer(&leaf[0])

	leafCS := C.CString(leafStr)
	defer C.free(unsafe.Pointer(leafCS))

	r := C.bpf_table_leaf_sscanf(mod, table.id, leafCS, leafP)
	if r != 0 {
		return nil, fmt.Errorf("error scanning leaf (%v) from string", leafStr)
	}
	return leaf, nil
}

func (table *Table) KeyStrToBytes(keyStr string) ([]byte, error) {
	mod := table.module.p

	keySize := C.bpf_table_key_size_id(mod, table.id)
	key := make([]byte, keySize)
	keyP := unsafe.Pointer(&key[0])

	keyCS := C.CString(keyStr)
	defer C.free(unsafe.Pointer(keyCS))

	r := C.bpf_table_key_sscanf(mod, table.id, keyCS, keyP)
	if r != 0 {
		return nil, fmt.Errorf("error scanning key (%v) from string", keyStr)
	}
	return key, nil
}

// KeyBytesToStr returns the given key value formatted using the bcc-table's key string printer.
func (table *Table) KeyBytesToStr(key []byte) (string, error) {
	keySize := len(key)
	keyP := unsafe.Pointer(&key[0])

	keyStr := make([]byte, keySize*8)
	keyStrP := (*C.char)(unsafe.Pointer(&keyStr[0]))

	if res := C.bpf_table_key_snprintf(table.module.p, table.id, keyStrP, C.size_t(len(keyStr)), keyP); res != 0 {
		return "", fmt.Errorf("formatting table-key: %d", res)
	}

	return string(keyStr[:bytes.IndexByte(keyStr, 0)]), nil
}

// LeafBytesToStr returns the given leaf value formatted using the bcc-table's leaf string printer.
func (table *Table) LeafBytesToStr(leaf []byte) (string, error) {
	leafSize := len(leaf)
	leafP := unsafe.Pointer(&leaf[0])

	leafStr := make([]byte, leafSize*8)
	leafStrP := (*C.char)(unsafe.Pointer(&leafStr[0]))

	if res := C.bpf_table_leaf_snprintf(table.module.p, table.id, leafStrP, C.size_t(len(leafStr)), leafP); res != 0 {
		return "", fmt.Errorf("formatting table-leaf: %d", res)
	}

	return string(leafStr[:bytes.IndexByte(leafStr, 0)]), nil
}

// Get takes a key and returns the value or nil, and an 'ok' style indicator.
func (table *Table) Get(key []byte) ([]byte, error) {
	mod := table.module.p
	fd := C.bpf_table_fd_id(mod, table.id)

	keyP := unsafe.Pointer(&key[0])

	leafSize := C.bpf_table_leaf_size_id(mod, table.id)
	leaf := make([]byte, leafSize)
	leafP := unsafe.Pointer(&leaf[0])

	r, err := C.bpf_lookup_elem(fd, keyP, leafP)
	if r != 0 {
		keyStr, errK := table.KeyBytesToStr(key)
		if errK != nil {
			keyStr = fmt.Sprintf("%v", key)
		}
		return nil, fmt.Errorf("Table.Get: key %v: %v", keyStr, err)
	}

	return leaf, nil
}

// GetP takes a key and returns the value or nil.
func (table *Table) GetP(key unsafe.Pointer) (unsafe.Pointer, error) {
	fd := C.bpf_table_fd_id(table.module.p, table.id)

	leafSize := C.bpf_table_leaf_size_id(table.module.p, table.id)
	leaf := make([]byte, leafSize)
	leafP := unsafe.Pointer(&leaf[0])

	_, err := C.bpf_lookup_elem(fd, key, leafP)
	if err != nil {
		return nil, err
	}
	return leafP, nil
}

// Set a key to a value.
func (table *Table) Set(key, leaf []byte) error {
	fd := C.bpf_table_fd_id(table.module.p, table.id)

	keyP := unsafe.Pointer(&key[0])
	leafP := unsafe.Pointer(&leaf[0])

	r, err := C.bpf_update_elem(fd, keyP, leafP, 0)
	if r != 0 {
		keyStr, errK := table.KeyBytesToStr(key)
		if errK != nil {
			keyStr = fmt.Sprintf("%v", key)
		}
		leafStr, errL := table.LeafBytesToStr(leaf)
		if errL != nil {
			leafStr = fmt.Sprintf("%v", leaf)
		}

		return fmt.Errorf("Table.Set: update %v to %v: %v", keyStr, leafStr, err)
	}

	return nil
}

// SetP a key to a value as unsafe.Pointer.
func (table *Table) SetP(key, leaf unsafe.Pointer) error {
	fd := C.bpf_table_fd_id(table.module.p, table.id)

	_, err := C.bpf_update_elem(fd, key, leaf, 0)
	if err != nil {
		return err
	}

	return nil
}

// Delete a key.
func (table *Table) Delete(key []byte) error {
	fd := C.bpf_table_fd_id(table.module.p, table.id)
	keyP := unsafe.Pointer(&key[0])
	r, err := C.bpf_delete_elem(fd, keyP)
	if r != 0 {
		keyStr, errK := table.KeyBytesToStr(key)
		if errK != nil {
			keyStr = fmt.Sprintf("%v", key)
		}
		return fmt.Errorf("Table.Delete: key %v: %v", keyStr, err)
	}
	return nil
}

// DeleteP a key.
func (table *Table) DeleteP(key unsafe.Pointer) error {
	fd := C.bpf_table_fd_id(table.module.p, table.id)
	_, err := C.bpf_delete_elem(fd, key)
	if err != nil {
		return err
	}
	return nil
}

// DeleteAll deletes all entries from the table
func (table *Table) DeleteAll() error {
	mod := table.module.p
	fd := C.bpf_table_fd_id(mod, table.id)

	keySize := C.bpf_table_key_size_id(mod, table.id)
	key := make([]byte, keySize)
	keyP := unsafe.Pointer(&key[0])
	for res := C.bpf_get_first_key(fd, keyP, keySize); res == 0; res = C.bpf_get_next_key(fd, keyP, keyP) {
		r, err := C.bpf_delete_elem(fd, keyP)
		if r != 0 {
			return fmt.Errorf("Table.DeleteAll: unable to delete element: %v", err)
		}
	}
	return nil
}

// TableIterator contains the current position for iteration over a *bcc.Table and provides methods for iteration.
type TableIterator struct {
	table *Table
	fd    C.int

	err error

	key  []byte
	leaf []byte
}

// Iter returns an iterator to list all table entries available as raw bytes.
func (table *Table) Iter() *TableIterator {
	fd := C.bpf_table_fd_id(table.module.p, table.id)

	return &TableIterator{
		table: table,
		fd:    fd,
	}
}

// Next looks up the next element and return true if one is available.
func (it *TableIterator) Next() bool {
	if it.err != nil {
		return false
	}

	if it.key == nil {
		keySize := C.bpf_table_key_size_id(it.table.module.p, it.table.id)

		key := make([]byte, keySize)
		keyP := unsafe.Pointer(&key[0])
		if res, err := C.bpf_get_first_key(it.fd, keyP, keySize); res != 0 {
			if !os.IsNotExist(err) {
				it.err = err
			}
			return false
		}

		leafSize := C.bpf_table_leaf_size_id(it.table.module.p, it.table.id)
		leaf := make([]byte, leafSize)

		it.key = key
		it.leaf = leaf
	} else {
		keyP := unsafe.Pointer(&it.key[0])
		if res, err := C.bpf_get_next_key(it.fd, keyP, keyP); res != 0 {
			if !os.IsNotExist(err) {
				it.err = err
			}
			return false
		}
	}

	keyP := unsafe.Pointer(&it.key[0])
	leafP := unsafe.Pointer(&it.leaf[0])
	if res, err := C.bpf_lookup_elem(it.fd, keyP, leafP); res != 0 {
		it.err = errIterationFailed
		if !os.IsNotExist(err) {
			it.err = err
		}
		return false
	}

	return true
}

// Key returns the current key value of the iterator, if the most recent call to Next returned true.
// The slice is valid only until the next call to Next.
func (it *TableIterator) Key() []byte {
	return it.key
}

// Leaf returns the current leaf value of the iterator, if the most recent call to Next returned true.
// The slice is valid only until the next call to Next.
func (it *TableIterator) Leaf() []byte {
	return it.leaf
}

// Err returns the last error that ocurred while table.Iter oder iter.Next
func (it *TableIterator) Err() error {
	return it.err
}
