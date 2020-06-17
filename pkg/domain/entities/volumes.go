package entities

import (
	"time"
)

// swagger:model VolumeCreate
type VolumeCreateOptions struct {
	// New volume's name. Can be left blank
	Name string `schema:"name"`
	// Volume driver to use
	Driver string `schema:"driver"`
	// User-defined key/value metadata.
	Label map[string]string `schema:"label"`
	// Mapping of driver options and values.
	Options map[string]string `schema:"opts"`
}

type IDOrNameResponse struct {
	// The Id or Name of an object
	IDOrName string
}

type VolumeConfigResponse struct {
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
	UID int `json:"UID"`
	// GID is the GID that the volume was created with.
	GID int `json:"GID"`
	// Anonymous indicates that the volume was created as an anonymous
	// volume for a specific container, and will be be removed when any
	// container using it is removed.
	Anonymous bool `json:"Anonymous"`
}

type VolumeRmOptions struct {
	All   bool
	Force bool
}

type VolumeRmReport struct {
	Err error
	Id  string //nolint
}

type VolumeInspectOptions struct {
	All bool
}

type VolumeInspectReport struct {
	*VolumeConfigResponse
}

type VolumePruneOptions struct {
	Force bool
}

type VolumePruneReport struct {
	Err error
	Id  string //nolint
}

type VolumeListOptions struct {
	Filter map[string][]string
}

type VolumeListReport struct {
	VolumeConfigResponse
}
