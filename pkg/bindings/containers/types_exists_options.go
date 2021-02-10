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
func (o *ExistsOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *ExistsOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.External != nil {
		params.Set("external", strconv.FormatBool(*o.External))
	}

	return params, nil
}

// WithExternal
func (o *ExistsOptions) WithExternal(value bool) *ExistsOptions {
	v := &value
	o.External = v
	return o
}

// GetExternal
func (o *ExistsOptions) GetExternal() bool {
	var external bool
	if o.External == nil {
		return external
	}
	return *o.External
}
