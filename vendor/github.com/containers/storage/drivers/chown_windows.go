//go:build windows
// +build windows

package graphdriver

import (
	"os"
	"syscall"

	"github.com/containers/storage/pkg/idtools"
)

type platformChowner struct{}

func newLChowner() *platformChowner {
	return &platformChowner{}
}

func (c *platformChowner) LChown(path string, info os.FileInfo, toHost, toContainer *idtools.IDMappings) error {
	return &os.PathError{"lchown", path, syscall.EWINDOWS}
}
