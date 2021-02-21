package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ExportOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *ExportOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithCompress
func (o *ExportOptions) WithCompress(value bool) *ExportOptions {
	v := &value
	o.Compress = v
	return o
}

// GetCompress
func (o *ExportOptions) GetCompress() bool {
	var compress bool
	if o.Compress == nil {
		return compress
	}
	return *o.Compress
}

// WithFormat
func (o *ExportOptions) WithFormat(value string) *ExportOptions {
	v := &value
	o.Format = v
	return o
}

// GetFormat
func (o *ExportOptions) GetFormat() string {
	var format string
	if o.Format == nil {
		return format
	}
	return *o.Format
}
