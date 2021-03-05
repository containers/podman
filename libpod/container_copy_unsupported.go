// +build !linux

package libpod

import (
	"context"
	"io"
)

func (c *Container) copyFromArchive(ctx context.Context, path string, reader io.Reader) (func() error, error) {
	return nil, nil
}

func (c *Container) copyToArchive(ctx context.Context, path string, writer io.Writer) (func() error, error) {
	return nil, nil
}
