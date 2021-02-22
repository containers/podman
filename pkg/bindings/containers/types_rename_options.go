package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *RenameOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *RenameOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithName
func (o *RenameOptions) WithName(value string) *RenameOptions {
	v := &value
	o.Name = v
	return o
}

// GetName
func (o *RenameOptions) GetName() string {
	var name string
	if o.Name == nil {
		return name
	}
	return *o.Name
}
