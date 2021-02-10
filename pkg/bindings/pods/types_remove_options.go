package pods

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

	if o.Force != nil {
		params.Set("force", strconv.FormatBool(*o.Force))
	}

	return params, nil
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
