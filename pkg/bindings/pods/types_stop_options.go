package pods

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

// WithTimeout
func (o *StopOptions) WithTimeout(value int) *StopOptions {
	v := &value
	o.Timeout = v
	return o
}

// GetTimeout
func (o *StopOptions) GetTimeout() int {
	var timeout int
	if o.Timeout == nil {
		return timeout
	}
	return *o.Timeout
}
