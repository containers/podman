package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *AttachOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *AttachOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithDetachKeys
func (o *AttachOptions) WithDetachKeys(value string) *AttachOptions {
	v := &value
	o.DetachKeys = v
	return o
}

// GetDetachKeys
func (o *AttachOptions) GetDetachKeys() string {
	var detachKeys string
	if o.DetachKeys == nil {
		return detachKeys
	}
	return *o.DetachKeys
}

// WithLogs
func (o *AttachOptions) WithLogs(value bool) *AttachOptions {
	v := &value
	o.Logs = v
	return o
}

// GetLogs
func (o *AttachOptions) GetLogs() bool {
	var logs bool
	if o.Logs == nil {
		return logs
	}
	return *o.Logs
}

// WithStream
func (o *AttachOptions) WithStream(value bool) *AttachOptions {
	v := &value
	o.Stream = v
	return o
}

// GetStream
func (o *AttachOptions) GetStream() bool {
	var stream bool
	if o.Stream == nil {
		return stream
	}
	return *o.Stream
}
