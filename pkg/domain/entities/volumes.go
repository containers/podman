package entities

import (
	"net/url"

	"github.com/containers/podman/v4/libpod/define"
)

// VolumeCreateOptions provides details for creating volumes
// swagger:model
type VolumeCreateOptions struct {
	// New volume's name. Can be left blank
	Name string `schema:"name"`
	// Volume driver to use
	Driver string `schema:"driver"`
	// User-defined key/value metadata. Provided for compatibility
	Label map[string]string `schema:"label"`
	// User-defined key/value metadata. Preferred field, will override Label
	Labels map[string]string `schema:"labels"`
	// Mapping of driver options and values.
	Options map[string]string `schema:"opts"`
	// Ignore existing volumes
	IgnoreIfExists bool `schema:"ignoreIfExist"`
}

type VolumeConfigResponse struct {
	define.InspectVolumeData
}

type VolumeRmOptions struct {
	All     bool
	Force   bool
	Ignore  bool
	Timeout *uint
}

type VolumeRmReport struct {
	Err error
	Id  string //nolint:revive,stylecheck
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

// VolumeReloadReport describes the response from reload volume plugins
type VolumeReloadReport struct {
	define.VolumeReload
}

/*
 * Docker API compatibility types
 */

// VolumeMountReport describes the response from volume mount
type VolumeMountReport struct {
	Err  error
	Id   string //nolint:revive,stylecheck
	Name string
	Path string
}

// VolumeUnmountReport describes the response from umounting a volume
type VolumeUnmountReport struct {
	Err error
	Id  string //nolint:revive,stylecheck
}
