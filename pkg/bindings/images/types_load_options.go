package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *LoadOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *LoadOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithReference
func (o *LoadOptions) WithReference(value string) *LoadOptions {
	v := &value
	o.Reference = v
	return o
}

// GetReference
func (o *LoadOptions) GetReference() string {
	var reference string
	if o.Reference == nil {
		return reference
	}
	return *o.Reference
}
