package entities

import (
	"net/url"

	"github.com/containers/podman/v5/pkg/domain/entities/types"
)

// VolumeCreateOptions provides details for creating volumes
type VolumeCreateOptions = types.VolumeCreateOptions

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
