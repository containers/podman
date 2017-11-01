package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/containers/storage/pkg/stringid"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
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
}

type lockfile struct {
	mu       sync.Mutex
	file     string
	fd       uintptr
	lw       string
	locktype int16
}

var (
	lockfiles     map[string]*lockfile
	lockfilesLock sync.Mutex
)

// GetLockfile opens a read-write lock file, creating it if necessary.  The
// Locker object it returns will be returned unlocked.
func GetLockfile(path string) (Locker, error) {
	lockfilesLock.Lock()
	defer lockfilesLock.Unlock()
	if lockfiles == nil {
		lockfiles = make(map[string]*lockfile)
	}
	cleanPath := filepath.Clean(path)
	if locker, ok := lockfiles[cleanPath]; ok {
		if !locker.IsReadWrite() {
			return nil, errors.Wrapf(ErrLockReadOnly, "lock %q is a read-only lock", cleanPath)
		}
		return locker, nil
	}
	fd, err := unix.Open(cleanPath, os.O_RDWR|os.O_CREATE, unix.S_IRUSR|unix.S_IWUSR)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening %q", cleanPath)
	}
	unix.CloseOnExec(fd)
	locker := &lockfile{file: path, fd: uintptr(fd), lw: stringid.GenerateRandomID(), locktype: unix.F_WRLCK}
	lockfiles[filepath.Clean(path)] = locker
	return locker, nil
}

// GetROLockfile opens a read-only lock file.  The Locker object it returns
// will be returned unlocked.
func GetROLockfile(path string) (Locker, error) {
	lockfilesLock.Lock()
	defer lockfilesLock.Unlock()
	if lockfiles == nil {
		lockfiles = make(map[string]*lockfile)
	}
	cleanPath := filepath.Clean(path)
	if locker, ok := lockfiles[cleanPath]; ok {
		if locker.IsReadWrite() {
			return nil, fmt.Errorf("lock %q is a read-write lock", cleanPath)
		}
		return locker, nil
	}
	fd, err := unix.Open(cleanPath, os.O_RDONLY, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening %q", cleanPath)
	}
	unix.CloseOnExec(fd)
	locker := &lockfile{file: path, fd: uintptr(fd), lw: stringid.GenerateRandomID(), locktype: unix.F_RDLCK}
	lockfiles[filepath.Clean(path)] = locker
	return locker, nil
}

// Lock locks the lock file
func (l *lockfile) Lock() {
	lk := unix.Flock_t{
		Type:   l.locktype,
		Whence: int16(os.SEEK_SET),
		Start:  0,
		Len:    0,
		Pid:    int32(os.Getpid()),
	}
	l.mu.Lock()
	for unix.FcntlFlock(l.fd, unix.F_SETLKW, &lk) != nil {
		time.Sleep(10 * time.Millisecond)
	}
}

// Unlock unlocks the lock file
func (l *lockfile) Unlock() {
	lk := unix.Flock_t{
		Type:   unix.F_UNLCK,
		Whence: int16(os.SEEK_SET),
		Start:  0,
		Len:    0,
		Pid:    int32(os.Getpid()),
	}
	for unix.FcntlFlock(l.fd, unix.F_SETLKW, &lk) != nil {
		time.Sleep(10 * time.Millisecond)
	}
	l.mu.Unlock()
}

// Touch updates the lock file with the UID of the user
func (l *lockfile) Touch() error {
	l.lw = stringid.GenerateRandomID()
	id := []byte(l.lw)
	_, err := unix.Seek(int(l.fd), 0, os.SEEK_SET)
	if err != nil {
		return err
	}
	n, err := unix.Write(int(l.fd), id)
	if err != nil {
		return err
	}
	if n != len(id) {
		return unix.ENOSPC
	}
	err = unix.Fsync(int(l.fd))
	if err != nil {
		return err
	}
	return nil
}

// Modified indicates if the lock file has been updated since the last time it was loaded
func (l *lockfile) Modified() (bool, error) {
	id := []byte(l.lw)
	_, err := unix.Seek(int(l.fd), 0, os.SEEK_SET)
	if err != nil {
		return true, err
	}
	n, err := unix.Read(int(l.fd), id)
	if err != nil {
		return true, err
	}
	if n != len(id) {
		return true, unix.ENOSPC
	}
	lw := l.lw
	l.lw = string(id)
	return l.lw != lw, nil
}

// TouchedSince indicates if the lock file has been touched since the specified time
func (l *lockfile) TouchedSince(when time.Time) bool {
	st := unix.Stat_t{}
	err := unix.Fstat(int(l.fd), &st)
	if err != nil {
		return true
	}
	touched := time.Unix(statTMtimeUnix(st))
	return when.Before(touched)
}

// IsRWLock indicates if the lock file is a read-write lock
func (l *lockfile) IsReadWrite() bool {
	return (l.locktype == unix.F_WRLCK)
}
