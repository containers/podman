package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *LogOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *LogOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithFollow
func (o *LogOptions) WithFollow(value bool) *LogOptions {
	v := &value
	o.Follow = v
	return o
}

// GetFollow
func (o *LogOptions) GetFollow() bool {
	var follow bool
	if o.Follow == nil {
		return follow
	}
	return *o.Follow
}

// WithSince
func (o *LogOptions) WithSince(value string) *LogOptions {
	v := &value
	o.Since = v
	return o
}

// GetSince
func (o *LogOptions) GetSince() string {
	var since string
	if o.Since == nil {
		return since
	}
	return *o.Since
}

// WithStderr
func (o *LogOptions) WithStderr(value bool) *LogOptions {
	v := &value
	o.Stderr = v
	return o
}

// GetStderr
func (o *LogOptions) GetStderr() bool {
	var stderr bool
	if o.Stderr == nil {
		return stderr
	}
	return *o.Stderr
}

// WithStdout
func (o *LogOptions) WithStdout(value bool) *LogOptions {
	v := &value
	o.Stdout = v
	return o
}

// GetStdout
func (o *LogOptions) GetStdout() bool {
	var stdout bool
	if o.Stdout == nil {
		return stdout
	}
	return *o.Stdout
}

// WithTail
func (o *LogOptions) WithTail(value string) *LogOptions {
	v := &value
	o.Tail = v
	return o
}

// GetTail
func (o *LogOptions) GetTail() string {
	var tail string
	if o.Tail == nil {
		return tail
	}
	return *o.Tail
}

// WithTimestamps
func (o *LogOptions) WithTimestamps(value bool) *LogOptions {
	v := &value
	o.Timestamps = v
	return o
}

// GetTimestamps
func (o *LogOptions) GetTimestamps() bool {
	var timestamps bool
	if o.Timestamps == nil {
		return timestamps
	}
	return *o.Timestamps
}

// WithUntil
func (o *LogOptions) WithUntil(value string) *LogOptions {
	v := &value
	o.Until = v
	return o
}

// GetUntil
func (o *LogOptions) GetUntil() string {
	var until string
	if o.Until == nil {
		return until
	}
	return *o.Until
}
