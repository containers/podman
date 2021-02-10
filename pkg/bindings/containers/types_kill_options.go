package containers

import (
	"net/url"
	"reflect"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *KillOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *KillOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Signal != nil {
		params.Set("signal", *o.Signal)
	}

	return params, nil
}

// WithSignal
func (o *KillOptions) WithSignal(value string) *KillOptions {
	v := &value
	o.Signal = v
	return o
}

// GetSignal
func (o *KillOptions) GetSignal() string {
	var signal string
	if o.Signal == nil {
		return signal
	}
	return *o.Signal
}
