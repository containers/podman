// +build linux solaris darwin freebsd

package storage

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/containers/storage/pkg/stringid"
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
}

// openLock opens the file at path and returns the corresponding file
// descriptor.  Note that the path is opened read-only when ro is set.  If ro
// is unset, openLock will open the path read-write and create the file if
// necessary.
func openLock(path string, ro bool) (int, error) {
	if ro {
		return unix.Open(path, os.O_RDONLY, 0)
	}
	return unix.Open(path, os.O_RDWR|os.O_CREATE, unix.S_IRUSR|unix.S_IWUSR)
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
func (l *lockfile) lock(l_type int16) {
	lk := unix.Flock_t{
		Type:   l_type,
		Whence: int16(os.SEEK_SET),
		Start:  0,
		Len:    0,
	}
	switch l_type {
	case unix.F_RDLCK:
		l.rwMutex.RLock()
	case unix.F_WRLCK:
		l.rwMutex.Lock()
	default:
		panic(fmt.Sprintf("attempted to acquire a file lock of unrecognized type %d", l_type))
	}
	l.stateMutex.Lock()
	defer l.stateMutex.Unlock()
	if l.counter == 0 {
		// If we're the first reference on the lock, we need to open the file again.
		fd, err := openLock(l.file, l.ro)
		if err != nil {
			panic(fmt.Sprintf("error opening %q", l.file))
		}
		unix.CloseOnExec(fd)
		l.fd = uintptr(fd)

		// Optimization: only use the (expensive) fcntl syscall when
		// the counter is 0.  In this case, we're either the first
		// reader lock or a writer lock.
		for unix.FcntlFlock(l.fd, unix.F_SETLKW, &lk) != nil {
			time.Sleep(10 * time.Millisecond)
		}
	}
	l.locktype = l_type
	l.locked = true
	l.counter++
}

// Lock locks the lockfile as a writer.  Note that RLock() will be called if
// the lock is a read-only one.
func (l *lockfile) Lock() {
	if l.ro {
		l.RLock()
	} else {
		l.lock(unix.F_WRLCK)
	}
}

// LockRead locks the lockfile as a reader.
func (l *lockfile) RLock() {
	l.lock(unix.F_RDLCK)
}

// Unlock unlocks the lockfile.
func (l *lockfile) Unlock() {
	lk := unix.Flock_t{
		Type:   unix.F_UNLCK,
		Whence: int16(os.SEEK_SET),
		Start:  0,
		Len:    0,
		Pid:    int32(os.Getpid()),
	}
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
		for unix.FcntlFlock(l.fd, unix.F_SETLKW, &lk) != nil {
			time.Sleep(10 * time.Millisecond)
		}
		// Close the file descriptor on the last unlock.
		unix.Close(int(l.fd))
	}
	if l.locktype == unix.F_RDLCK {
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
	l.stateMutex.Unlock()
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

// Modified indicates if the lockfile has been updated since the last time it
// was loaded.
func (l *lockfile) Modified() (bool, error) {
	id := []byte(l.lw)
	l.stateMutex.Lock()
	if !l.locked {
		panic("attempted to check last-writer in lockfile without locking it first")
	}
	l.stateMutex.Unlock()
	_, err := unix.Seek(int(l.fd), 0, os.SEEK_SET)
	if err != nil {
		return true, err
	}
	n, err := unix.Read(int(l.fd), id)
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
