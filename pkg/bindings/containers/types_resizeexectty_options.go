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
func (o *ResizeExecTTYOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *ResizeExecTTYOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Height != nil {
		params.Set("height", strconv.FormatInt(int64(*o.Height), 10))
	}

	if o.Width != nil {
		params.Set("width", strconv.FormatInt(int64(*o.Width), 10))
	}

	return params, nil
}

// WithHeight
func (o *ResizeExecTTYOptions) WithHeight(value int) *ResizeExecTTYOptions {
	v := &value
	o.Height = v
	return o
}

// GetHeight
func (o *ResizeExecTTYOptions) GetHeight() int {
	var height int
	if o.Height == nil {
		return height
	}
	return *o.Height
}

// WithWidth
func (o *ResizeExecTTYOptions) WithWidth(value int) *ResizeExecTTYOptions {
	v := &value
	o.Width = v
	return o
}

// GetWidth
func (o *ResizeExecTTYOptions) GetWidth() int {
	var width int
	if o.Width == nil {
		return width
	}
	return *o.Width
}
