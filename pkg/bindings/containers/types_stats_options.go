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
func (o *StatsOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *StatsOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Stream != nil {
		params.Set("stream", strconv.FormatBool(*o.Stream))
	}

	return params, nil
}

// WithStream
func (o *StatsOptions) WithStream(value bool) *StatsOptions {
	v := &value
	o.Stream = v
	return o
}

// GetStream
func (o *StatsOptions) GetStream() bool {
	var stream bool
	if o.Stream == nil {
		return stream
	}
	return *o.Stream
}
