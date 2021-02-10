package images

import (
	"net/url"
	"reflect"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ImportOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *ImportOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Changes != nil {
		for _, val := range o.Changes {
			params.Add("changes", val)
		}
	}

	if o.Message != nil {
		params.Set("message", *o.Message)
	}

	if o.Reference != nil {
		params.Set("reference", *o.Reference)
	}

	if o.URL != nil {
		params.Set("url", *o.URL)
	}

	return params, nil
}

// WithChanges
func (o *ImportOptions) WithChanges(value []string) *ImportOptions {
	v := value
	o.Changes = v
	return o
}

// GetChanges
func (o *ImportOptions) GetChanges() []string {
	var changes []string
	if o.Changes == nil {
		return changes
	}
	return o.Changes
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
