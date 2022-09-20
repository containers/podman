//go:build !linux && !freebsd
// +build !linux,!freebsd

package libpod

import (
	"context"
	"errors"

	"github.com/containers/podman/v4/libpod/define"
)

// NewVolume creates a new empty volume
func (r *Runtime) NewVolume(ctx context.Context, options ...VolumeCreateOption) (*Volume, error) {
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}
	return r.newVolume(false, options...)
}

// NewVolume creates a new empty volume
func (r *Runtime) newVolume(noCreatePluginVolume bool, options ...VolumeCreateOption) (*Volume, error) {
	return nil, errors.New("not implemented (*Runtime) newVolume")
}

// UpdateVolumePlugins reads all volumes from all configured volume plugins and
// imports them into the libpod db. It also checks if existing libpod volumes
// are removed in the plugin, in this case we try to remove it from libpod.
// On errors we continue and try to do as much as possible. all errors are
// returned as array in the returned struct.
// This function has many race conditions, it is best effort but cannot guarantee
// a perfect state since plugins can be modified from the outside at any time.
func (r *Runtime) UpdateVolumePlugins(ctx context.Context) *define.VolumeReload {
	return nil
}

// removeVolume removes the specified volume from state as well tears down its mountpoint and storage.
// ignoreVolumePlugin is used to only remove the volume from the db and not the plugin,
// this is required when the volume was already removed from the plugin, i.e. in UpdateVolumePlugins().
func (r *Runtime) removeVolume(ctx context.Context, v *Volume, force bool, timeout *uint, ignoreVolumePlugin bool) error {
	return errors.New("not implemented (*Runtime) removeVolume")
}
