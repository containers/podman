package file

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// FileLocks is a struct enabling POSIX lock locking in a shared memory
// segment.
type FileLocks struct { // nolint
	lockPath string
	valid    bool
}

// CreateFileLock sets up a directory containing the various lock files.
func CreateFileLock(path string) (*FileLocks, error) {
	_, err := os.Stat(path)
	if err == nil {
		return nil, errors.Wrapf(syscall.EEXIST, "directory %s exists", path)
	}
	if err := os.MkdirAll(path, 0711); err != nil {
		return nil, err
	}

	locks := new(FileLocks)
	locks.lockPath = path
	locks.valid = true

	return locks, nil
}

// OpenFileLock opens an existing directory with the lock files.
func OpenFileLock(path string) (*FileLocks, error) {
	_, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	locks := new(FileLocks)
	locks.lockPath = path
	locks.valid = true

	return locks, nil
}

// Close closes an existing shared-memory segment.
// The segment will be rendered unusable after closing.
// WARNING: If you Close() while there are still locks locked, these locks may
// fail to release, causing a program freeze.
// Close() is only intended to be used while testing the locks.
func (locks *FileLocks) Close() error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}
	err := os.RemoveAll(locks.lockPath)
	if err != nil {
		return errors.Wrapf(err, "deleting directory %s", locks.lockPath)
	}
	return nil
}

func (locks *FileLocks) getLockPath(lck uint32) string {
	return filepath.Join(locks.lockPath, strconv.FormatInt(int64(lck), 10))
}

// AllocateLock allocates a lock and returns the index of the lock that was allocated.
func (locks *FileLocks) AllocateLock() (uint32, error) {
	if !locks.valid {
		return 0, errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	id := uint32(0)
	for ; ; id++ {
		path := locks.getLockPath(id)
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return 0, errors.Wrap(err, "creating lock file")
		}
		f.Close()
		break
	}
	return id, nil
}

// AllocateGivenLock allocates the given lock from the shared-memory
// segment for use by a container or pod.
// If the lock is already in use or the index is invalid an error will be
// returned.
func (locks *FileLocks) AllocateGivenLock(lck uint32) error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	f, err := os.OpenFile(locks.getLockPath(lck), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return errors.Wrapf(err, "error creating lock %d", lck)
	}
	f.Close()

	return nil
}

// DeallocateLock frees a lock in a shared-memory segment so it can be
// reallocated to another container or pod.
// The given lock must be already allocated, or an error will be returned.
func (locks *FileLocks) DeallocateLock(lck uint32) error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}
	if err := os.Remove(locks.getLockPath(lck)); err != nil {
		return errors.Wrapf(err, "deallocating lock %d", lck)
	}
	return nil
}

// DeallocateAllLocks frees all locks so they can be reallocated to
// other containers and pods.
func (locks *FileLocks) DeallocateAllLocks() error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}
	files, err := ioutil.ReadDir(locks.lockPath)
	if err != nil {
		return errors.Wrapf(err, "error reading directory %s", locks.lockPath)
	}
	var lastErr error
	for _, f := range files {
		p := filepath.Join(locks.lockPath, f.Name())
		err := os.Remove(p)
		if err != nil {
			lastErr = err
			logrus.Errorf("Deallocating lock %s", p)
		}
	}
	return lastErr
}

// LockFileLock locks the given lock.
func (locks *FileLocks) LockFileLock(lck uint32) error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	l, err := storage.GetLockfile(locks.getLockPath(lck))
	if err != nil {
		return errors.Wrapf(err, "error acquiring lock")
	}

	l.Lock()
	return nil
}

// UnlockFileLock unlocks the given lock.
func (locks *FileLocks) UnlockFileLock(lck uint32) error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}
	l, err := storage.GetLockfile(locks.getLockPath(lck))
	if err != nil {
		return errors.Wrapf(err, "error acquiring lock")
	}

	l.Unlock()
	return nil
}
