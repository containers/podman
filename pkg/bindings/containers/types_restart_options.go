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
func (o *RestartOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *RestartOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Timeout != nil {
		params.Set("timeout", strconv.FormatInt(int64(*o.Timeout), 10))
	}

	return params, nil
}

// WithTimeout
func (o *RestartOptions) WithTimeout(value int) *RestartOptions {
	v := &value
	o.Timeout = v
	return o
}

// GetTimeout
func (o *RestartOptions) GetTimeout() int {
	var timeout int
	if o.Timeout == nil {
		return timeout
	}
	return *o.Timeout
}
