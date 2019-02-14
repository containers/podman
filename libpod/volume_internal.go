package libpod

import (
	"os"
	"path/filepath"
)

// Creates a new volume
func newVolume(runtime *Runtime) (*Volume, error) {
	volume := new(Volume)
	volume.config = new(VolumeConfig)
	volume.runtime = runtime
	volume.config.Labels = make(map[string]string)
	volume.config.Options = make(map[string]string)

	return volume, nil
}

// teardownStorage deletes the volume from volumePath
func (v *Volume) teardownStorage() error {
	if !v.valid {
		return ErrNoSuchVolume
	}
	return os.RemoveAll(filepath.Join(v.runtime.config.VolumePath, v.Name()))
}
