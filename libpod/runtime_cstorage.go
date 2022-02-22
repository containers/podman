package libpod

import (
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// StorageContainer represents a container present in c/storage but not in
// libpod.
type StorageContainer struct {
	ID              string
	Names           []string
	Image           string
	CreateTime      time.Time
	PresentInLibpod bool
}

// ListStorageContainers lists all containers visible to c/storage.
func (r *Runtime) ListStorageContainers() ([]*StorageContainer, error) {
	finalCtrs := []*StorageContainer{}

	ctrs, err := r.store.Containers()
	if err != nil {
		return nil, err
	}

	for _, ctr := range ctrs {
		storageCtr := new(StorageContainer)
		storageCtr.ID = ctr.ID
		storageCtr.Names = ctr.Names
		storageCtr.Image = ctr.ImageID
		storageCtr.CreateTime = ctr.Created

		// Look up if container is in state
		hasCtr, err := r.state.HasContainer(ctr.ID)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking up container %s in state", ctr.ID)
		}

		storageCtr.PresentInLibpod = hasCtr

		finalCtrs = append(finalCtrs, storageCtr)
	}

	return finalCtrs, nil
}

func (r *Runtime) StorageContainer(idOrName string) (*storage.Container, error) {
	return r.store.Container(idOrName)
}

// RemoveStorageContainer removes a container from c/storage.
// The container WILL NOT be removed if it exists in libpod.
// Accepts ID or full name of container.
// If force is set, the container will be unmounted first to ensure removal.
func (r *Runtime) RemoveStorageContainer(idOrName string, force bool) error {
	targetID, err := r.store.Lookup(idOrName)
	if err != nil {
		if errors.Cause(err) == storage.ErrLayerUnknown {
			return errors.Wrapf(define.ErrNoSuchCtr, "no container with ID or name %q found", idOrName)
		}
		return errors.Wrapf(err, "error looking up container %q", idOrName)
	}

	// Lookup returns an ID but it's not guaranteed to be a container ID.
	// So we can still error here.
	ctr, err := r.store.Container(targetID)
	if err != nil {
		if errors.Cause(err) == storage.ErrContainerUnknown {
			return errors.Wrapf(define.ErrNoSuchCtr, "%q does not refer to a container", idOrName)
		}
		return errors.Wrapf(err, "error retrieving container %q", idOrName)
	}

	// Error out if the container exists in libpod
	exists, err := r.state.HasContainer(ctr.ID)
	if err != nil {
		return err
	}
	if exists {
		return errors.Wrapf(define.ErrCtrExists, "refusing to remove %q as it exists in libpod as container %s", idOrName, ctr.ID)
	}

	if !force {
		timesMounted, err := r.store.Mounted(ctr.ID)
		if err != nil {
			if errors.Cause(err) == storage.ErrContainerUnknown {
				// Container was removed from under us.
				// It's gone, so don't bother erroring.
				logrus.Infof("Storage for container %s already removed", ctr.ID)
				return nil
			}
			logrus.Warnf("Checking if container %q is mounted, attempting to delete: %v", idOrName, err)
		}
		if timesMounted > 0 {
			return errors.Wrapf(define.ErrCtrStateInvalid, "container %q is mounted and cannot be removed without using force", idOrName)
		}
	} else if _, err := r.store.Unmount(ctr.ID, true); err != nil {
		if errors.Is(err, storage.ErrContainerUnknown) {
			// Container again gone, no error
			logrus.Infof("Storage for container %s already removed", ctr.ID)
			return nil
		}
		logrus.Warnf("Unmounting container %q while attempting to delete storage: %v", idOrName, err)
	}

	if err := r.store.DeleteContainer(ctr.ID); err != nil {
		if errors.Cause(err) == storage.ErrNotAContainer || errors.Cause(err) == storage.ErrContainerUnknown {
			// Container again gone, no error
			logrus.Infof("Storage for container %s already removed", ctr.ID)
			return nil
		}
		return errors.Wrapf(err, "error removing storage for container %q", idOrName)
	}

	return nil
}
