// +build windows

package config

import (
	"github.com/pkg/errors"
)

// getRuntimeDir returns the runtime directory
func getRuntimeDir() (string, error) {
	return "", errors.New("this function is not implemented for windows")
}
