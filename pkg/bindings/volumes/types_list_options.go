package volumes

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ListOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *ListOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithFilters
func (o *ListOptions) WithFilters(value map[string][]string) *ListOptions {
	v := value
	o.Filters = v
	return o
}

// GetFilters
func (o *ListOptions) GetFilters() map[string][]string {
	var filters map[string][]string
	if o.Filters == nil {
		return filters
	}
	return o.Filters
}
