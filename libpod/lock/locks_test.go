package lock

import (
	"fmt"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// All tests here are in the same process, which somewhat limits their utility
// The big intent of this package it multiprocess locking, which is really hard
// to test without actually having multiple processes...
// We can at least verify that the locks work within the local process.

// 4 * BITMAP_SIZE to ensure we have to traverse bitmaps
const numLocks = 128

// We need a test main to ensure that the SHM is created before the tests run
func TestMain(m *testing.M) {
	shmLock, err := CreateSHMLock(numLocks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating SHM for tests: %v\n", err)
		os.Exit(-1)
	}

	// Close the SHM - every subsequent test will reopen
	if err := shmLock.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Error closing SHM locks: %v\n", err)
		os.Exit(-1)
	}

	exitCode := m.Run()

	// We need to remove the SHM segment to clean up after ourselves
	os.RemoveAll("/dev/shm/libpod_lock")

	os.Exit(exitCode)
}


func runLockTest(t *testing.T, testFunc func(*testing.T, *SHMLocks)) {
	locks, err := OpenSHMLock(numLocks)
	if err != nil {
		t.Fatalf("Error opening locks: %v", err)
	}
	defer func() {
		// Unlock and deallocate all locks
		// Ignore EBUSY (lock is already unlocked)
		// Ignore ENOENT (lock is not allocated)
		var i uint32
		for i = 0; i < numLocks; i++ {
			if err := locks.UnlockSemaphore(i); err != nil && err != syscall.EBUSY {
				t.Fatalf("Error unlocking semaphore %d: %v", i, err)
			}
			if err := locks.DeallocateSemaphore(i); err != nil && err != syscall.ENOENT {
				t.Fatalf("Error deallocating semaphore %d: %v", i, err)
			}
		}

		if err := locks.Close(); err != nil {
			t.Fatalf("Error closing locks: %v", err)
		}
	}()

	success := t.Run("locks", func (t *testing.T) {
		testFunc(t, locks)
	})
	if !success {
		t.Fail()
	}
}

// Test that creating an SHM with a bad size fails
func TestCreateNewSHMBadSize(t *testing.T) {
	// Odd number, not a power of 2, should never be a word size on a system
	_, err := CreateSHMLock(7)
	assert.Error(t, err)
}

// Test allocating - lock - unlock - deallocate cycle, single lock
func TestLockLifecycleSingleLock(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		sem, err := locks.AllocateSemaphore()
		require.NoError(t, err)

		err = locks.LockSemaphore(sem)
		assert.NoError(t, err)

		err = locks.UnlockSemaphore(sem)
		assert.NoError(t, err)

		err = locks.DeallocateSemaphore(sem)
		assert.NoError(t, err)
	})
}

// Test allocate two locks returns different locks
func TestAllocateTwoLocksGetsDifferentLocks(t *testing.T) {
	runLockTest(t, func(t *testing.T, locks *SHMLocks) {
		sem1, err := locks.AllocateSemaphore()
		assert.NoError(t, err)

		sem2, err := locks.AllocateSemaphore()
		assert.NoError(t, err)

		assert.NotEqual(t, sem1, sem2)
	})
}
