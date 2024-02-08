package types

import (
	"github.com/containers/podman/v5/libpod/define"
)

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

type VolumeRmReport struct {
	Err error
	Id  string //nolint:revive,stylecheck
}
type VolumeInspectReport struct {
	*VolumeConfigResponse
}

type VolumeListReport struct {
	VolumeConfigResponse
}

type VolumeReloadReport struct {
	define.VolumeReload
}

type VolumeMountReport struct {
	Err  error
	Id   string //nolint:revive,stylecheck
	Name string
	Path string
}

type VolumeUnmountReport struct {
	Err error
	Id  string //nolint:revive,stylecheck
}

type VolumeConfigResponse struct {
	define.InspectVolumeData
}
