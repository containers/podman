// misc. utilities for synchronization.
package syncutil

import (
	"sync"
)

// Runs f while holding the lock
func With(l sync.Locker, f func()) {
	l.Lock()
	defer l.Unlock()
	f()
}

// Runs f while not holding the lock
func Without(l sync.Locker, f func()) {
	l.Unlock()
	defer l.Lock()
	f()
}
