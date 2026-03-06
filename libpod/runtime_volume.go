//go:build !remote && (linux || freebsd)

package libpod

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v6/libpod/define"
	"github.com/containers/podman/v6/libpod/events"
	"github.com/containers/podman/v6/pkg/domain/entities/reports"
	"github.com/sirupsen/logrus"
)

// Contains the public Runtime API for volumes

// A VolumeCreateOption is a functional option which alters the Volume created by
// NewVolume
type VolumeCreateOption func(*Volume) error

// VolumeFilter is a function to determine whether a volume is included in command
// output. Volumes to be outputted are tested using the function. a true return will
// include the volume, a false return will exclude it.
type VolumeFilter func(*Volume) bool

// RemoveVolume removes a volumes
func (r *Runtime) RemoveVolume(ctx context.Context, v *Volume, force bool, timeout *uint) error {
	if !r.valid {
		return define.ErrRuntimeStopped
	}

	return r.removeVolume(ctx, v, force, timeout, false)
}

// GetVolume retrieves a volume given its full name.
func (r *Runtime) GetVolume(name string) (*Volume, error) {
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	vol, err := r.state.Volume(name)
	if err != nil {
		return nil, err
	}

	return vol, nil
}

// LookupVolume retrieves a volume by unambiguous partial name.
func (r *Runtime) LookupVolume(name string) (*Volume, error) {
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	vol, err := r.state.LookupVolume(name)
	if err != nil {
		return nil, err
	}

	return vol, nil
}

// HasVolume checks to see if a volume with the given name exists
func (r *Runtime) HasVolume(name string) (bool, error) {
	if !r.valid {
		return false, define.ErrRuntimeStopped
	}

	return r.state.HasVolume(name)
}

// Volumes retrieves all volumes
// Filters can be provided which will determine which volumes are included in the
// output. If multiple filters are used, a volume will be returned if
// any of the filters are matched
func (r *Runtime) Volumes(filters ...VolumeFilter) ([]*Volume, error) {
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	vols, err := r.state.AllVolumes()
	if err != nil {
		return nil, err
	}

	if len(filters) == 0 {
		return vols, nil
	}

	volsFiltered := make([]*Volume, 0, len(vols))
	for _, vol := range vols {
		include := false
		for _, filter := range filters {
			include = include || filter(vol)
		}

		if include {
			volsFiltered = append(volsFiltered, vol)
		}
	}

	return volsFiltered, nil
}

// GetAllVolumes retrieves all the volumes
func (r *Runtime) GetAllVolumes() ([]*Volume, error) {
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	return r.state.AllVolumes()
}

// RenameVolume renames the given volume to a new name.
// The volume must not be in use by any containers, and must use the
// local driver (not a volume plugin).
func (r *Runtime) RenameVolume(_ context.Context, vol *Volume, newName string) (*Volume, error) {
	if !r.valid {
		return nil, define.ErrRuntimeStopped
	}

	vol.lock.Lock()
	defer vol.lock.Unlock()

	if err := vol.update(); err != nil {
		return nil, err
	}

	newName = strings.TrimSpace(newName)
	if newName == "" || !define.NameRegex.MatchString(newName) {
		return nil, define.RegexError
	}

	if vol.Name() == newName {
		return nil, fmt.Errorf("renaming volume %s: new name is the same as the old name: %w", vol.Name(), define.ErrInvalidArg)
	}

	// Anonymous volumes should not be renamed
	if vol.config.IsAnon {
		return nil, fmt.Errorf("renaming volume %s: cannot rename anonymous volumes: %w", vol.Name(), define.ErrInvalidArg)
	}

	// Image-driver volumes cannot be renamed
	if vol.config.Driver == define.VolumeDriverImage {
		return nil, fmt.Errorf("renaming volume %s: rename is not supported for image-based volumes: %w", vol.Name(), define.ErrInvalidArg)
	}

	// Only local-driver volumes can be renamed
	if vol.UsesVolumeDriver() {
		return nil, fmt.Errorf("renaming volume %s: rename is not supported for volumes using driver %q: %w", vol.Name(), vol.Driver(), define.ErrInvalidArg)
	}

	// Refuse rename if the volume is currently mounted
	if vol.state.MountCount > 0 {
		return nil, fmt.Errorf("renaming volume %s: volume is currently mounted: %w", vol.Name(), define.ErrVolumeBeingUsed)
	}

	// Refuse rename if the volume is in use by any container
	ctrs, err := r.state.VolumeInUse(vol)
	if err != nil {
		return nil, fmt.Errorf("checking if volume %s is in use: %w", vol.Name(), err)
	}
	if len(ctrs) > 0 {
		return nil, fmt.Errorf("volume %s is being used by the following container(s): %s: %w", vol.Name(), strings.Join(ctrs, ", "), define.ErrVolumeBeingUsed)
	}

	// Check that no volume with the new name already exists
	if _, err := r.state.Volume(newName); err == nil {
		return nil, fmt.Errorf("volume with name %q already exists: %w", newName, define.ErrVolumeExists)
	}

	oldName := vol.config.Name
	oldMountPoint := vol.config.MountPoint

	// Rename the filesystem directory
	oldPath := filepath.Join(r.config.Engine.VolumePath, oldName)
	newPath := filepath.Join(r.config.Engine.VolumePath, newName)
	if err := os.Rename(oldPath, newPath); err != nil {
		// If the directory doesn't exist (e.g., plugin volumes), skip
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("renaming volume directory %q to %q: %w", oldPath, newPath, err)
		}
	}

	// Update mount point in-memory before the DB call. This is safe
	// because MountPoint is only stored in the JSON blob, not used in
	// any SQL WHERE clauses. The Name field stays unchanged so the
	// WHERE clauses still match the old name correctly.
	vol.config.MountPoint = filepath.Join(r.config.Engine.VolumePath, newName, "_data")

	// Persist to database (vol.config.Name still equals oldName,
	// so the SQL WHERE clauses match correctly).
	if err := r.state.RenameVolume(vol, newName); err != nil {
		// Rollback filesystem rename and mount point
		vol.config.MountPoint = oldMountPoint
		if rerr := os.Rename(newPath, oldPath); rerr != nil {
			logrus.Errorf("Failed to rollback volume directory rename from %q to %q: %v", newPath, oldPath, rerr)
		}
		return nil, fmt.Errorf("renaming volume %s in database: %w", oldName, err)
	}

	// Update the name in-memory only after the database has been
	// successfully updated.
	vol.config.Name = newName

	vol.newVolumeEvent(events.Rename)
	return vol, nil
}

// PruneVolumes removes unused volumes from the system
func (r *Runtime) PruneVolumes(ctx context.Context, filterFuncs []VolumeFilter) ([]*reports.PruneReport, error) {
	preports := make([]*reports.PruneReport, 0)
	vols, err := r.Volumes(filterFuncs...)
	if err != nil {
		return nil, err
	}

	for _, vol := range vols {
		report := new(reports.PruneReport)
		volSize, err := vol.Size()
		if err != nil {
			volSize = 0
		}
		report.Size = volSize
		report.Id = vol.Name()
		var timeout *uint
		if err := r.RemoveVolume(ctx, vol, false, timeout); err != nil {
			if !errors.Is(err, define.ErrVolumeBeingUsed) && !errors.Is(err, define.ErrVolumeRemoved) {
				report.Err = err
			} else {
				// We didn't remove the volume for some reason
				continue
			}
		} else {
			vol.newVolumeEvent(events.Prune)
		}
		preports = append(preports, report)
	}
	return preports, nil
}
