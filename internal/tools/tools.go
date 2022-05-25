//go:build tools
// +build tools

// This file is a NOP. It exists solely because we need k8s.io/release
// for bin/release-notes, and the import below tells Go to keep that
// line in go.mod.

package tools

import (
	_ "k8s.io/release/cmd/release-notes"
)
