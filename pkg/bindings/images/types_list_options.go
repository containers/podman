package images

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

// WithAll
func (o *ListOptions) WithAll(value bool) *ListOptions {
	v := &value
	o.All = v
	return o
}

// GetAll
func (o *ListOptions) GetAll() bool {
	var all bool
	if o.All == nil {
		return all
	}
	return *o.All
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
