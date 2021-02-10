package manifests

import (
	"net/url"
	"reflect"
	"strconv"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *CreateOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *CreateOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.All != nil {
		params.Set("all", strconv.FormatBool(*o.All))
	}

	return params, nil
}

// WithAll
func (o *CreateOptions) WithAll(value bool) *CreateOptions {
	v := &value
	o.All = v
	return o
}

// GetAll
func (o *CreateOptions) GetAll() bool {
	var all bool
	if o.All == nil {
		return all
	}
	return *o.All
}
