package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *CheckpointOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *CheckpointOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithExport
func (o *CheckpointOptions) WithExport(value string) *CheckpointOptions {
	v := &value
	o.Export = v
	return o
}

// GetExport
func (o *CheckpointOptions) GetExport() string {
	var export string
	if o.Export == nil {
		return export
	}
	return *o.Export
}

// WithIgnoreRootfs
func (o *CheckpointOptions) WithIgnoreRootfs(value bool) *CheckpointOptions {
	v := &value
	o.IgnoreRootfs = v
	return o
}

// GetIgnoreRootfs
func (o *CheckpointOptions) GetIgnoreRootfs() bool {
	var ignoreRootfs bool
	if o.IgnoreRootfs == nil {
		return ignoreRootfs
	}
	return *o.IgnoreRootfs
}

// WithKeep
func (o *CheckpointOptions) WithKeep(value bool) *CheckpointOptions {
	v := &value
	o.Keep = v
	return o
}

// GetKeep
func (o *CheckpointOptions) GetKeep() bool {
	var keep bool
	if o.Keep == nil {
		return keep
	}
	return *o.Keep
}

// WithLeaveRunning
func (o *CheckpointOptions) WithLeaveRunning(value bool) *CheckpointOptions {
	v := &value
	o.LeaveRunning = v
	return o
}

// GetLeaveRunning
func (o *CheckpointOptions) GetLeaveRunning() bool {
	var leaveRunning bool
	if o.LeaveRunning == nil {
		return leaveRunning
	}
	return *o.LeaveRunning
}

// WithTCPEstablished
func (o *CheckpointOptions) WithTCPEstablished(value bool) *CheckpointOptions {
	v := &value
	o.TCPEstablished = v
	return o
}

// GetTCPEstablished
func (o *CheckpointOptions) GetTCPEstablished() bool {
	var tCPEstablished bool
	if o.TCPEstablished == nil {
		return tCPEstablished
	}
	return *o.TCPEstablished
}
