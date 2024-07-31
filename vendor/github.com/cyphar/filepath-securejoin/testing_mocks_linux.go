//go:build linux

// Copyright (C) 2024 SUSE LLC. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package securejoin

import (
	"os"
	"testing"
)

type forceGetProcRootLevel int

const (
	forceGetProcRootDefault             forceGetProcRootLevel = iota
	forceGetProcRootOpenTree                                  // force open_tree()
	forceGetProcRootOpenTreeAtRecursive                       // force open_tree(AT_RECURSIVE)
	forceGetProcRootUnsafe                                    // force open()
)

var testingForceGetProcRoot *forceGetProcRootLevel

func testingCheckClose(check bool, f *os.File) bool {
	if check {
		if f != nil {
			_ = f.Close()
		}
		return true
	}
	return false
}

func testingForcePrivateProcRootOpenTree(f *os.File) bool {
	return testing.Testing() && testingForceGetProcRoot != nil &&
		testingCheckClose(*testingForceGetProcRoot >= forceGetProcRootOpenTree, f)
}

func testingForcePrivateProcRootOpenTreeAtRecursive(f *os.File) bool {
	return testing.Testing() && testingForceGetProcRoot != nil &&
		testingCheckClose(*testingForceGetProcRoot >= forceGetProcRootOpenTreeAtRecursive, f)
}

func testingForceGetProcRootUnsafe() bool {
	return testing.Testing() && testingForceGetProcRoot != nil &&
		*testingForceGetProcRoot >= forceGetProcRootUnsafe
}

type forceProcThreadSelfLevel int

const (
	forceProcThreadSelfDefault forceProcThreadSelfLevel = iota
	forceProcSelfTask
	forceProcSelf
)

var testingForceProcThreadSelf *forceProcThreadSelfLevel

func testingForceProcSelfTask() bool {
	return testing.Testing() && testingForceProcThreadSelf != nil &&
		*testingForceProcThreadSelf >= forceProcSelfTask
}

func testingForceProcSelf() bool {
	return testing.Testing() && testingForceProcThreadSelf != nil &&
		*testingForceProcThreadSelf >= forceProcSelf
}
