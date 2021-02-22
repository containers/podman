package generate

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *KubeOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *KubeOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithService
func (o *KubeOptions) WithService(value bool) *KubeOptions {
	v := &value
	o.Service = v
	return o
}

// GetService
func (o *KubeOptions) GetService() bool {
	var service bool
	if o.Service == nil {
		return service
	}
	return *o.Service
}
