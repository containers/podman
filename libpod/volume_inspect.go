package libpod

import (
	"time"

	"github.com/containers/podman/v2/libpod/define"
)

// InspectVolumeData is the output of Inspect() on a volume. It is matched to
// the format of 'docker volume inspect'.
type InspectVolumeData struct {
	// Name is the name of the volume.
	Name string `json:"Name"`
	// Driver is the driver used to create the volume.
	// This will be properly implemented in a future version.
	Driver string `json:"Driver"`
	// Mountpoint is the path on the host where the volume is mounted.
	Mountpoint string `json:"Mountpoint"`
	// CreatedAt is the date and time the volume was created at. This is not
	// stored for older Libpod volumes; if so, it will be omitted.
	CreatedAt time.Time `json:"CreatedAt,omitempty"`
	// Status is presently unused and provided only for Docker compatibility.
	// In the future it will be used to return information on the volume's
	// current state.
	Status map[string]string `json:"Status,omitempty"`
	// Labels includes the volume's configured labels, key:value pairs that
	// can be passed during volume creation to provide information for third
	// party tools.
	Labels map[string]string `json:"Labels"`
	// Scope is unused and provided solely for Docker compatibility. It is
	// unconditionally set to "local".
	Scope string `json:"Scope"`
	// Options is a set of options that were used when creating the volume.
	// It is presently not used.
	Options map[string]string `json:"Options"`
	// UID is the UID that the volume was created with.
	UID int `json:"UID,omitempty"`
	// GID is the GID that the volume was created with.
	GID int `json:"GID,omitempty"`
	// Anonymous indicates that the volume was created as an anonymous
	// volume for a specific container, and will be be removed when any
	// container using it is removed.
	Anonymous bool `json:"Anonymous,omitempty"`
}

// Inspect provides detailed information about the configuration of the given
// volume.
func (v *Volume) Inspect() (*InspectVolumeData, error) {
	if !v.valid {
		return nil, define.ErrVolumeRemoved
	}

	data := new(InspectVolumeData)

	data.Name = v.config.Name
	data.Driver = v.config.Driver
	data.Mountpoint = v.config.MountPoint
	data.CreatedAt = v.config.CreatedTime
	data.Labels = make(map[string]string)
	for k, v := range v.config.Labels {
		data.Labels[k] = v
	}
	data.Scope = v.Scope()
	data.Options = make(map[string]string)
	for k, v := range v.config.Options {
		data.Options[k] = v
	}
	var err error
	data.UID, err = v.UID()
	if err != nil {
		return nil, err
	}
	data.GID, err = v.GID()
	if err != nil {
		return nil, err
	}
	data.Anonymous = v.config.IsAnon

	return data, nil
}
