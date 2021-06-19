package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *CopyOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *CopyOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithChown
func (o *CopyOptions) WithChown(value bool) *CopyOptions {
	v := &value
	o.Chown = v
	return o
}

// GetChown
func (o *CopyOptions) GetChown() bool {
	var chown bool
	if o.Chown == nil {
		return chown
	}
	return *o.Chown
}

// WithRename
func (o *CopyOptions) WithRename(value map[string]string) *CopyOptions {
	v := value
	o.Rename = v
	return o
}

// GetRename
func (o *CopyOptions) GetRename() map[string]string {
	var rename map[string]string
	if o.Rename == nil {
		return rename
	}
	return o.Rename
}
