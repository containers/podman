// +build windows

package utils

import (
	"os"

	"golang.org/x/sys/windows"
)

// Platform specific signal synonyms
var (
	SIGHUP os.Signal = windows.SIGHUP
)
