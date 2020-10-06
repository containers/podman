package network

import (
	"github.com/containers/storage"
)

// acquireCNILock gets a lock that should be used in create and
// delete cases to avoid unwanted collisions in network names.
// TODO this uses a file lock and should be converted to shared memory
// when we have a more general shared memory lock in libpod
func acquireCNILock(lockPath string) (*CNILock, error) {
	l, err := storage.GetLockfile(lockPath)
	if err != nil {
		return nil, err
	}
	l.Lock()
	cnilock := CNILock{
		Locker: l,
	}
	return &cnilock, nil
}

// ReleaseCNILock unlocks the previously held lock
func (l *CNILock) releaseCNILock() {
	l.Unlock()
}
