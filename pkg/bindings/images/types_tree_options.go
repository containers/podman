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
func (o *TreeOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *TreeOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.WhatRequires != nil {
		params.Set("whatrequires", strconv.FormatBool(*o.WhatRequires))
	}

	return params, nil
}

// WithWhatRequires
func (o *TreeOptions) WithWhatRequires(value bool) *TreeOptions {
	v := &value
	o.WhatRequires = v
	return o
}

// GetWhatRequires
func (o *TreeOptions) GetWhatRequires() bool {
	var whatRequires bool
	if o.WhatRequires == nil {
		return whatRequires
	}
	return *o.WhatRequires
}
