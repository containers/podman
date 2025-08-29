//go:build !linux && !remote

package abi

import (
	"errors"
	"os"
)

// openSymlinkPath is not supported on this platform.
func openSymlinkPath(root *os.File, unsafePath string, flags int) (*os.File, error) {
	return nil, errors.New("cannot safely open symlink on this platform")
}
