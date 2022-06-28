package vfs

import "github.com/containers/storage/drivers/copy"

func dirCopy(srcDir, dstDir string) error {
	return copy.DirCopy(srcDir, dstDir, copy.Content, true)
}
