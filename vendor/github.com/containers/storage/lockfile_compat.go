package storage

import (
	"github.com/containers/storage/pkg/lockfile"
)

type Locker = lockfile.Locker

func GetLockfile(path string) (lockfile.Locker, error) {
	return lockfile.GetLockfile(path)
}

func GetROLockfile(path string) (lockfile.Locker, error) {
	return lockfile.GetROLockfile(path)
}

func GetLockfileRWRO(path string, readOnly bool) (lockfile.Locker, error) {
	return lockfile.GetLockfileRWRO(path, readOnly)
}
