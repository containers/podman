//go:build !linux && !freebsd
// +build !linux,!freebsd

package libpod

import (
	"errors"
	"io"
)

func (c *Container) copyFromArchive(path string, chown, noOverwriteDirNonDir bool, rename map[string]string, reader io.Reader) (func() error, error) {
	return nil, errors.New("not implemented (*Container) copyFromArchive")
}

func (c *Container) copyToArchive(path string, writer io.Writer) (func() error, error) {
	return nil, errors.New("not implemented (*Container) copyToArchive")
}
