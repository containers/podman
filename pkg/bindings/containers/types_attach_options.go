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
func (o *AttachOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *AttachOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.DetachKeys != nil {
		params.Set("detachkeys", *o.DetachKeys)
	}

	if o.Logs != nil {
		params.Set("logs", strconv.FormatBool(*o.Logs))
	}

	if o.Stream != nil {
		params.Set("stream", strconv.FormatBool(*o.Stream))
	}

	return params, nil
}

// WithDetachKeys
func (o *AttachOptions) WithDetachKeys(value string) *AttachOptions {
	v := &value
	o.DetachKeys = v
	return o
}

// GetDetachKeys
func (o *AttachOptions) GetDetachKeys() string {
	var detachKeys string
	if o.DetachKeys == nil {
		return detachKeys
	}
	return *o.DetachKeys
}

// WithLogs
func (o *AttachOptions) WithLogs(value bool) *AttachOptions {
	v := &value
	o.Logs = v
	return o
}

// GetLogs
func (o *AttachOptions) GetLogs() bool {
	var logs bool
	if o.Logs == nil {
		return logs
	}
	return *o.Logs
}

// WithStream
func (o *AttachOptions) WithStream(value bool) *AttachOptions {
	v := &value
	o.Stream = v
	return o
}

// GetStream
func (o *AttachOptions) GetStream() bool {
	var stream bool
	if o.Stream == nil {
		return stream
	}
	return *o.Stream
}
