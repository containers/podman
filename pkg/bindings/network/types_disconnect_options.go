package network

import (
	"net/url"
	"reflect"
	"strconv"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *DisconnectOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *DisconnectOptions) ToParams() (url.Values, error) {
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
