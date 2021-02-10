package generate

import (
	"net/url"
	"reflect"
	"strconv"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *KubeOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *KubeOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Service != nil {
		params.Set("service", strconv.FormatBool(*o.Service))
	}

	return params, nil
}

// WithService
func (o *KubeOptions) WithService(value bool) *KubeOptions {
	v := &value
	o.Service = v
	return o
}

// GetService
func (o *KubeOptions) GetService() bool {
	var service bool
	if o.Service == nil {
		return service
	}
	return *o.Service
}
