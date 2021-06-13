package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *DiffOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *DiffOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithOtherImg
func (o *DiffOptions) WithOtherImg(value string) *DiffOptions {
	v := &value
	o.OtherImg = v
	return o
}

// GetOtherImg
func (o *DiffOptions) GetOtherImg() string {
	var otherImg string
	if o.OtherImg == nil {
		return otherImg
	}
	return *o.OtherImg
}
