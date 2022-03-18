//go:build linux && cgo
// +build linux,cgo

package shm

// #cgo LDFLAGS: -lrt -lpthread
// #cgo CFLAGS: -Wall -Werror
// #include <stdlib.h>
// #include "shm_lock.h"
// const uint32_t bitmap_size_c = BITMAP_SIZE;
import "C"

import (
	"runtime"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// BitmapSize is the size of the bitmap used when managing SHM locks.
	// an SHM lock manager's max locks will be rounded up to a multiple of
	// this number.
	BitmapSize = uint32(C.bitmap_size_c)
)

// SHMLocks is a struct enabling POSIX semaphore locking in a shared memory
// segment.
type SHMLocks struct { // nolint
	lockStruct *C.shm_struct_t
	maxLocks   uint32
	valid      bool
}

// CreateSHMLock sets up a shared-memory segment holding a given number of POSIX
// semaphores, and returns a struct that can be used to operate on those locks.
// numLocks must not be 0, and may be rounded up to a multiple of the bitmap
// size used by the underlying implementation.
func CreateSHMLock(path string, numLocks uint32) (*SHMLocks, error) {
	if numLocks == 0 {
		return nil, errors.Wrapf(syscall.EINVAL, "number of locks must be greater than 0")
	}

	locks := new(SHMLocks)

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var errCode C.int
	lockStruct := C.setup_lock_shm(cPath, C.uint32_t(numLocks), &errCode)
	if lockStruct == nil {
		// We got a null pointer, so something errored
		return nil, errors.Wrapf(syscall.Errno(-1*errCode), "failed to create %d locks in %s", numLocks, path)
	}

	locks.lockStruct = lockStruct
	locks.maxLocks = uint32(lockStruct.num_locks)
	locks.valid = true

	logrus.Debugf("Initialized SHM lock manager at path %s", path)

	return locks, nil
}

// OpenSHMLock opens an existing shared-memory segment holding a given number of
// POSIX semaphores. numLocks must match the number of locks the shared memory
// segment was created with.
func OpenSHMLock(path string, numLocks uint32) (*SHMLocks, error) {
	if numLocks == 0 {
		return nil, errors.Wrapf(syscall.EINVAL, "number of locks must be greater than 0")
	}

	locks := new(SHMLocks)

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var errCode C.int
	lockStruct := C.open_lock_shm(cPath, C.uint32_t(numLocks), &errCode)
	if lockStruct == nil {
		// We got a null pointer, so something errored
		return nil, errors.Wrapf(syscall.Errno(-1*errCode), "failed to open %d locks in %s", numLocks, path)
	}

	locks.lockStruct = lockStruct
	locks.maxLocks = numLocks
	locks.valid = true

	return locks, nil
}

// GetMaxLocks returns the maximum number of locks in the SHM
func (locks *SHMLocks) GetMaxLocks() uint32 {
	return locks.maxLocks
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
// Returns the index of the semaphore that was allocated.
// Allocations past the maximum number of locks given when the SHM segment was
// created will result in an error, and no semaphore will be allocated.
func (locks *SHMLocks) AllocateSemaphore() (uint32, error) {
	if !locks.valid {
		return 0, errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	// This returns a U64, so we have the full u32 range available for
	// semaphore indexes, and can still return error codes.
	retCode := C.allocate_semaphore(locks.lockStruct)
	if retCode < 0 {
		var err = syscall.Errno(-1 * retCode)
		// Negative errno returned
		if errors.Is(err, syscall.ENOSPC) {
			// ENOSPC expands to "no space left on device".  While it is technically true
			// that there's no room in the SHM inn for this lock, this tends to send normal people
			// down the path of checking disk-space which is not actually their problem.
			// Give a clue that it's actually due to num_locks filling up.
			var errFull = errors.Errorf("allocation failed; exceeded num_locks (%d)", locks.maxLocks)
			return uint32(retCode), errFull
		}
		return uint32(retCode), syscall.Errno(-1 * retCode)
	}

	return uint32(retCode), nil
}

// AllocateGivenSemaphore allocates the given semaphore from the shared-memory
// segment for use by a container or pod.
// If the semaphore is already in use or the index is invalid an error will be
// returned.
func (locks *SHMLocks) AllocateGivenSemaphore(sem uint32) error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	retCode := C.allocate_given_semaphore(locks.lockStruct, C.uint32_t(sem))
	if retCode < 0 {
		return syscall.Errno(-1 * retCode)
	}

	return nil
}

// DeallocateSemaphore frees a semaphore in a shared-memory segment so it can be
// reallocated to another container or pod.
// The given semaphore must be already allocated, or an error will be returned.
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

// DeallocateAllSemaphores frees all semaphores so they can be reallocated to
// other containers and pods.
func (locks *SHMLocks) DeallocateAllSemaphores() error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	retCode := C.deallocate_all_semaphores(locks.lockStruct)
	if retCode < 0 {
		// Negative errno return from C
		return syscall.Errno(-1 * retCode)
	}

	return nil
}

// LockSemaphore locks the given semaphore.
// If the semaphore is already locked, LockSemaphore will block until the lock
// can be acquired.
// There is no requirement that the given semaphore be allocated.
// This ensures that attempts to lock a container after it has been deleted,
// but before the caller has queried the database to determine this, will
// succeed.
func (locks *SHMLocks) LockSemaphore(sem uint32) error {
	if !locks.valid {
		return errors.Wrapf(syscall.EINVAL, "locks have already been closed")
	}

	if sem > locks.maxLocks {
		return errors.Wrapf(syscall.EINVAL, "given semaphore %d is higher than maximum locks count %d", sem, locks.maxLocks)
	}

	// For pthread mutexes, we have to guarantee lock and unlock happen in
	// the same thread.
	runtime.LockOSThread()

	retCode := C.lock_semaphore(locks.lockStruct, C.uint32_t(sem))
	if retCode < 0 {
		// Negative errno returned
		return syscall.Errno(-1 * retCode)
	}

	return nil
}

// UnlockSemaphore unlocks the given semaphore.
// Unlocking a semaphore that is already unlocked with return EBUSY.
// There is no requirement that the given semaphore be allocated.
// This ensures that attempts to lock a container after it has been deleted,
// but before the caller has queried the database to determine this, will
// succeed.
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

	// For pthread mutexes, we have to guarantee lock and unlock happen in
	// the same thread.
	// OK if we take multiple locks - UnlockOSThread() won't actually unlock
	// until the number of calls equals the number of calls to
	// LockOSThread()
	runtime.UnlockOSThread()

	return nil
}
