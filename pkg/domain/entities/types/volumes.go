package types

import (
	"github.com/containers/podman/v4/libpod/define"
)

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
