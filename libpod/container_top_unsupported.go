//go:build !(linux && cgo) && !freebsd
// +build !linux !cgo
// +build !freebsd

package libpod

import (
	"errors"
)

// Top gathers statistics about the running processes in a container. It returns a
// []string for output
func (c *Container) Top(descriptors []string) ([]string, error) {
	return nil, errors.New("not implemented (*Container) Top")
}
