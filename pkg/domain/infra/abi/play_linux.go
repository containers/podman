//go:build !remote

package abi

import (
	"os"

	securejoin "github.com/cyphar/filepath-securejoin"
)

// openSymlinkPath opens the path under root using securejoin.OpenatInRoot().
func openSymlinkPath(root *os.File, unsafePath string, flags int) (*os.File, error) {
	file, err := securejoin.OpenatInRoot(root, unsafePath)
	if err != nil {
		return nil, err
	}
	return securejoin.Reopen(file, flags)
}
