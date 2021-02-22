package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *TreeOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *TreeOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithWhatRequires
func (o *TreeOptions) WithWhatRequires(value bool) *TreeOptions {
	v := &value
	o.WhatRequires = v
	return o
}

// GetWhatRequires
func (o *TreeOptions) GetWhatRequires() bool {
	var whatRequires bool
	if o.WhatRequires == nil {
		return whatRequires
	}
	return *o.WhatRequires
}
