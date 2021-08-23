// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !plan9
// +build !plan9

// Package fsnotify provides a platform-independent interface for file system notifications.
package fsnotify

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
)

// Event represents a single file system notification.
type Event struct {
	Name string // Relative path to the file or directory.
	Op   Op     // File operation that triggered the event.
}

// Op describes a set of file operations.
type Op uint32

// These are the generalized file operations that can trigger a notification.
const (
	Create Op = 1 << iota
	Write
	Remove
	Rename
	Chmod
)

func (op Op) String() string {
	// Use a buffer for efficient string concatenation
	var buffer bytes.Buffer

	if op&Create == Create {
		buffer.WriteString("|CREATE")
	}
	if op&Remove == Remove {
		buffer.WriteString("|REMOVE")
	}
	if op&Write == Write {
		buffer.WriteString("|WRITE")
	}
	if op&Rename == Rename {
		buffer.WriteString("|RENAME")
	}
	if op&Chmod == Chmod {
		buffer.WriteString("|CHMOD")
	}
	if buffer.Len() == 0 {
		return ""
	}
	return buffer.String()[1:] // Strip leading pipe
}

// String returns a string representation of the event in the form
// "file: REMOVE|WRITE|..."
func (e Event) String() string {
	return fmt.Sprintf("%q: %s", e.Name, e.Op.String())
}

// Common errors that can be reported by a watcher
var (
	ErrEventOverflow = errors.New("fsnotify queue overflow")
)

// Add starts watching the named file or directory (non-recursively). Symlinks are implicitly resolved.
func (w *Watcher) Add(name string) error {
	rawPath, err := filepath.EvalSymlinks(name)
	if err != nil {
		return fmt.Errorf("error resolving %#v: %s", name, err)
	}
	err = w.AddRaw(rawPath)
	if err != nil {
		if name != rawPath {
			return fmt.Errorf("error adding %#v for %#v: %s", rawPath, name, err)
		}
		return err
	}
	return nil
}
