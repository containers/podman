package libpod

import (
	"context"
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
func (r *Runtime) RemoveVolume(ctx context.Context, v *Volume, force, prune bool) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	if !v.valid {
		if ok, _ := r.state.HasVolume(v.Name()); !ok {
			// Volume probably already removed
			// Or was never in the runtime to begin with
			return nil
		}
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	return r.removeVolume(ctx, v, force, prune)
}

// GetVolume retrieves a volume by its name
func (r *Runtime) GetVolume(name string) (*Volume, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	return r.state.Volume(name)
}

// HasVolume checks to see if a volume with the given name exists
func (r *Runtime) HasVolume(name string) (bool, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return false, ErrRuntimeStopped
	}

	return r.state.HasVolume(name)
}

// Volumes retrieves all volumes
// Filters can be provided which will determine which volumes are included in the
// output. Multiple filters are handled by ANDing their output, so only volumes
// matching all filters are returned
func (r *Runtime) Volumes(filters ...VolumeFilter) ([]*Volume, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	vols, err := r.state.AllVolumes()
	if err != nil {
		return nil, err
	}

	volsFiltered := make([]*Volume, 0, len(vols))
	for _, vol := range vols {
		include := true
		for _, filter := range filters {
			include = include && filter(vol)
		}

		if include {
			volsFiltered = append(volsFiltered, vol)
		}
	}

	return volsFiltered, nil
}

// GetAllVolumes retrieves all the volumes
func (r *Runtime) GetAllVolumes() ([]*Volume, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	return r.state.AllVolumes()
}
