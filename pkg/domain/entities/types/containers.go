package types

import (
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/specgen"
)

type ContainerCopyFunc func() error

type ContainerStatReport struct {
	define.FileInfo
}

type CheckpointReport struct {
	Err             error                                   `json:"-"`
	Id              string                                  `json:"Id"` //nolint:revive,stylecheck
	RawInput        string                                  `json:"-"`
	RuntimeDuration int64                                   `json:"runtime_checkpoint_duration"`
	CRIUStatistics  *define.CRIUCheckpointRestoreStatistics `json:"criu_statistics"`
}

type RestoreReport struct {
	Err             error                                   `json:"-"`
	Id              string                                  `json:"Id"` //nolint:revive,stylecheck
	RawInput        string                                  `json:"-"`
	RuntimeDuration int64                                   `json:"runtime_restore_duration"`
	CRIUStatistics  *define.CRIUCheckpointRestoreStatistics `json:"criu_statistics"`
}

// ContainerStatsReport is used for streaming container stats.
type ContainerStatsReport struct {
	// Error from reading stats.
	Error error
	// Results, set when there is no error.
	Stats []define.ContainerStats
}

type ContainerUpdateOptions struct {
	NameOrID string
	Specgen  *specgen.SpecGenerator
}
