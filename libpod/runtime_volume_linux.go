// +build linux

package libpod

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/events"
	"github.com/containers/storage/pkg/stringid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NewVolume creates a new empty volume
func (r *Runtime) NewVolume(ctx context.Context, options ...VolumeCreateOption) (*Volume, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}
	return r.newVolume(ctx, options...)
}

// newVolume creates a new empty volume
func (r *Runtime) newVolume(ctx context.Context, options ...VolumeCreateOption) (_ *Volume, deferredErr error) {
	volume := newVolume(r)
	for _, option := range options {
		if err := option(volume); err != nil {
			return nil, errors.Wrapf(err, "error running volume create option")
		}
	}

	if volume.config.Name == "" {
		volume.config.Name = stringid.GenerateNonCryptoID()
	}
	if volume.config.Driver == "" {
		volume.config.Driver = define.VolumeDriverLocal
	}
	volume.config.CreatedTime = time.Now()

	// Check if volume with given name exists.
	exists, err := r.state.HasVolume(volume.config.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "error checking if volume with name %s exists", volume.config.Name)
	}
	if exists {
		return nil, errors.Wrapf(define.ErrVolumeExists, "volume with name %s already exists", volume.config.Name)
	}

	if volume.config.Driver == define.VolumeDriverLocal {
		logrus.Debugf("Validating options for local driver")
		// Validate options
		for key := range volume.config.Options {
			switch key {
			case "device", "o", "type":
				// Do nothing, valid keys
			default:
				return nil, errors.Wrapf(define.ErrInvalidArg, "invalid mount option %s for driver 'local'", key)
			}
		}
	}

	// Create the mountpoint of this volume
	volPathRoot := filepath.Join(r.config.Engine.VolumePath, volume.config.Name)
	if err := os.MkdirAll(volPathRoot, 0700); err != nil {
		return nil, errors.Wrapf(err, "error creating volume directory %q", volPathRoot)
	}
	if err := os.Chown(volPathRoot, volume.config.UID, volume.config.GID); err != nil {
		return nil, errors.Wrapf(err, "error chowning volume directory %q to %d:%d", volPathRoot, volume.config.UID, volume.config.GID)
	}
	fullVolPath := filepath.Join(volPathRoot, "_data")
	if err := os.MkdirAll(fullVolPath, 0755); err != nil {
		return nil, errors.Wrapf(err, "error creating volume directory %q", fullVolPath)
	}
	if err := os.Chown(fullVolPath, volume.config.UID, volume.config.GID); err != nil {
		return nil, errors.Wrapf(err, "error chowning volume directory %q to %d:%d", fullVolPath, volume.config.UID, volume.config.GID)
	}
	if err := LabelVolumePath(fullVolPath); err != nil {
		return nil, err
	}
	volume.config.MountPoint = fullVolPath

	lock, err := r.lockManager.AllocateLock()
	if err != nil {
		return nil, errors.Wrapf(err, "error allocating lock for new volume")
	}
	volume.lock = lock
	volume.config.LockID = volume.lock.ID()

	defer func() {
		if deferredErr != nil {
			if err := volume.lock.Free(); err != nil {
				logrus.Errorf("Error freeing volume lock after failed creation: %v", err)
			}
		}
	}()

	volume.valid = true

	// Add the volume to state
	if err := r.state.AddVolume(volume); err != nil {
		return nil, errors.Wrapf(err, "error adding volume to state")
	}
	defer volume.newVolumeEvent(events.Create)
	return volume, nil
}

// removeVolume removes the specified volume from state as well tears down its mountpoint and storage
func (r *Runtime) removeVolume(ctx context.Context, v *Volume, force bool) error {
	if !v.valid {
		if ok, _ := r.state.HasVolume(v.Name()); !ok {
			return nil
		}
		return define.ErrVolumeRemoved
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	// Update volume status to pick up a potential removal from state
	if err := v.update(); err != nil {
		return err
	}

	deps, err := r.state.VolumeInUse(v)
	if err != nil {
		return err
	}
	if len(deps) != 0 {
		depsStr := strings.Join(deps, ", ")
		if !force {
			return errors.Wrapf(define.ErrVolumeBeingUsed, "volume %s is being used by the following container(s): %s", v.Name(), depsStr)
		}

		// We need to remove all containers using the volume
		for _, dep := range deps {
			ctr, err := r.state.Container(dep)
			if err != nil {
				// If the container's removed, no point in
				// erroring.
				if errors.Cause(err) == define.ErrNoSuchCtr || errors.Cause(err) == define.ErrCtrRemoved {
					continue
				}

				return errors.Wrapf(err, "error removing container %s that depends on volume %s", dep, v.Name())
			}

			logrus.Debugf("Removing container %s (depends on volume %q)", ctr.ID(), v.Name())

			// TODO: do we want to set force here when removing
			// containers?
			// I'm inclined to say no, in case someone accidentally
			// wipes a container they're using...
			if err := r.removeContainer(ctx, ctr, false, false, false); err != nil {
				return errors.Wrapf(err, "error removing container %s that depends on volume %s", ctr.ID(), v.Name())
			}
		}
	}

	// If the volume is still mounted - force unmount it
	if err := v.unmount(true); err != nil {
		if force {
			// If force is set, evict the volume, even if errors
			// occur. Otherwise we'll never be able to get rid of
			// them.
			logrus.Errorf("Error unmounting volume %s: %v", v.Name(), err)
		} else {
			return errors.Wrapf(err, "error unmounting volume %s", v.Name())
		}
	}

	// Set volume as invalid so it can no longer be used
	v.valid = false

	// Remove the volume from the state
	if err := r.state.RemoveVolume(v); err != nil {
		return errors.Wrapf(err, "error removing volume %s", v.Name())
	}

	var removalErr error

	// Free the volume's lock
	if err := v.lock.Free(); err != nil {
		removalErr = errors.Wrapf(err, "error freeing lock for volume %s", v.Name())
	}

	// Delete the mountpoint path of the volume, that is delete the volume
	// from /var/lib/containers/storage/volumes
	if err := v.teardownStorage(); err != nil {
		if removalErr == nil {
			removalErr = errors.Wrapf(err, "error cleaning up volume storage for %q", v.Name())
		} else {
			logrus.Errorf("error cleaning up volume storage for volume %q: %v", v.Name(), err)
		}
	}

	defer v.newVolumeEvent(events.Remove)
	logrus.Debugf("Removed volume %s", v.Name())
	return removalErr
}
