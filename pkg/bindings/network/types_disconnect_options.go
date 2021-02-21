package network

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *DisconnectOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *DisconnectOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithForce
func (o *DisconnectOptions) WithForce(value bool) *DisconnectOptions {
	v := &value
	o.Force = v
	return o
}

// GetForce
func (o *DisconnectOptions) GetForce() bool {
	var force bool
	if o.Force == nil {
		return force
	}
	return *o.Force
}
