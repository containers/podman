// +build linux darwin freebsd solaris

package directory

import (
	"os"
	"path/filepath"
	"syscall"
)

// Size walks a directory tree and returns its total size in bytes.
func Size(dir string) (size int64, err error) {
	usage, err := Usage(dir)
	if err != nil {
		return 0, err
	}
	return usage.Size, nil
}

// Usage walks a directory tree and returns its total size in bytes and the number of inodes.
func Usage(dir string) (usage *DiskUsage, err error) {
	usage = &DiskUsage{}
	data := make(map[uint64]struct{})
	err = filepath.Walk(dir, func(d string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			// if dir does not exist, Usage() returns the error.
			// if dir/x disappeared while walking, Usage() ignores dir/x.
			if os.IsNotExist(err) && d != dir {
				return nil
			}
			return err
		}

		if fileInfo == nil {
			return nil
		}

		// Check inode to only count the sizes of files with multiple hard links once.
		inode := fileInfo.Sys().(*syscall.Stat_t).Ino
		// inode is not a uint64 on all platforms. Cast it to avoid issues.
		if _, exists := data[uint64(inode)]; exists {
			return nil
		}

		// inode is not a uint64 on all platforms. Cast it to avoid issues.
		data[uint64(inode)] = struct{}{}

		// Ignore directory sizes
		if fileInfo.IsDir() {
			return nil
		}

		usage.Size += fileInfo.Size()

		return nil
	})
	// inode count is the number of unique inode numbers we saw
	usage.InodeCount = int64(len(data))
	return
}
