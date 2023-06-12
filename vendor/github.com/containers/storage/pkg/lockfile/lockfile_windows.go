//go:build windows
// +build windows

package lockfile

import (
	"os"
	"sync"
	"time"
)

// createLockFileForPath returns a *LockFile object, possibly (depending on the platform)
// working inter-process and associated with the specified path.
//
// This function will be called at most once for each path value within a single process.
//
// If ro, the lock is a read-write lock and the returned *LockFile should correspond to the
// “lock for reading” (shared) operation; otherwise, the lock is either an exclusive lock,
// or a read-write lock and *LockFile should correspond to the “lock for writing” (exclusive) operation.
//
// WARNING:
// - The lock may or MAY NOT be inter-process.
// - There may or MAY NOT be an actual object on the filesystem created for the specified path.
// - Even if ro, the lock MAY be exclusive.
func createLockFileForPath(path string, ro bool) (*LockFile, error) {
	return &LockFile{locked: false}, nil
}

// *LockFile represents a file lock where the file is used to cache an
// identifier of the last party that made changes to whatever's being protected
// by the lock.
//
// It MUST NOT be created manually. Use GetLockFile or GetROLockFile instead.
type LockFile struct {
	mu     sync.Mutex
	file   string
	locked bool
}

// LastWrite is an opaque identifier of the last write to some *LockFile.
// It can be used by users of a *LockFile to determine if the lock indicates changes
// since the last check.
// A default-initialized LastWrite never matches any last write, i.e. it always indicates changes.
type LastWrite struct {
	// Nothing: The Windows “implementation” does not actually track writes.
}

func (l *LockFile) Lock() {
	l.mu.Lock()
	l.locked = true
}

func (l *LockFile) RLock() {
	l.mu.Lock()
	l.locked = true
}

func (l *LockFile) Unlock() {
	l.locked = false
	l.mu.Unlock()
}

func (l *LockFile) AssertLocked() {
	// DO NOT provide a variant that returns the value of l.locked.
	//
	// If the caller does not hold the lock, l.locked might nevertheless be true because another goroutine does hold it, and
	// we can’t tell the difference.
	//
	// Hence, this “AssertLocked” method, which exists only for sanity checks.
	if !l.locked {
		panic("internal error: lock is not held by the expected owner")
	}
}

func (l *LockFile) AssertLockedForWriting() {
	// DO NOT provide a variant that returns the current lock state.
	//
	// The same caveats as for AssertLocked apply equally.
	l.AssertLocked() // The current implementation does not distinguish between read and write locks.
}

// GetLastWrite() returns a LastWrite value corresponding to current state of the lock.
// This is typically called before (_not after_) loading the state when initializing a consumer
// of the data protected by the lock.
// During the lifetime of the consumer, the consumer should usually call ModifiedSince instead.
//
// The caller must hold the lock (for reading or writing) before this function is called.
func (l *LockFile) GetLastWrite() (LastWrite, error) {
	l.AssertLocked()
	return LastWrite{}, nil
}

// RecordWrite updates the lock with a new LastWrite value, and returns the new value.
//
// If this function fails, the LastWriter value of the lock is indeterminate;
// the caller should keep using the previously-recorded LastWrite value,
// and possibly detecting its own modification as an external one:
//
//	lw, err := state.lock.RecordWrite()
//	if err != nil { /* fail */ }
//	state.lastWrite = lw
//
// The caller must hold the lock for writing.
func (l *LockFile) RecordWrite() (LastWrite, error) {
	return LastWrite{}, nil
}

// ModifiedSince checks if the lock has been changed since a provided LastWrite value,
// and returns the one to record instead.
//
// If ModifiedSince reports no modification, the previous LastWrite value
// is still valid and can continue to be used.
//
// If this function fails, the LastWriter value of the lock is indeterminate;
// the caller should fail and keep using the previously-recorded LastWrite value,
// so that it continues failing until the situation is resolved. Similarly,
// it should only update the recorded LastWrite value after processing the update:
//
//	lw2, modified, err := state.lock.ModifiedSince(state.lastWrite)
//	if err != nil { /* fail */ }
//	state.lastWrite = lw2
//	if modified {
//		if err := reload(); err != nil { /* fail */ }
//		state.lastWrite = lw2
//	}
//
// The caller must hold the lock (for reading or writing).
func (l *LockFile) ModifiedSince(previous LastWrite) (LastWrite, bool, error) {
	return LastWrite{}, false, nil
}

// Deprecated: Use *LockFile.ModifiedSince.
func (l *LockFile) Modified() (bool, error) {
	return false, nil
}

// Deprecated: Use *LockFile.RecordWrite.
func (l *LockFile) Touch() error {
	return nil
}

func (l *LockFile) IsReadWrite() bool {
	return false
}

func (l *LockFile) TouchedSince(when time.Time) bool {
	stat, err := os.Stat(l.file)
	if err != nil {
		return true
	}
	return when.Before(stat.ModTime())
}
