package lock

import (
	"fmt"
	"math"
	"strconv"
	"syscall"

	"github.com/pkg/errors"
)

// SHMLockManager manages shared memory locks.
type SHMLockManager struct {
	locks *SHMLocks
}

// NewSHMLockManager makes a new SHMLockManager with the given number of locks.
func NewSHMLockManager(numLocks uint32) (Manager, error) {
	locks, err := CreateSHMLock(numLocks)
	if err != nil {
		return nil, err
	}

	manager := new(SHMLockManager)
	manager.locks = locks

	return manager, nil
}

// OpenSHMLockManager opens an existing SHMLockManager with the given number of
// locks.
func OpenSHMLockManager(numLocks uint32) (LockManager, error) {
	locks, err := OpenSHMLock(numLocks)
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
func (m *SHMLockManager) RetrieveLock(id string) (Locker, error) {
	intID, err := strconv.ParseInt(id, 16, 64)
	if err != nil {
		return errors.Wrapf(err, "given ID %q is not a valid SHMLockManager ID - cannot be parsed as int", id)
	}

	if intID < 0 {
		return errors.Wrapf(syscall.EINVAL, "given ID %q is not a valid SHMLockManager ID - must be positive", id)
	}

	if intID > math.MaxUint32 {
		return errors.Wrapf(syscall.EINVAL, "given ID %q is not a valid SHMLockManager ID - too large", id)
	}

	var u32ID uint32 = uint32(intID)
	if u32ID >= m.locks.maxLocks {
		return errors.Wrapf(syscall.EINVAL, "given ID %q is not a valid SHMLockManager ID - too large to fit", id)
	}

	lock := new(SHMLock)
	lock.lockID = u32ID
	lock.manager = m

	return lock, nil
}

// SHMLock is an individual shared memory lock.
type SHMLock struct {
	lockID  uint32
	manager *SHMLockManager
}

// ID returns the ID of the lock.
func (l *SHMLock) ID() string {
	return fmt.Sprintf("%x", l.lockID)
}

// Lock acquires the lock.
func (l *SHMLock) Lock() error {
	return l.manager.locks.LockSemaphore(l.lockID)
}

// Unlock releases the lock.
func (l *SHMLock) Unlock() error {
	return l.manager.locks.UnlockSemaphore(l.lockID)
}

// Free releases the lock, allowing it to be reused.
func (l *SHMLock) Free() error {
	return l.manager.locks.DeallocateSemaphore(l.lockID)
}
