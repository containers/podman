package images

import (
	"net/url"
	"reflect"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *LoadOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *LoadOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Reference != nil {
		params.Set("reference", *o.Reference)
	}

	return params, nil
}

// WithReference
func (o *LoadOptions) WithReference(value string) *LoadOptions {
	v := &value
	o.Reference = v
	return o
}

// GetReference
func (o *LoadOptions) GetReference() string {
	var reference string
	if o.Reference == nil {
		return reference
	}
	return *o.Reference
}
