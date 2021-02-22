package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *GetOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *GetOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithSize
func (o *GetOptions) WithSize(value bool) *GetOptions {
	v := &value
	o.Size = v
	return o
}

// GetSize
func (o *GetOptions) GetSize() bool {
	var size bool
	if o.Size == nil {
		return size
	}
	return *o.Size
}
