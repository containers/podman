package containers

import (
	"net/url"
	"reflect"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *TopOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *TopOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Descriptors != nil {
		for _, val := range o.Descriptors {
			params.Add("descriptors", val)
		}
	}

	return params, nil
}

// WithDescriptors
func (o *TopOptions) WithDescriptors(value []string) *TopOptions {
	v := value
	o.Descriptors = v
	return o
}

// GetDescriptors
func (o *TopOptions) GetDescriptors() []string {
	var descriptors []string
	if o.Descriptors == nil {
		return descriptors
	}
	return o.Descriptors
}
