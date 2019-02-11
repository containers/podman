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

func getLockFile(path string, ro bool) (Locker, error) {
	var fd int
	var err error
	if ro {
		fd, err = unix.Open(path, os.O_RDONLY, 0)
	} else {
		fd, err = unix.Open(path, os.O_RDWR|os.O_CREATE, unix.S_IRUSR|unix.S_IWUSR)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "error opening %q", path)
	}
	unix.CloseOnExec(fd)

	locktype := unix.F_WRLCK
	if ro {
		locktype = unix.F_RDLCK
	}
	return &lockfile{
		stateMutex: &sync.Mutex{},
		writeMutex: &sync.Mutex{},
		file:       path,
		fd:         uintptr(fd),
		lw:         stringid.GenerateRandomID(),
		locktype:   int16(locktype),
		locked:     false,
		ro:         ro}, nil
}

type lockfile struct {
	// stateMutex is used to synchronize concurrent accesses
	stateMutex *sync.Mutex
	// writeMutex is used to serialize and avoid recursive writer locks
	writeMutex *sync.Mutex
	counter    int64
	file       string
	fd         uintptr
	lw         string
	locktype   int16
	locked     bool
	ro         bool
}

// lock locks the lockfile via FCTNL(2) based on the specified type and
// command.
func (l *lockfile) lock(l_type int16, l_cmd int) {
	lk := unix.Flock_t{
		Type:   l_type,
		Whence: int16(os.SEEK_SET),
		Start:  0,
		Len:    0,
		Pid:    int32(os.Getpid()),
	}
	if l_type == unix.F_WRLCK {
		// If we try to lock as a writer, lock the writerMutex first to
		// avoid multiple writer acquisitions of the same process.
		// Note: it's important to lock it prior to the stateMutex to
		// avoid a deadlock.
		l.writeMutex.Lock()
	}
	l.stateMutex.Lock()
	l.locktype = l_type
	if l.counter == 0 {
		// Optimization: only use the (expensive) fcntl syscall when
		// the counter is 0.  If it's greater than that, we're owning
		// the lock already and can only be a reader.
		for unix.FcntlFlock(l.fd, unix.F_SETLKW, &lk) != nil {
			time.Sleep(10 * time.Millisecond)
		}
	}
	l.locked = true
	l.counter++
	l.stateMutex.Unlock()
}

// Lock locks the lockfile as a writer.  Note that RLock() will be called if
// the lock is a read-only one.
func (l *lockfile) Lock() {
	if l.ro {
		l.RLock()
	} else {
		l.lock(unix.F_WRLCK, unix.F_SETLKW)
	}
}

// LockRead locks the lockfile as a reader.
func (l *lockfile) RLock() {
	l.lock(unix.F_RDLCK, unix.F_SETLK)
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
		// Panic when unlocking and unlocked lock.  That's a vioalation
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
	}
	if l.locktype == unix.F_WRLCK {
		l.writeMutex.Unlock()
	}
	l.stateMutex.Unlock()
}

// Locked checks if lockfile is locked.
func (l *lockfile) Locked() bool {
	return l.locked
}

// Touch updates the lock file with the UID of the user.
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

// Modified indicates if the lockfile has been updated since the last time it
// was loaded.
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

// IsReadWriteLock indicates if the lock file is a read-write lock.
func (l *lockfile) IsReadWrite() bool {
	return !l.ro
}
