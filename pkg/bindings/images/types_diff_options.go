package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *DiffOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *DiffOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithParent
func (o *DiffOptions) WithParent(value string) *DiffOptions {
	v := &value
	o.Parent = v
	return o
}

// GetParent
func (o *DiffOptions) GetParent() string {
	var parent string
	if o.Parent == nil {
		return parent
	}
	return *o.Parent
}

// WithDiffType
func (o *DiffOptions) WithDiffType(value string) *DiffOptions {
	v := &value
	o.DiffType = v
	return o
}

// GetDiffType
func (o *DiffOptions) GetDiffType() string {
	var diffType string
	if o.DiffType == nil {
		return diffType
	}
	return *o.DiffType
}
