package chrootarchive

import (
	"io"

	"github.com/containers/storage/pkg/archive"
)

func chroot(path string) error {
	return nil
}

func invokeUnpack(decompressedArchive io.ReadCloser,
	dest string,
	options *archive.TarOptions, root string) error {
	return archive.Unpack(decompressedArchive, dest, options)
}

func invokePack(srcPath string, options *archive.TarOptions, root string) (io.ReadCloser, error) {
	return archive.TarWithOptions(srcPath, options)
}
