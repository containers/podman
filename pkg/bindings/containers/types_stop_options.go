package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *StopOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *StopOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithIgnore
func (o *StopOptions) WithIgnore(value bool) *StopOptions {
	v := &value
	o.Ignore = v
	return o
}

// GetIgnore
func (o *StopOptions) GetIgnore() bool {
	var ignore bool
	if o.Ignore == nil {
		return ignore
	}
	return *o.Ignore
}

// WithTimeout
func (o *StopOptions) WithTimeout(value uint) *StopOptions {
	v := &value
	o.Timeout = v
	return o
}

// GetTimeout
func (o *StopOptions) GetTimeout() uint {
	var timeout uint
	if o.Timeout == nil {
		return timeout
	}
	return *o.Timeout
}
