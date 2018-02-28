// +build linux solaris darwin freebsd

package storage

import (
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
	if ro {
		return &lockfile{file: path, fd: uintptr(fd), lw: stringid.GenerateRandomID(), locktype: unix.F_RDLCK}, nil
	}
	return &lockfile{file: path, fd: uintptr(fd), lw: stringid.GenerateRandomID(), locktype: unix.F_WRLCK}, nil
}

type lockfile struct {
	mu       sync.Mutex
	file     string
	fd       uintptr
	lw       string
	locktype int16
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

// IsRWLock indicates if the lock file is a read-write lock
func (l *lockfile) IsReadWrite() bool {
	return (l.locktype == unix.F_WRLCK)
}
