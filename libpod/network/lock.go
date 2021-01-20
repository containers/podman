package network

import (
	"os"
	"path/filepath"

	"github.com/containers/common/pkg/config"
	"github.com/containers/storage"
)

// acquireCNILock gets a lock that should be used in create and
// delete cases to avoid unwanted collisions in network names.
// TODO this uses a file lock and should be converted to shared memory
// when we have a more general shared memory lock in libpod
func acquireCNILock(config *config.Config) (*CNILock, error) {
	cniDir := GetCNIConfDir(config)
	err := os.MkdirAll(cniDir, 0755)
	if err != nil {
		return nil, err
	}
	l, err := storage.GetLockfile(filepath.Join(cniDir, LockFileName))
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
