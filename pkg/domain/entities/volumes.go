package entities

import (
	"net/url"

	"github.com/containers/podman/v3/libpod/define"
	docker_api_types "github.com/docker/docker/api/types"
	docker_api_types_volume "github.com/docker/docker/api/types/volume"
)

// Volume volume
// swagger:model Volume
type volume struct {

	// Date/Time the volume was created.
	CreatedAt string `json:"CreatedAt,omitempty"`

	// Name of the volume driver used by the volume.
	// Required: true
	Driver string `json:"Driver"`

	// User-defined key/value metadata.
	// Required: true
	Labels map[string]string `json:"Labels"`

	// Mount path of the volume on the host.
	// Required: true
	Mountpoint string `json:"Mountpoint"`

	// Name of the volume.
	// Required: true
	Name string `json:"Name"`

	// The driver specific options used when creating the volume.
	//
	// Required: true
	Options map[string]string `json:"Options"`

	// The level at which the volume exists. Either `global` for cluster-wide,
	// or `local` for machine level.
	//
	// Required: true
	Scope string `json:"Scope"`

	// Low-level details about the volume, provided by the volume driver.
	// Details are returned as a map with key/value pairs:
	// `{"key":"value","key2":"value2"}`.
	//
	// The `Status` field is optional, and is omitted if the volume driver
	// does not support this feature.
	//
	Status map[string]interface{} `json:"Status,omitempty"`

	// usage data
	UsageData *VolumeUsageData `json:"UsageData,omitempty"`
}

type VolumeUsageData struct {

	// The number of containers referencing this volume. This field
	// is set to `-1` if the reference-count is not available.
	//
	// Required: true
	RefCount int64 `json:"RefCount"`

	// Amount of disk space used by the volume (in bytes). This information
	// is only available for volumes created with the `"local"` volume
	// driver. For volumes created with other volume drivers, this field
	// is set to `-1` ("not available")
	//
	// Required: true
	Size int64 `json:"Size"`
}

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
	Volumes []docker_api_types_volume.VolumeListOKBody
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
type DockerVolumeCreate VolumeCreateBody

// This response definition is used for both the create and inspect endpoints
// swagger:response DockerVolumeInfoResponse
type SwagDockerVolumeInfoResponse struct {
	// in:body
	Body struct {
		volume
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

// VolumeCreateBody Volume configuration
// swagger:model VolumeCreateBody
type VolumeCreateBody struct {

	// Name of the volume driver to use.
	// Required: true
	Driver string `json:"Driver"`

	// A mapping of driver options and values. These options are
	// passed directly to the driver and are driver specific.
	//
	// Required: true
	DriverOpts map[string]string `json:"DriverOpts"`

	// User-defined key/value metadata.
	// Required: true
	Labels map[string]string `json:"Labels"`

	// The new volume's name. If not specified, Docker generates a name.
	//
	// Required: true
	Name string `json:"Name"`
}
