// +build !linux

package vfs // import "github.com/containers/storage/drivers/vfs"

import "github.com/containers/storage/pkg/chrootarchive"

func dirCopy(srcDir, dstDir string) error {
	return chrootarchive.NewArchiver(nil).CopyWithTar(srcDir, dstDir)
}
