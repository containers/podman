package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ExistsOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *ExistsOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithExternal
func (o *ExistsOptions) WithExternal(value bool) *ExistsOptions {
	v := &value
	o.External = v
	return o
}

// GetExternal
func (o *ExistsOptions) GetExternal() bool {
	var external bool
	if o.External == nil {
		return external
	}
	return *o.External
}
