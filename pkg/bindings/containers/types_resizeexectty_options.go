package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ResizeExecTTYOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *ResizeExecTTYOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
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
