package system

import (
	"io/fs"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

func lchflags(path string, flags int) (err error) {
	p, err := unix.BytePtrFromString(path)
	if err != nil {
		return err
	}
	_, _, e1 := unix.Syscall(unix.SYS_LCHFLAGS, uintptr(unsafe.Pointer(p)), uintptr(flags), 0)
	if e1 != 0 {
		return e1
	}
	return nil
}

// Reset file flags in a directory tree. This allows EnsureRemoveAll
// to delete trees which have the immutable flag set.
func resetFileFlags(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err := lchflags(path, 0); err != nil {
			return err
		}
		return nil
	})
}
