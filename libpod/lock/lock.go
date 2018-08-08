package lock

// LockManager provides an interface for allocating multiprocess locks.
// Locks returned by LockManager MUST be multiprocess - allocating a lock in
// process A and retrieving that lock's ID in process B must return handles for
// the same lock, and locking the lock in A should exclude B from the lock until
// it is unlocked in A.
// All locks must be identified by a UUID (retrieved with Locker's ID() method).
// All locks with a given UUID must refer to the same underlying lock, and it
// must be possible to retrieve the lock given its UUID.
// Each UUID should refer to a unique underlying lock.
// Calls to AllocateLock() must return a unique, unallocated UUID.
// AllocateLock() must fail once all available locks have been allocated.
// Locks are returned to use by calls to Free(), and can subsequently be
// reallocated.
type LockManager interface {
	// AllocateLock returns an unallocated lock.
	// It is guaranteed that the same lock will not be returned again by
	// AllocateLock until the returned lock has Free() called on it.
	// If all available locks are allocated, AllocateLock will return an
	// error.
	AllocateLock() (Locker, error)
	// RetrieveLock retrieves a lock given its UUID.
	// The underlying lock MUST be the same as another other lock with the
	// same UUID.
	RetrieveLock(id string) (Locker, error)
}

// Locker is similar to sync.Locker, but provides a method for freeing the lock
// to allow its reuse.
// All Locker implementations must maintain mutex semantics - the lock only
// allows one caller in the critical section at a time.
// All locks with the same ID must refer to the same underlying lock, even
// if they are within multiple processes.
type Locker interface {
	// ID retrieves the lock's ID.
	// ID is guaranteed to uniquely identify the lock within the
	// LockManager - that is, calling RetrieveLock with this ID will return
	// another instance of the same lock.
	ID() string
	// Lock locks the lock.
	// This call MUST block until it successfully acquires the lock or
	// encounters a fatal error.
	Lock() error
	// Unlock unlocks the lock.
	// A call to Unlock() on a lock that is already unlocked lock MUST
	// error.
	Unlock() error
	// Deallocate deallocates the underlying lock, allowing its reuse by
	// other pods and containers.
	// The lock MUST still be usable after a Free() - some libpod instances
	// may still retain Container structs with the old lock. This simply
	// advises the manager that the lock may be reallocated.
	Free() error
}
