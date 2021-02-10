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
func (o *ExportOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *ExportOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Compress != nil {
		params.Set("compress", strconv.FormatBool(*o.Compress))
	}

	if o.Format != nil {
		params.Set("format", *o.Format)
	}

	return params, nil
}

// WithCompress
func (o *ExportOptions) WithCompress(value bool) *ExportOptions {
	v := &value
	o.Compress = v
	return o
}

// GetCompress
func (o *ExportOptions) GetCompress() bool {
	var compress bool
	if o.Compress == nil {
		return compress
	}
	return *o.Compress
}

// WithFormat
func (o *ExportOptions) WithFormat(value string) *ExportOptions {
	v := &value
	o.Format = v
	return o
}

// GetFormat
func (o *ExportOptions) GetFormat() string {
	var format string
	if o.Format == nil {
		return format
	}
	return *o.Format
}
