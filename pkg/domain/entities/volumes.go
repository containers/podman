package entities

import (
	"net/url"

	"github.com/containers/podman/v2/libpod/define"
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
	define.InspectVolumeData
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

// VolumePruneOptions describes the options needed
// to prune a volume from the CLI
type VolumePruneOptions struct {
	Filters url.Values `json:"filters" schema:"filters"`
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
