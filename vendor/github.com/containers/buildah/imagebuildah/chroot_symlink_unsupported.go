// +build !linux

package imagebuildah

import "github.com/pkg/errors"

func resolveSymlink(rootdir, filename string) (string, error) {
	return "", errors.New("function not supported on non-linux systems")
}

func resolveModifiedTime(rootdir, filename, historyTime string) (bool, error) {
	return false, errors.New("function not supported on non-linux systems")
}
