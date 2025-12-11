package storage

import (
	"github.com/containers/storage/pkg/lockfile"
)

// Deprecated: Use lockfile.*LockFile.
type Locker = lockfile.Locker //lint:ignore SA1019 // lockfile.Locker is deprecated

// Deprecated: Use lockfile.GetLockFile.
func GetLockfile(path string) (lockfile.Locker, error) {
	return lockfile.GetLockfile(path)
}

// Deprecated: Use lockfile.GetROLockFile.
func GetROLockfile(path string) (lockfile.Locker, error) {
	return lockfile.GetROLockfile(path)
}
