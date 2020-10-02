package rootless

import (
	"os"
	"sync"

	"github.com/containers/storage"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/pkg/errors"
)

// TryJoinPauseProcess attempts to join the namespaces of the pause PID via
// TryJoinFromFilePaths.  If joining fails, it attempts to delete the specified
// file.
func TryJoinPauseProcess(pausePidPath string) (bool, int, error) {
	if _, err := os.Stat(pausePidPath); err != nil {
		return false, -1, nil
	}

	became, ret, err := TryJoinFromFilePaths("", false, []string{pausePidPath})
	if err == nil {
		return became, ret, err
	}

	// It could not join the pause process, let's lock the file before trying to delete it.
	pidFileLock, err := storage.GetLockfile(pausePidPath)
	if err != nil {
		// The file was deleted by another process.
		if os.IsNotExist(err) {
			return false, -1, nil
		}
		return false, -1, errors.Wrapf(err, "error acquiring lock on %s", pausePidPath)
	}

	pidFileLock.Lock()
	defer func() {
		if pidFileLock.Locked() {
			pidFileLock.Unlock()
		}
	}()

	// Now the pause PID file is locked.  Try to join once again in case it changed while it was not locked.
	became, ret, err = TryJoinFromFilePaths("", false, []string{pausePidPath})
	if err != nil {
		// It is still failing.  We can safely remove it.
		os.Remove(pausePidPath)
		return false, -1, nil
	}
	return became, ret, err
}

var (
	availableGids     int64
	availableGidsErr  error
	availableGidsOnce sync.Once
)

// GetAvailableGids returns how many GIDs are available in the
// current user namespace.
func GetAvailableGids() (int64, error) {
	availableGidsOnce.Do(func() {
		idMap, err := user.ParseIDMapFile("/proc/self/gid_map")
		if err != nil {
			availableGidsErr = err
			return
		}
		availableGids = int64(0)
		for _, r := range idMap {
			availableGids += r.Count
		}
	})
	return availableGids, availableGidsErr
}
