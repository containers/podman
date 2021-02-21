package pods

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *StatsOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *StatsOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithAll
func (o *StatsOptions) WithAll(value bool) *StatsOptions {
	v := &value
	o.All = v
	return o
}

// GetAll
func (o *StatsOptions) GetAll() bool {
	var all bool
	if o.All == nil {
		return all
	}
	return *o.All
}
