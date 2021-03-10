// +build windows

package directory

import (
	"os"
	"path/filepath"
)

// Size walks a directory tree and returns its total size in bytes
func Size(dir string) (size int64, err error) {
	usage, err := Usage(dir)
	if err != nil {
		return 0, nil
	}
	return usage.Size, nil
}

// Usage walks a directory tree and returns its total size in bytes and the number of inodes.
func Usage(dir string) (usage *DiskUsage, err error) {
	usage = &DiskUsage{}
	err = filepath.Walk(dir, func(d string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			// if dir does not exist, Size() returns the error.
			// if dir/x disappeared while walking, Size() ignores dir/x.
			if os.IsNotExist(err) && d != dir {
				return nil
			}
			return err
		}

		usage.InodeCount++

		// Ignore directory sizes
		if fileInfo == nil {
			return nil
		}

		s := fileInfo.Size()
		if fileInfo.IsDir() || s == 0 {
			return nil
		}

		usage.Size += s

		return nil
	})
	return
}
