// +build !linux

package libpod

import (
	"github.com/containers/common/pkg/config"

	"github.com/containers/podman/v2/libpod/define"
)

const (
	osNotSupported = "Not supported on this OS"
)

// ConmonOCIRuntime is not supported on this OS.
type ConmonOCIRuntime struct {
}

// newConmonOCIRuntime is not supported on this OS.
func newConmonOCIRuntime(name string, paths []string, conmonPath string, runtimeFlags []string, runtimeCfg *config.Config) (OCIRuntime, error) {
	return nil, define.ErrNotImplemented
}

// Name is not supported on this OS.
func (r *ConmonOCIRuntime) Name() string {
	return osNotSupported
}

// Path is not supported on this OS.
func (r *ConmonOCIRuntime) Path() string {
	return osNotSupported
}

// CreateContainer is not supported on this OS.
func (r *ConmonOCIRuntime) CreateContainer(ctr *Container, restoreOptions *ContainerCheckpointOptions) error {
	return define.ErrNotImplemented
}

// UpdateContainerStatus is not supported on this OS.
func (r *ConmonOCIRuntime) UpdateContainerStatus(ctr *Container, useRuntime bool) error {
	return define.ErrNotImplemented
}

// StartContainer is not supported on this OS.
func (r *ConmonOCIRuntime) StartContainer(ctr *Container) error {
	return define.ErrNotImplemented
}

// KillContainer is not supported on this OS.
func (r *ConmonOCIRuntime) KillContainer(ctr *Container, signal uint, all bool) error {
	return define.ErrNotImplemented
}

// StopContainer is not supported on this OS.
func (r *ConmonOCIRuntime) StopContainer(ctr *Container, timeout uint, all bool) error {
	return define.ErrNotImplemented
}

// DeleteContainer is not supported on this OS.
func (r *ConmonOCIRuntime) DeleteContainer(ctr *Container) error {
	return define.ErrNotImplemented
}

// PauseContainer is not supported on this OS.
func (r *ConmonOCIRuntime) PauseContainer(ctr *Container) error {
	return define.ErrNotImplemented
}

// UnpauseContainer is not supported on this OS.
func (r *ConmonOCIRuntime) UnpauseContainer(ctr *Container) error {
	return define.ErrNotImplemented
}

// ExecContainer is not supported on this OS.
func (r *ConmonOCIRuntime) ExecContainer(ctr *Container, sessionID string, options *ExecOptions) (int, chan error, error) {
	return -1, nil, define.ErrNotImplemented
}

// ExecStopContainer is not supported on this OS.
func (r *ConmonOCIRuntime) ExecStopContainer(ctr *Container, sessionID string, timeout uint) error {
	return define.ErrNotImplemented
}

// CheckpointContainer is not supported on this OS.
func (r *ConmonOCIRuntime) CheckpointContainer(ctr *Container, options ContainerCheckpointOptions) error {
	return define.ErrNotImplemented
}

// SupportsCheckpoint is not supported on this OS.
func (r *ConmonOCIRuntime) SupportsCheckpoint() bool {
	return false
}

// SupportsJSONErrors is not supported on this OS.
func (r *ConmonOCIRuntime) SupportsJSONErrors() bool {
	return false
}

// SupportsNoCgroups is not supported on this OS.
func (r *ConmonOCIRuntime) SupportsNoCgroups() bool {
	return false
}

// AttachSocketPath is not supported on this OS.
func (r *ConmonOCIRuntime) AttachSocketPath(ctr *Container) (string, error) {
	return "", define.ErrNotImplemented
}

// ExecAttachSocketPath is not supported on this OS.
func (r *ConmonOCIRuntime) ExecAttachSocketPath(ctr *Container, sessionID string) (string, error) {
	return "", define.ErrNotImplemented
}

// ExitFilePath is not supported on this OS.
func (r *ConmonOCIRuntime) ExitFilePath(ctr *Container) (string, error) {
	return "", define.ErrNotImplemented
}

// RuntimeInfo is not supported on this OS.
func (r *ConmonOCIRuntime) RuntimeInfo() (*define.ConmonInfo, *define.OCIRuntimeInfo, error) {
	return nil, nil, define.ErrNotImplemented
}

// Package is not supported on this OS.
func (r *ConmonOCIRuntime) Package() string {
	return osNotSupported
}

// ConmonPackage is not supported on this OS.
func (r *ConmonOCIRuntime) ConmonPackage() string {
	return osNotSupported
}
