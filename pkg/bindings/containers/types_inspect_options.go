package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *InspectOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *InspectOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithSize
func (o *InspectOptions) WithSize(value bool) *InspectOptions {
	v := &value
	o.Size = v
	return o
}

// GetSize
func (o *InspectOptions) GetSize() bool {
	var size bool
	if o.Size == nil {
		return size
	}
	return *o.Size
}
