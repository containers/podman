package libpod

import (
	"os"
	"path/filepath"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/pkg/errors"
)

// Creates a new volume
func newVolume(runtime *Runtime) *Volume {
	volume := new(Volume)
	volume.config = new(VolumeConfig)
	volume.state = new(VolumeState)
	volume.runtime = runtime
	volume.config.Labels = make(map[string]string)
	volume.config.Options = make(map[string]string)
	volume.state.NeedsCopyUp = true
	return volume
}

// teardownStorage deletes the volume from volumePath
func (v *Volume) teardownStorage() error {
	return os.RemoveAll(filepath.Join(v.runtime.config.Engine.VolumePath, v.Name()))
}

// Volumes with options set, or a filesystem type, or a device to mount need to
// be mounted and unmounted.
func (v *Volume) needsMount() bool {
	return len(v.config.Options) > 0 && v.config.Driver == define.VolumeDriverLocal
}

// update() updates the volume state from the DB.
func (v *Volume) update() error {
	if err := v.runtime.state.UpdateVolume(v); err != nil {
		return err
	}
	if !v.valid {
		return define.ErrVolumeRemoved
	}
	return nil
}

// save() saves the volume state to the DB
func (v *Volume) save() error {
	return v.runtime.state.SaveVolume(v)
}

// Refresh volume state after a restart.
func (v *Volume) refresh() error {
	lock, err := v.runtime.lockManager.AllocateAndRetrieveLock(v.config.LockID)
	if err != nil {
		return errors.Wrapf(err, "error acquiring lock %d for volume %s", v.config.LockID, v.Name())
	}
	v.lock = lock

	return nil
}
