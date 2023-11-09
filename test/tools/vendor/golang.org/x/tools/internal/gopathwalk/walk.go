// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gopathwalk is like filepath.Walk but specialized for finding Go
// packages, particularly in $GOPATH and $GOROOT.
package gopathwalk

import (
	"bufio"
	"bytes"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Options controls the behavior of a Walk call.
type Options struct {
	// If Logf is non-nil, debug logging is enabled through this function.
	Logf func(format string, args ...interface{})
	// Search module caches. Also disables legacy goimports ignore rules.
	ModulesEnabled bool
}

// RootType indicates the type of a Root.
type RootType int

const (
	RootUnknown RootType = iota
	RootGOROOT
	RootGOPATH
	RootCurrentModule
	RootModuleCache
	RootOther
)

// A Root is a starting point for a Walk.
type Root struct {
	Path string
	Type RootType
}

// Walk walks Go source directories ($GOROOT, $GOPATH, etc) to find packages.
// For each package found, add will be called (concurrently) with the absolute
// paths of the containing source directory and the package directory.
// add will be called concurrently.
func Walk(roots []Root, add func(root Root, dir string), opts Options) {
	WalkSkip(roots, add, func(Root, string) bool { return false }, opts)
}

// WalkSkip walks Go source directories ($GOROOT, $GOPATH, etc) to find packages.
// For each package found, add will be called (concurrently) with the absolute
// paths of the containing source directory and the package directory.
// For each directory that will be scanned, skip will be called (concurrently)
// with the absolute paths of the containing source directory and the directory.
// If skip returns false on a directory it will be processed.
// add will be called concurrently.
// skip will be called concurrently.
func WalkSkip(roots []Root, add func(root Root, dir string), skip func(root Root, dir string) bool, opts Options) {
	for _, root := range roots {
		walkDir(root, add, skip, opts)
	}
}

// walkDir creates a walker and starts fastwalk with this walker.
func walkDir(root Root, add func(Root, string), skip func(root Root, dir string) bool, opts Options) {
	if _, err := os.Stat(root.Path); os.IsNotExist(err) {
		if opts.Logf != nil {
			opts.Logf("skipping nonexistent directory: %v", root.Path)
		}
		return
	}
	start := time.Now()
	if opts.Logf != nil {
		opts.Logf("scanning %s", root.Path)
	}

	w := &walker{
		root:  root,
		add:   add,
		skip:  skip,
		opts:  opts,
		added: make(map[string]bool),
	}
	w.init()

	// Add a trailing path separator to cause filepath.WalkDir to traverse symlinks.
	path := root.Path
	if len(path) == 0 {
		path = "." + string(filepath.Separator)
	} else if !os.IsPathSeparator(path[len(path)-1]) {
		path = path + string(filepath.Separator)
	}

	if err := filepath.WalkDir(path, w.walk); err != nil {
		logf := opts.Logf
		if logf == nil {
			logf = log.Printf
		}
		logf("scanning directory %v: %v", root.Path, err)
	}

	if opts.Logf != nil {
		opts.Logf("scanned %s in %v", root.Path, time.Since(start))
	}
}

// walker is the callback for fastwalk.Walk.
type walker struct {
	root Root                    // The source directory to scan.
	add  func(Root, string)      // The callback that will be invoked for every possible Go package dir.
	skip func(Root, string) bool // The callback that will be invoked for every dir. dir is skipped if it returns true.
	opts Options                 // Options passed to Walk by the user.

	ignoredDirs []string

	added map[string]bool
}

// init initializes the walker based on its Options
func (w *walker) init() {
	var ignoredPaths []string
	if w.root.Type == RootModuleCache {
		ignoredPaths = []string{"cache"}
	}
	if !w.opts.ModulesEnabled && w.root.Type == RootGOPATH {
		ignoredPaths = w.getIgnoredDirs(w.root.Path)
		ignoredPaths = append(ignoredPaths, "v", "mod")
	}

	for _, p := range ignoredPaths {
		full := filepath.Join(w.root.Path, p)
		w.ignoredDirs = append(w.ignoredDirs, full)
		if w.opts.Logf != nil {
			w.opts.Logf("Directory added to ignore list: %s", full)
		}
	}
}

// getIgnoredDirs reads an optional config file at <path>/.goimportsignore
// of relative directories to ignore when scanning for go files.
// The provided path is one of the $GOPATH entries with "src" appended.
func (w *walker) getIgnoredDirs(path string) []string {
	file := filepath.Join(path, ".goimportsignore")
	slurp, err := os.ReadFile(file)
	if w.opts.Logf != nil {
		if err != nil {
			w.opts.Logf("%v", err)
		} else {
			w.opts.Logf("Read %s", file)
		}
	}
	if err != nil {
		return nil
	}

	var ignoredDirs []string
	bs := bufio.NewScanner(bytes.NewReader(slurp))
	for bs.Scan() {
		line := strings.TrimSpace(bs.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ignoredDirs = append(ignoredDirs, line)
	}
	return ignoredDirs
}

// shouldSkipDir reports whether the file should be skipped or not.
func (w *walker) shouldSkipDir(dir string) bool {
	for _, ignoredDir := range w.ignoredDirs {
		if dir == ignoredDir {
			return true
		}
	}
	if w.skip != nil {
		// Check with the user specified callback.
		return w.skip(w.root, dir)
	}
	return false
}

// walk walks through the given path.
func (w *walker) walk(path string, d fs.DirEntry, err error) error {
	typ := d.Type()
	if typ.IsRegular() {
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		dir := filepath.Dir(path)
		if dir == w.root.Path && (w.root.Type == RootGOROOT || w.root.Type == RootGOPATH) {
			// Doesn't make sense to have regular files
			// directly in your $GOPATH/src or $GOROOT/src.
			return nil
		}

		if !w.added[dir] {
			w.add(w.root, dir)
			w.added[dir] = true
		}
		return nil
	}
	if typ == os.ModeDir {
		base := filepath.Base(path)
		if base == "" || base[0] == '.' || base[0] == '_' ||
			base == "testdata" ||
			(w.root.Type == RootGOROOT && w.opts.ModulesEnabled && base == "vendor") ||
			(!w.opts.ModulesEnabled && base == "node_modules") {
			return filepath.SkipDir
		}
		if w.shouldSkipDir(path) {
			return filepath.SkipDir
		}
		return nil
	}
	if typ == os.ModeSymlink && err == nil {
		// TODO(bcmills): 'go list all' itself ignores symlinks within GOROOT/src
		// and GOPATH/src. Do we really need to traverse them here? If so, why?

		if os.IsPathSeparator(path[len(path)-1]) {
			// The OS was supposed to resolve a directory symlink but didn't.
			//
			// On macOS this may be caused by a known libc/kernel bug;
			// see https://go.dev/issue/59586.
			//
			// On Windows before Go 1.21, this may be caused by a bug in
			// os.Lstat (fixed in https://go.dev/cl/463177).
			//
			// In either case, we can work around the bug by walking this level
			// explicitly: first the symlink target itself, then its contents.

			fi, err := os.Stat(path)
			if err != nil || !fi.IsDir() {
				return nil
			}
			err = w.walk(path, fs.FileInfoToDirEntry(fi), nil)
			if err == filepath.SkipDir {
				return nil
			} else if err != nil {
				return err
			}

			ents, _ := os.ReadDir(path) // ignore error if unreadable
			for _, d := range ents {
				nextPath := filepath.Join(path, d.Name())
				var err error
				if d.IsDir() {
					err = filepath.WalkDir(nextPath, w.walk)
				} else {
					err = w.walk(nextPath, d, nil)
					if err == filepath.SkipDir {
						break
					}
				}
				if err != nil {
					return err
				}
			}
			return nil
		}

		base := filepath.Base(path)
		if strings.HasPrefix(base, ".#") {
			// Emacs noise.
			return nil
		}
		if w.shouldTraverse(path) {
			// Add a trailing separator to traverse the symlink.
			nextPath := path + string(filepath.Separator)
			return filepath.WalkDir(nextPath, w.walk)
		}
	}
	return nil
}

// shouldTraverse reports whether the symlink fi, found in dir,
// should be followed.  It makes sure symlinks were never visited
// before to avoid symlink loops.
func (w *walker) shouldTraverse(path string) bool {
	if w.shouldSkipDir(path) {
		return false
	}

	ts, err := os.Stat(path)
	if err != nil {
		logf := w.opts.Logf
		if logf == nil {
			logf = log.Printf
		}
		logf("%v", err)
		return false
	}
	if !ts.IsDir() {
		return false
	}

	// Check for symlink loops by statting each directory component
	// and seeing if any are the same file as ts.
	for {
		parent := filepath.Dir(path)
		if parent == path {
			// Made it to the root without seeing a cycle.
			// Use this symlink.
			return true
		}
		parentInfo, err := os.Stat(parent)
		if err != nil {
			return false
		}
		if os.SameFile(ts, parentInfo) {
			// Cycle. Don't traverse.
			return false
		}
		path = parent
	}

}
