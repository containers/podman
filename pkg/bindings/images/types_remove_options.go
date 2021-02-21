package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *RemoveOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *RemoveOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithAll
func (o *RemoveOptions) WithAll(value bool) *RemoveOptions {
	v := &value
	o.All = v
	return o
}

// GetAll
func (o *RemoveOptions) GetAll() bool {
	var all bool
	if o.All == nil {
		return all
	}
	return *o.All
}

// WithForce
func (o *RemoveOptions) WithForce(value bool) *RemoveOptions {
	v := &value
	o.Force = v
	return o
}

// GetForce
func (o *RemoveOptions) GetForce() bool {
	var force bool
	if o.Force == nil {
		return force
	}
	return *o.Force
}
