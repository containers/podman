package images

import (
	"net/url"
	"reflect"
	"strconv"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *GetOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *GetOptions) ToParams() (url.Values, error) {
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
func (o *GetOptions) WithSize(value bool) *GetOptions {
	v := &value
	o.Size = v
	return o
}

// GetSize
func (o *GetOptions) GetSize() bool {
	var size bool
	if o.Size == nil {
		return size
	}
	return *o.Size
}
