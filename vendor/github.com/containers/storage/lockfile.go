package storage

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// A Locker represents a file lock where the file is used to cache an
// identifier of the last party that made changes to whatever's being protected
// by the lock.
type Locker interface {
	sync.Locker

	// Touch records, for others sharing the lock, that the caller was the
	// last writer.  It should only be called with the lock held.
	Touch() error

	// Modified() checks if the most recent writer was a party other than the
	// last recorded writer.  It should only be called with the lock held.
	Modified() (bool, error)

	// TouchedSince() checks if the most recent writer modified the file (likely using Touch()) after the specified time.
	TouchedSince(when time.Time) bool

	// IsReadWrite() checks if the lock file is read-write
	IsReadWrite() bool

	// Locked() checks if lock is locked
	Locked() bool
}

var (
	lockfiles     map[string]Locker
	lockfilesLock sync.Mutex
)

// GetLockfile opens a read-write lock file, creating it if necessary.  The
// Locker object it returns will be returned unlocked.
func GetLockfile(path string) (Locker, error) {
	lockfilesLock.Lock()
	defer lockfilesLock.Unlock()
	if lockfiles == nil {
		lockfiles = make(map[string]Locker)
	}
	cleanPath := filepath.Clean(path)
	if locker, ok := lockfiles[cleanPath]; ok {
		if !locker.IsReadWrite() {
			return nil, errors.Wrapf(ErrLockReadOnly, "lock %q is a read-only lock", cleanPath)
		}
		return locker, nil
	}
	locker, err := getLockFile(path, false) // platform dependent locker
	if err != nil {
		return nil, err
	}
	lockfiles[filepath.Clean(path)] = locker
	return locker, nil
}

// GetROLockfile opens a read-only lock file.  The Locker object it returns
// will be returned unlocked.
func GetROLockfile(path string) (Locker, error) {
	lockfilesLock.Lock()
	defer lockfilesLock.Unlock()
	if lockfiles == nil {
		lockfiles = make(map[string]Locker)
	}
	cleanPath := filepath.Clean(path)
	if locker, ok := lockfiles[cleanPath]; ok {
		if locker.IsReadWrite() {
			return nil, fmt.Errorf("lock %q is a read-write lock", cleanPath)
		}
		return locker, nil
	}
	locker, err := getLockFile(path, true) // platform dependent locker
	if err != nil {
		return nil, err
	}
	lockfiles[filepath.Clean(path)] = locker
	return locker, nil
}
