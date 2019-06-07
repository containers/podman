// +build windows

package storage

import (
	"os"
	"sync"
	"time"
)

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
	return &lockfile{locked: false}, nil
}

type lockfile struct {
	mu     sync.Mutex
	file   string
	locked bool
}

func (l *lockfile) Lock() {
	l.mu.Lock()
	l.locked = true
}

func (l *lockfile) RecursiveLock() {
	// We don't support Windows but a recursive writer-lock in one process-space
	// is really a writer lock, so just panic.
	panic("not supported")
}

func (l *lockfile) RLock() {
	l.mu.Lock()
	l.locked = true
}

func (l *lockfile) Unlock() {
	l.locked = false
	l.mu.Unlock()
}

func (l *lockfile) Locked() bool {
	return l.locked
}

func (l *lockfile) Modified() (bool, error) {
	return false, nil
}
func (l *lockfile) Touch() error {
	return nil
}
func (l *lockfile) IsReadWrite() bool {
	return false
}

func (l *lockfile) TouchedSince(when time.Time) bool {
	stat, err := os.Stat(l.file)
	if err != nil {
		return true
	}
	return when.Before(stat.ModTime())
}
