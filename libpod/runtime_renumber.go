package libpod

import (
	"github.com/containers/podman/v2/libpod/events"
	"github.com/pkg/errors"
)

// renumberLocks reassigns lock numbers for all containers and pods in the
// state.
// TODO: It would be desirable to make it impossible to call this until all
// other libpod sessions are dead.
// Possibly use a read-write file lock, with all non-renumber podmans owning the
// lock as read, renumber attempting to take a write lock?
// The alternative is some sort of session tracking, and I don't know how
// reliable that can be.
func (r *Runtime) renumberLocks() error {
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
	allVols, err := r.state.AllVolumes()
	if err != nil {
		return err
	}
	for _, vol := range allVols {
		lock, err := r.lockManager.AllocateLock()
		if err != nil {
			return errors.Wrapf(err, "error allocating lock for volume %s", vol.Name())
		}

		vol.config.LockID = lock.ID()

		// Write the new lock ID
		if err := r.state.RewriteVolumeConfig(vol, vol.config); err != nil {
			return err
		}
	}

	r.newSystemEvent(events.Renumber)

	return nil
}
