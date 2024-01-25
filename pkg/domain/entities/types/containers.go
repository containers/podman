package types

import (
	"github.com/containers/podman/v4/libpod/define"
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
