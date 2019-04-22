// +build windows

package util

import (
	"github.com/pkg/errors"
)

// GetRootlessRuntimeDir returns the runtime directory when running as non root
func GetRootlessRuntimeDir() (string, error) {
	return "", errors.New("this function is not implemented for windows")
}
