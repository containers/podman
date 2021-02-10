package containers

import (
	"net/url"
	"reflect"
	"strconv"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *RemoveOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *RemoveOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Ignore != nil {
		params.Set("ignore", strconv.FormatBool(*o.Ignore))
	}

	if o.Force != nil {
		params.Set("force", strconv.FormatBool(*o.Force))
	}

	if o.Volumes != nil {
		params.Set("volumes", strconv.FormatBool(*o.Volumes))
	}

	return params, nil
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
