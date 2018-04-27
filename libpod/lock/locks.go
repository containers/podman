package lock

// #cgo LDFLAGS: -lrt -lpthread
// #include "shm_lock.h"
// const uint32_t bitmap_size_c = BITMAP_SIZE;
import "C"

import (
	"syscall"

	"github.com/pkg/errors"
)

var (
	bitmapSize uint32 = uint32(C.bitmap_size_c)
)

// SHMLocks is a struct enabling POSIX semaphore locking in a shared memory
// segment
type SHMLocks struct {
	lockStruct *C.shm_struct_t
	valid      bool
	maxLocks   uint32
}

// CreateSHMLock sets up a shared-memory segment holding a given number of POSIX
// semaphores, and returns a struct that can be used to operate on those locks.
// numLocks must be a multiple of the lock bitmap size (by default, 32).
func CreateSHMLock(numLocks uint32) (*SHMLocks, error) {
	if numLocks % bitmapSize != 0 {
		return nil, errors.Wrapf(syscall.EINVAL, "number of locks must be a multiple of %d", C.bitmap_size_c)
	}

	locks := new(SHMLocks)

	lockStruct := C.setup_lock_shm(C.uint32_t(numLocks))
	if lockStruct == nil {
		// We got a null pointer, so something errored
		return nil, errors.Wrapf(syscall.ENOENT, "error creating shared memory locks")
	}

	locks.lockStruct = lockStruct
	locks.maxLocks = numLocks
	locks.valid = true

	return locks, nil
}

// OpenSHMLock opens an existing shared-memory segment holding a given number of
// POSIX semaphores. numLocks must match the number of locks the shared memory
// segment was created with and be a multiple of the lock bitmap size (default
// 32)
func OpenSHMLock(numLocks uint32) (*SHMLocks, error) {
	if numLocks % bitmapSize != 0 {
		return nil, errors.Wrapf(syscall.EINVAL, "number of locks must be a multiple of %d", C.bitmap_size_c)
	}

	locks := new(SHMLocks)

	lockStruct := C.open_lock_shm(C.uint32_t(numLocks))
	if lockStruct == nil {
		// We got a null pointer, so something errored
		return nil, errors.Wrapf(syscall.ENOENT, "error creating shared memory locks")
	}

	locks.lockStruct = lockStruct
	locks.maxLocks = numLocks
	locks.valid = true

	return locks, nil
}

// Close closes an existing shared-memory segment.
// The segment will be rendered unusable after closing.
// WARNING: If you Close() while there are still locks locked, these locks may
// fail to release, causing a program freeze.
// Close() is only intended to be used while testing the locks.
func (locks *SHMLocks) Close() error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	locks.valid = false

	retCode := C.close_lock_shm(locks.lockStruct)
	if retCode < 0 {
		// Negative errno returned
		return syscall.Errno(-1 * retCode)
	}

	return nil
}

// AllocateSemaphore allocates a semaphore from a shared-memory segment for use
// by a container or pod.
// Returns the index of the semaphore that was allocated
func (locks *SHMLocks) AllocateSemaphore() (uint32, error) {
	if !locks.valid {
		return 0, errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	retCode := C.allocate_semaphore(locks.lockStruct)
	if retCode < 0 {
		// Negative errno returned
		return 0, syscall.Errno(-1 * retCode)
	}

	return uint32(retCode), nil
}

// DeallocateSemaphore frees a semaphore in a shared-memory segment for use by
// a container of pod
func (locks *SHMLocks) DeallocateSemaphore(sem uint32) error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	if sem > locks.maxLocks {
		return errors.Wrapf(syscall.EINVAL, "given semaphore %d is higher than maximum locks count %d", sem, locks.maxLocks)
	}

	retCode := C.deallocate_semaphore(locks.lockStruct, C.uint32_t(sem))
	if retCode < 0 {
		// Negative errno returned
		return syscall.Errno(-1 * retCode)
	}

	return nil
}

// LockSemaphore locks the given semaphore
func (locks *SHMLocks) LockSemaphore(sem uint32) error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	if sem > locks.maxLocks {
		return errors.Wrapf(syscall.EINVAL, "given semaphore %d is higher than maximum locks count %d", sem, locks.maxLocks)
	}

	retCode := C.lock_semaphore(locks.lockStruct, C.uint32_t(sem))
	if retCode < 0 {
		// Negative errno returned
		return syscall.Errno(-1 * retCode)
	}

	return nil
}

// UnlockSemaphore locks the given semaphore
func (locks *SHMLocks) UnlockSemaphore(sem uint32) error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	if sem > locks.maxLocks {
		return errors.Wrapf(syscall.EINVAL, "given semaphore %d is higher than maximum locks count %d", sem, locks.maxLocks)
	}

	retCode := C.unlock_semaphore(locks.lockStruct, C.uint32_t(sem))
	if retCode < 0 {
		// Negative errno returned
		return syscall.Errno(-1 * retCode)
	}

	return nil
}
