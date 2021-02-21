package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *StatsOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *StatsOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithStream
func (o *StatsOptions) WithStream(value bool) *StatsOptions {
	v := &value
	o.Stream = v
	return o
}

// GetStream
func (o *StatsOptions) GetStream() bool {
	var stream bool
	if o.Stream == nil {
		return stream
	}
	return *o.Stream
}
