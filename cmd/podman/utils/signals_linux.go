//go:build !windows
// +build !windows

package utils

import (
	"os"

	"golang.org/x/sys/unix"
)

// Platform specific signal synonyms
var (
	SIGHUP os.Signal = unix.SIGHUP
)
