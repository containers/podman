package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *RestartOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *RestartOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithTimeout
func (o *RestartOptions) WithTimeout(value int) *RestartOptions {
	v := &value
	o.Timeout = v
	return o
}

// GetTimeout
func (o *RestartOptions) GetTimeout() int {
	var timeout int
	if o.Timeout == nil {
		return timeout
	}
	return *o.Timeout
}
