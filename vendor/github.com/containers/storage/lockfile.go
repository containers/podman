package storage

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// A Locker represents a file lock where the file is used to cache an
// identifier of the last party that made changes to whatever's being protected
// by the lock.
type Locker interface {
	// Acquire a writer lock.
	Lock()

	// Unlock the lock.
	Unlock()

	// Acquire a reader lock.
	RLock()

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

	// Locked() checks if lock is locked for writing by a thread in this process
	Locked() bool
}

var (
	lockfiles     map[string]Locker
	lockfilesLock sync.Mutex
)

// GetLockfile opens a read-write lock file, creating it if necessary.  The
// Locker object may already be locked if the path has already been requested
// by the current process.
func GetLockfile(path string) (Locker, error) {
	return getLockfile(path, false)
}

// GetROLockfile opens a read-only lock file, creating it if necessary.  The
// Locker object may already be locked if the path has already been requested
// by the current process.
func GetROLockfile(path string) (Locker, error) {
	return getLockfile(path, true)
}

// getLockfile is a helper for GetLockfile and GetROLockfile and returns Locker
// based on the path and read-only property.
func getLockfile(path string, ro bool) (Locker, error) {
	lockfilesLock.Lock()
	defer lockfilesLock.Unlock()
	if lockfiles == nil {
		lockfiles = make(map[string]Locker)
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.Wrapf(err, "error ensuring that path %q is an absolute path", path)
	}
	if locker, ok := lockfiles[cleanPath]; ok {
		if ro && locker.IsReadWrite() {
			return nil, errors.Errorf("lock %q is not a read-only lock", cleanPath)
		}
		if !ro && !locker.IsReadWrite() {
			return nil, errors.Errorf("lock %q is not a read-write lock", cleanPath)
		}
		return locker, nil
	}
	locker, err := getLockFile(path, ro) // platform dependent locker
	if err != nil {
		return nil, err
	}
	lockfiles[filepath.Clean(path)] = locker
	return locker, nil
}
