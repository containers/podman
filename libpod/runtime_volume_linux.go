// +build linux

package libpod

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/libpod/libpod/events"
	"github.com/containers/storage/pkg/stringid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NewVolume creates a new empty volume
func (r *Runtime) NewVolume(ctx context.Context, options ...VolumeCreateOption) (*Volume, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}
	return r.newVolume(ctx, options...)
}

// newVolume creates a new empty volume
func (r *Runtime) newVolume(ctx context.Context, options ...VolumeCreateOption) (*Volume, error) {
	volume, err := newVolume(r)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating volume")
	}

	for _, option := range options {
		if err := option(volume); err != nil {
			return nil, errors.Wrapf(err, "error running volume create option")
		}
	}

	if volume.config.Name == "" {
		volume.config.Name = stringid.GenerateNonCryptoID()
	}
	// TODO: support for other volume drivers
	if volume.config.Driver == "" {
		volume.config.Driver = "local"
	}
	// TODO: determine when the scope is global and set it to that
	if volume.config.Scope == "" {
		volume.config.Scope = "local"
	}

	// Create the mountpoint of this volume
	volPathRoot := filepath.Join(r.config.VolumePath, volume.config.Name)
	if err := os.MkdirAll(volPathRoot, 0700); err != nil {
		return nil, errors.Wrapf(err, "error creating volume directory %q", volPathRoot)
	}
	if err := os.Chown(volPathRoot, volume.config.UID, volume.config.GID); err != nil {
		return nil, errors.Wrapf(err, "error chowning volume directory %q to %d:%d", volPathRoot, volume.config.UID, volume.config.GID)
	}
	fullVolPath := filepath.Join(volPathRoot, "_data")
	if err := os.Mkdir(fullVolPath, 0755); err != nil {
		return nil, errors.Wrapf(err, "error creating volume directory %q", fullVolPath)
	}
	if err := os.Chown(fullVolPath, volume.config.UID, volume.config.GID); err != nil {
		return nil, errors.Wrapf(err, "error chowning volume directory %q to %d:%d", fullVolPath, volume.config.UID, volume.config.GID)
	}
	if err := LabelVolumePath(fullVolPath, true); err != nil {
		return nil, err
	}
	volume.config.MountPoint = fullVolPath

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
		return ErrVolumeRemoved
	}

	deps, err := r.state.VolumeInUse(v)
	if err != nil {
		return err
	}
	if len(deps) != 0 {
		depsStr := strings.Join(deps, ", ")
		if !force {
			return errors.Wrapf(ErrVolumeBeingUsed, "volume %s is being used by the following container(s): %s", v.Name(), depsStr)
		}
		// If using force, log the warning that the volume is being used by at least one container
		logrus.Warnf("volume %s is being used by the following container(s): %s", v.Name(), depsStr)
		// Remove the container dependencies so we can go ahead and delete the volume
		for _, dep := range deps {
			if err := r.state.RemoveVolCtrDep(v, dep); err != nil {
				return errors.Wrapf(err, "unable to remove container dependency %q from volume %q while trying to delete volume by force", dep, v.Name())
			}
		}
	}

	// Set volume as invalid so it can no longer be used
	v.valid = false

	// Remove the volume from the state
	if err := r.state.RemoveVolume(v); err != nil {
		return errors.Wrapf(err, "error removing volume %s", v.Name())
	}

	// Delete the mountpoint path of the volume, that is delete the volume from /var/lib/containers/storage/volumes
	if err := v.teardownStorage(); err != nil {
		return errors.Wrapf(err, "error cleaning up volume storage for %q", v.Name())
	}

	defer v.newVolumeEvent(events.Remove)
	logrus.Debugf("Removed volume %s", v.Name())
	return nil
}
