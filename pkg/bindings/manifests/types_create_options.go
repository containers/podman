package manifests

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *CreateOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *CreateOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithAll
func (o *CreateOptions) WithAll(value bool) *CreateOptions {
	v := &value
	o.All = v
	return o
}

// GetAll
func (o *CreateOptions) GetAll() bool {
	var all bool
	if o.All == nil {
		return all
	}
	return *o.All
}
