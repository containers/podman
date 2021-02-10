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
func (o *StopOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *StopOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Ignore != nil {
		params.Set("ignore", strconv.FormatBool(*o.Ignore))
	}

	if o.Timeout != nil {
		params.Set("timeout", strconv.FormatInt(int64(*o.Timeout), 10))
	}

	return params, nil
}

// WithIgnore
func (o *StopOptions) WithIgnore(value bool) *StopOptions {
	v := &value
	o.Ignore = v
	return o
}

// GetIgnore
func (o *StopOptions) GetIgnore() bool {
	var ignore bool
	if o.Ignore == nil {
		return ignore
	}
	return *o.Ignore
}

// WithTimeout
func (o *StopOptions) WithTimeout(value uint) *StopOptions {
	v := &value
	o.Timeout = v
	return o
}

// GetTimeout
func (o *StopOptions) GetTimeout() uint {
	var timeout uint
	if o.Timeout == nil {
		return timeout
	}
	return *o.Timeout
}
