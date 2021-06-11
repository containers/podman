// +build linux solaris darwin freebsd

package lockfile

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/containers/storage/pkg/stringid"
	"github.com/containers/storage/pkg/system"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type lockfile struct {
	// rwMutex serializes concurrent reader-writer acquisitions in the same process space
	rwMutex *sync.RWMutex
	// stateMutex is used to synchronize concurrent accesses to the state below
	stateMutex *sync.Mutex
	counter    int64
	file       string
	fd         uintptr
	lw         string
	locktype   int16
	locked     bool
	ro         bool
	recursive  bool
}

// openLock opens the file at path and returns the corresponding file
// descriptor.  Note that the path is opened read-only when ro is set.  If ro
// is unset, openLock will open the path read-write and create the file if
// necessary.
func openLock(path string, ro bool) (int, error) {
	if ro {
		return unix.Open(path, os.O_RDONLY|unix.O_CLOEXEC, 0)
	}
	return unix.Open(path, os.O_RDWR|unix.O_CLOEXEC|os.O_CREATE, unix.S_IRUSR|unix.S_IWUSR|unix.S_IRGRP|unix.S_IROTH)
}

// createLockerForPath returns a Locker object, possibly (depending on the platform)
// working inter-process and associated with the specified path.
//
// This function will be called at most once for each path value within a single process.
//
// If ro, the lock is a read-write lock and the returned Locker should correspond to the
// “lock for reading” (shared) operation; otherwise, the lock is either an exclusive lock,
// or a read-write lock and Locker should correspond to the “lock for writing” (exclusive) operation.
//
// WARNING:
// - The lock may or MAY NOT be inter-process.
// - There may or MAY NOT be an actual object on the filesystem created for the specified path.
// - Even if ro, the lock MAY be exclusive.
func createLockerForPath(path string, ro bool) (Locker, error) {
	// Check if we can open the lock.
	fd, err := openLock(path, ro)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening %q", path)
	}
	unix.Close(fd)

	locktype := unix.F_WRLCK
	if ro {
		locktype = unix.F_RDLCK
	}
	return &lockfile{
		stateMutex: &sync.Mutex{},
		rwMutex:    &sync.RWMutex{},
		file:       path,
		lw:         stringid.GenerateRandomID(),
		locktype:   int16(locktype),
		locked:     false,
		ro:         ro}, nil
}

// lock locks the lockfile via FCTNL(2) based on the specified type and
// command.
func (l *lockfile) lock(lType int16, recursive bool) {
	lk := unix.Flock_t{
		Type:   lType,
		Whence: int16(os.SEEK_SET),
		Start:  0,
		Len:    0,
	}
	switch lType {
	case unix.F_RDLCK:
		l.rwMutex.RLock()
	case unix.F_WRLCK:
		if recursive {
			// NOTE: that's okay as recursive is only set in RecursiveLock(), so
			// there's no need to protect against hypothetical RDLCK cases.
			l.rwMutex.RLock()
		} else {
			l.rwMutex.Lock()
		}
	default:
		panic(fmt.Sprintf("attempted to acquire a file lock of unrecognized type %d", lType))
	}
	l.stateMutex.Lock()
	defer l.stateMutex.Unlock()
	if l.counter == 0 {
		// If we're the first reference on the lock, we need to open the file again.
		fd, err := openLock(l.file, l.ro)
		if err != nil {
			panic(fmt.Sprintf("error opening %q: %v", l.file, err))
		}
		l.fd = uintptr(fd)

		// Optimization: only use the (expensive) fcntl syscall when
		// the counter is 0.  In this case, we're either the first
		// reader lock or a writer lock.
		for unix.FcntlFlock(l.fd, unix.F_SETLKW, &lk) != nil {
			time.Sleep(10 * time.Millisecond)
		}
	}
	l.locktype = lType
	l.locked = true
	l.recursive = recursive
	l.counter++
}

// Lock locks the lockfile as a writer.  Panic if the lock is a read-only one.
func (l *lockfile) Lock() {
	if l.ro {
		panic("can't take write lock on read-only lock file")
	} else {
		l.lock(unix.F_WRLCK, false)
	}
}

// RecursiveLock locks the lockfile as a writer but allows for recursive
// acquisitions within the same process space.  Note that RLock() will be called
// if it's a lockTypReader lock.
func (l *lockfile) RecursiveLock() {
	if l.ro {
		l.RLock()
	} else {
		l.lock(unix.F_WRLCK, true)
	}
}

// LockRead locks the lockfile as a reader.
func (l *lockfile) RLock() {
	l.lock(unix.F_RDLCK, false)
}

// Unlock unlocks the lockfile.
func (l *lockfile) Unlock() {
	l.stateMutex.Lock()
	if l.locked == false {
		// Panic when unlocking an unlocked lock.  That's a violation
		// of the lock semantics and will reveal such.
		panic("calling Unlock on unlocked lock")
	}
	l.counter--
	if l.counter < 0 {
		// Panic when the counter is negative.  There is no way we can
		// recover from a corrupted lock and we need to protect the
		// storage from corruption.
		panic(fmt.Sprintf("lock %q has been unlocked too often", l.file))
	}
	if l.counter == 0 {
		// We should only release the lock when the counter is 0 to
		// avoid releasing read-locks too early; a given process may
		// acquire a read lock multiple times.
		l.locked = false
		// Close the file descriptor on the last unlock, releasing the
		// file lock.
		unix.Close(int(l.fd))
	}
	if l.locktype == unix.F_RDLCK || l.recursive {
		l.rwMutex.RUnlock()
	} else {
		l.rwMutex.Unlock()
	}
	l.stateMutex.Unlock()
}

// Locked checks if lockfile is locked for writing by a thread in this process.
func (l *lockfile) Locked() bool {
	l.stateMutex.Lock()
	defer l.stateMutex.Unlock()
	return l.locked && (l.locktype == unix.F_WRLCK)
}

// Touch updates the lock file with the UID of the user.
func (l *lockfile) Touch() error {
	l.stateMutex.Lock()
	if !l.locked || (l.locktype != unix.F_WRLCK) {
		panic("attempted to update last-writer in lockfile without the write lock")
	}
	defer l.stateMutex.Unlock()
	l.lw = stringid.GenerateRandomID()
	id := []byte(l.lw)
	n, err := unix.Pwrite(int(l.fd), id, 0)
	if err != nil {
		return err
	}
	if n != len(id) {
		return unix.ENOSPC
	}
	return nil
}

// Modified indicates if the lockfile has been updated since the last time it
// was loaded.
func (l *lockfile) Modified() (bool, error) {
	l.stateMutex.Lock()
	id := []byte(l.lw)
	if !l.locked {
		panic("attempted to check last-writer in lockfile without locking it first")
	}
	defer l.stateMutex.Unlock()
	n, err := unix.Pread(int(l.fd), id, 0)
	if err != nil {
		return true, err
	}
	if n != len(id) {
		return true, nil
	}
	lw := l.lw
	l.lw = string(id)
	return l.lw != lw, nil
}

// IsReadWriteLock indicates if the lock file is a read-write lock.
func (l *lockfile) IsReadWrite() bool {
	return !l.ro
}

// TouchedSince indicates if the lock file has been touched since the specified time
func (l *lockfile) TouchedSince(when time.Time) bool {
	st, err := system.Fstat(int(l.fd))
	if err != nil {
		return true
	}
	mtim := st.Mtim()
	touched := time.Unix(mtim.Unix())
	return when.Before(touched)
}
