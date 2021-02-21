package containers

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

// WithIgnore
func (o *RemoveOptions) WithIgnore(value bool) *RemoveOptions {
	v := &value
	o.Ignore = v
	return o
}

// GetIgnore
func (o *RemoveOptions) GetIgnore() bool {
	var ignore bool
	if o.Ignore == nil {
		return ignore
	}
	return *o.Ignore
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

// WithVolumes
func (o *RemoveOptions) WithVolumes(value bool) *RemoveOptions {
	v := &value
	o.Volumes = v
	return o
}

// GetVolumes
func (o *RemoveOptions) GetVolumes() bool {
	var volumes bool
	if o.Volumes == nil {
		return volumes
	}
	return *o.Volumes
}
