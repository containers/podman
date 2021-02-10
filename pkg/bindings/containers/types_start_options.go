package containers

import (
	"net/url"
	"reflect"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *StartOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *StartOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.DetachKeys != nil {
		params.Set("detachkeys", *o.DetachKeys)
	}

	return params, nil
}

// WithDetachKeys
func (o *StartOptions) WithDetachKeys(value string) *StartOptions {
	v := &value
	o.DetachKeys = v
	return o
}

// GetDetachKeys
func (o *StartOptions) GetDetachKeys() string {
	var detachKeys string
	if o.DetachKeys == nil {
		return detachKeys
	}
	return *o.DetachKeys
}
