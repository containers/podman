package entities

import (
	"net/url"

	"github.com/containers/podman/v4/pkg/domain/entities/types"
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

type VolumeConfigResponse = types.VolumeConfigResponse

type VolumeRmOptions struct {
	All     bool
	Force   bool
	Ignore  bool
	Timeout *uint
}

type VolumeRmReport = types.VolumeRmReport

type VolumeInspectReport = types.VolumeInspectReport

// VolumePruneOptions describes the options needed
// to prune a volume from the CLI
type VolumePruneOptions struct {
	Filters url.Values `json:"filters" schema:"filters"`
}

type VolumeListOptions struct {
	Filter map[string][]string
}

type VolumeListReport = types.VolumeListReport

// VolumeReloadReport describes the response from reload volume plugins
type VolumeReloadReport = types.VolumeReloadReport

/*
 * Docker API compatibility types
 */

// VolumeMountReport describes the response from volume mount
type VolumeMountReport = types.VolumeMountReport

// VolumeUnmountReport describes the response from umounting a volume
type VolumeUnmountReport = types.VolumeUnmountReport
