package chanmutex

// mutex based around on channel with buffer size 1.
//
// Locking & unlocking operations are just channel receive/send respectively,
// which allows the caller to lock/unlock the mutex as part of a select.
// We also provide lock/unlock methods for convenience.
type Mutex chan struct{}

// Return a new, locked mutex.
func NewLocked() Mutex {
	return make(Mutex, 1)
}

// Return a new, unlocked mutex
func NewUnlocked() Mutex {
	ret := NewLocked()
	ret.Unlock()
	return ret
}

// Lock the mutex. Blocks if it is already locked.
func (m Mutex) Lock() {
	<-m
}

// Unlock the mutex.
func (m Mutex) Unlock() {
	m <- struct{}{}
}
