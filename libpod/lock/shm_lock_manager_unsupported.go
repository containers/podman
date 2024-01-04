//go:build !linux

package lock

import "fmt"

// SHMLockManager is a shared memory lock manager.
// It is not supported on non-Unix platforms.
type SHMLockManager struct{}

// NewSHMLockManager is not supported on this platform
func NewSHMLockManager(path string, numLocks uint32) (Manager, error) {
	return nil, fmt.Errorf("not supported")
}

// OpenSHMLockManager is not supported on this platform
func OpenSHMLockManager(path string, numLocks uint32) (Manager, error) {
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

// FreeAllLocks is not supported on this platform
func (m *SHMLockManager) FreeAllLocks() error {
	return fmt.Errorf("not supported")
}

// AvailableLocks is not supported on this platform
func (m *SHMLockManager) AvailableLocks() (*uint32, error) {
	return nil, fmt.Errorf("not supported")
}

// LocksHeld is not supported on this platform
func (m *SHMLockManager) LocksHeld() ([]uint32, error) {
	return nil, fmt.Errorf("not supported")
}
