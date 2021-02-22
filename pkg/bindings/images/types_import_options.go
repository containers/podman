package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ImportOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *ImportOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithChanges
func (o *ImportOptions) WithChanges(value []string) *ImportOptions {
	v := &value
	o.Changes = v
	return o
}

// GetChanges
func (o *ImportOptions) GetChanges() []string {
	var changes []string
	if o.Changes == nil {
		return changes
	}
	return *o.Changes
}

// WithMessage
func (o *ImportOptions) WithMessage(value string) *ImportOptions {
	v := &value
	o.Message = v
	return o
}

// GetMessage
func (o *ImportOptions) GetMessage() string {
	var message string
	if o.Message == nil {
		return message
	}
	return *o.Message
}

// WithReference
func (o *ImportOptions) WithReference(value string) *ImportOptions {
	v := &value
	o.Reference = v
	return o
}

// GetReference
func (o *ImportOptions) GetReference() string {
	var reference string
	if o.Reference == nil {
		return reference
	}
	return *o.Reference
}

// WithURL
func (o *ImportOptions) WithURL(value string) *ImportOptions {
	v := &value
	o.URL = v
	return o
}

// GetURL
func (o *ImportOptions) GetURL() string {
	var uRL string
	if o.URL == nil {
		return uRL
	}
	return *o.URL
}
