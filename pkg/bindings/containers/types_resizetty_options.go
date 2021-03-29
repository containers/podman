package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ResizeTTYOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *ResizeTTYOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithHeight
func (o *ResizeTTYOptions) WithHeight(value int) *ResizeTTYOptions {
	v := &value
	o.Height = v
	return o
}

// GetHeight
func (o *ResizeTTYOptions) GetHeight() int {
	var height int
	if o.Height == nil {
		return height
	}
	return *o.Height
}

// WithWidth
func (o *ResizeTTYOptions) WithWidth(value int) *ResizeTTYOptions {
	v := &value
	o.Width = v
	return o
}

// GetWidth
func (o *ResizeTTYOptions) GetWidth() int {
	var width int
	if o.Width == nil {
		return width
	}
	return *o.Width
}

// WithRunning
func (o *ResizeTTYOptions) WithRunning(value bool) *ResizeTTYOptions {
	v := &value
	o.Running = v
	return o
}

// GetRunning
func (o *ResizeTTYOptions) GetRunning() bool {
	var running bool
	if o.Running == nil {
		return running
	}
	return *o.Running
}
