package entities

import (
	"time"

	docker_api_types "github.com/docker/docker/api/types"
	docker_api_types_volume "github.com/docker/docker/api/types/volume"
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

// VolumeInfo Volume list response
// swagger:model VolumeInfo
type VolumeInfo struct {

	// Date/Time the volume was created.
	CreatedAt string `json:"CreatedAt,omitempty"`

	// Name of the volume driver used by the volume. Only supports local driver
	// Required: true
	Driver string `json:"Driver"`

	// User-defined key/value metadata.
	// Always included
	Labels map[string]string `json:"Labels"`

	// Mount path of the volume on the host.
	// Required: true
	Mountpoint string `json:"Mountpoint"`

	// Name of the volume.
	// Required: true
	Name string `json:"Name"`

	// The driver specific options used when creating the volume.
	// Required: true
	Options map[string]string `json:"Options"`

	// The level at which the volume exists.
	// Libpod does not implement volume scoping, and this is provided solely for
	// Docker compatibility. The value is only "local".
	// Required: true
	Scope string `json:"Scope"`

	// TODO: We don't include the volume `Status` for now
}

type VolumeRmOptions struct {
	All   bool
	Force bool
}

type VolumeRmReport struct {
	Err error
	Id  string //nolint
}

type VolumeInspectReport struct {
	*VolumeConfigResponse
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

// VolumeListBody Volume list response
// swagger:model VolumeListBody
type VolumeListBody struct {
	Volumes []*VolumeInfo
}

// Volume list response
// swagger:response VolumeListResponse
type SwagVolumeListResponse struct {
	// in:body
	Body struct {
		VolumeListBody
	}
}

/*
 * Docker API compatibility types
 */

// swagger:model DockerVolumeCreate
type DockerVolumeCreate docker_api_types_volume.VolumeCreateBody

// This response definition is used for both the create and inspect endpoints
// swagger:response DockerVolumeInfoResponse
type SwagDockerVolumeInfoResponse struct {
	// in:body
	Body struct {
		docker_api_types.Volume
	}
}

// Volume prune response
// swagger:response DockerVolumePruneResponse
type SwagDockerVolumePruneResponse struct {
	// in:body
	Body struct {
		docker_api_types.VolumesPruneReport
	}
}
