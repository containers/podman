package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *PruneOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *PruneOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithAll
func (o *PruneOptions) WithAll(value bool) *PruneOptions {
	v := &value
	o.All = v
	return o
}

// GetAll
func (o *PruneOptions) GetAll() bool {
	var all bool
	if o.All == nil {
		return all
	}
	return *o.All
}

// WithFilters
func (o *PruneOptions) WithFilters(value map[string][]string) *PruneOptions {
	v := value
	o.Filters = v
	return o
}

// GetFilters
func (o *PruneOptions) GetFilters() map[string][]string {
	var filters map[string][]string
	if o.Filters == nil {
		return filters
	}
	return o.Filters
}
