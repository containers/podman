package libpod

import (
	"path/filepath"

	"github.com/containers/storage"
	"github.com/pkg/errors"
)

// RenumberLocks reassigns lock numbers for all containers, pods, and volumes in
// the state.
// It renders the runtime it is called on, and all container/pod/volume structs
// from that runtime, unusable, and requires that a new runtime be initialized
// after it is called.
func (r *Runtime) RenumberLocks() error {
	r.lock.Lock()
	locked := true
	defer func() {
		if locked {
			r.lock.Unlock()
		}
	}()

	runtimeAliveLock := filepath.Join(r.config.TmpDir, "alive.lck")
	aliveLock, err := storage.GetLockfile(runtimeAliveLock)
	if err != nil {
		return errors.Wrapf(err, "error acquiring runtime init lock")
	}
	aliveLock.Lock()
	// It's OK to defer until Shutdown() has run, so no need to check locked
	defer aliveLock.Unlock()

	// Start off by deallocating all locks
	if err := r.lockManager.FreeAllLocks(); err != nil {
		return err
	}

	allCtrs, err := r.state.AllContainers()
	if err != nil {
		return err
	}
	for _, ctr := range allCtrs {
		lock, err := r.lockManager.AllocateLock()
		if err != nil {
			return errors.Wrapf(err, "error allocating lock for container %s", ctr.ID())
		}

		ctr.config.LockID = lock.ID()

		// Write the new lock ID
		if err := r.state.RewriteContainerConfig(ctr, ctr.config); err != nil {
			return err
		}
	}
	allPods, err := r.state.AllPods()
	if err != nil {
		return err
	}
	for _, pod := range allPods {
		lock, err := r.lockManager.AllocateLock()
		if err != nil {
			return errors.Wrapf(err, "error allocating lock for pod %s", pod.ID())
		}

		pod.config.LockID = lock.ID()

		// Write the new lock ID
		if err := r.state.RewritePodConfig(pod, pod.config); err != nil {
			return err
		}
	}

	r.lock.Unlock()
	locked = false

	return r.Shutdown(false)
}
