// +build linux

package libpod

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/storage/pkg/stringid"
	"github.com/opencontainers/selinux/go-selinux/label"
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
	fullVolPath := filepath.Join(r.config.VolumePath, volume.config.Name, "_data")
	if err := os.MkdirAll(fullVolPath, 0755); err != nil {
		return nil, errors.Wrapf(err, "error creating volume directory %q", fullVolPath)
	}
	_, mountLabel, err := label.InitLabels([]string{})
	if err != nil {
		return nil, errors.Wrapf(err, "error getting default mountlabels")
	}
	if err := label.ReleaseLabel(mountLabel); err != nil {
		return nil, errors.Wrapf(err, "error releasing label %q", mountLabel)
	}
	if err := label.Relabel(fullVolPath, mountLabel, true); err != nil {
		return nil, errors.Wrapf(err, "error setting selinux label to %q", fullVolPath)
	}
	volume.config.MountPoint = fullVolPath

	volume.valid = true

	// Add the volume to state
	if err := r.state.AddVolume(volume); err != nil {
		return nil, errors.Wrapf(err, "error adding volume to state")
	}

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

	logrus.Debugf("Removed volume %s", v.Name())

	return nil
}
