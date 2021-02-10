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
func (o *InspectOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *InspectOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Size != nil {
		params.Set("size", strconv.FormatBool(*o.Size))
	}

	return params, nil
}

// WithSize
func (o *InspectOptions) WithSize(value bool) *InspectOptions {
	v := &value
	o.Size = v
	return o
}

// GetSize
func (o *InspectOptions) GetSize() bool {
	var size bool
	if o.Size == nil {
		return size
	}
	return *o.Size
}
