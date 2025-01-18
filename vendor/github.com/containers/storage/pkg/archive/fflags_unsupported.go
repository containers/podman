//go:build !freebsd

package archive

import (
	"github.com/containers/storage/pkg/archive/hacktar"
	"os"
)

func ReadFileFlagsToTarHeader(path string, hdr *tar.Header) error {
	return nil
}

func WriteFileFlagsFromTarHeader(path string, hdr *tar.Header) error {
	return nil
}

func resetImmutable(path string, fi *os.FileInfo) error {
	return nil
}
