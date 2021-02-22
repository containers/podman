package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *TopOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *TopOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithDescriptors
func (o *TopOptions) WithDescriptors(value []string) *TopOptions {
	v := &value
	o.Descriptors = v
	return o
}

// GetDescriptors
func (o *TopOptions) GetDescriptors() []string {
	var descriptors []string
	if o.Descriptors == nil {
		return descriptors
	}
	return *o.Descriptors
}
