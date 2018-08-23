// +build linux

package lock

import (
	"github.com/containers/libpod/libpod/lock/shm"
)

// SHMLockManager manages shared memory locks.
type SHMLockManager struct {
	locks *shm.SHMLocks
}

// NewSHMLockManager makes a new SHMLockManager with the given number of locks.
func NewSHMLockManager(path string, numLocks uint32) (Manager, error) {
	locks, err := shm.CreateSHMLock(path, numLocks)
	if err != nil {
		return nil, err
	}

	manager := new(SHMLockManager)
	manager.locks = locks

	return manager, nil
}

// OpenSHMLockManager opens an existing SHMLockManager with the given number of
// locks.
func OpenSHMLockManager(path string, numLocks uint32) (Manager, error) {
	locks, err := shm.OpenSHMLock(path, numLocks)
	if err != nil {
		return nil, err
	}

	manager := new(SHMLockManager)
	manager.locks = locks

	return manager, nil
}

// AllocateLock allocates a new lock from the manager.
func (m *SHMLockManager) AllocateLock() (Locker, error) {
	semIndex, err := m.locks.AllocateSemaphore()
	if err != nil {
		return nil, err
	}

	lock := new(SHMLock)
	lock.lockID = semIndex
	lock.manager = m

	return lock, nil
}

// RetrieveLock retrieves a lock from the manager given its ID.
func (m *SHMLockManager) RetrieveLock(id uint32) (Locker, error) {
	lock := new(SHMLock)
	lock.lockID = id
	lock.manager = m

	return lock, nil
}

// SHMLock is an individual shared memory lock.
type SHMLock struct {
	lockID  uint32
	manager *SHMLockManager
}

// ID returns the ID of the lock.
func (l *SHMLock) ID() uint32 {
	return l.lockID
}

// Lock acquires the lock.
func (l *SHMLock) Lock() {
	if err := l.manager.locks.LockSemaphore(l.lockID); err != nil {
		panic(err.Error())
	}
}

// Unlock releases the lock.
func (l *SHMLock) Unlock() {
	if err := l.manager.locks.UnlockSemaphore(l.lockID); err != nil {
		panic(err.Error())
	}
}

// Free releases the lock, allowing it to be reused.
func (l *SHMLock) Free() error {
	return l.manager.locks.DeallocateSemaphore(l.lockID)
}
