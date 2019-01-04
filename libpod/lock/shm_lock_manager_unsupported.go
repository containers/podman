// +build !linux

package lock

import "fmt"

// SHMLockManager is a shared memory lock manager.
// It is not supported on non-Unix platforms.
type SHMLockManager struct{}

// NewSHMLockManager is not supported on this platform
func NewSHMLockManager(numLocks uint32) (Manager, error) {
	return nil, fmt.Errorf("not supported")
}

// OpenSHMLockManager is not supported on this platform
func OpenSHMLockManager(numLocks uint32) (Manager, error) {
	return nil, fmt.Errorf("not supported")
}

// AllocateLock is not supported on this platform
func (m *SHMLockManager) AllocateLock() (Locker, error) {
	return nil, fmt.Errorf("not supported")
}

// RetrieveLock is not supported on this platform
func (m *SHMLockManager) RetrieveLock(id string) (Locker, error) {
	return nil, fmt.Errorf("not supported")
}
