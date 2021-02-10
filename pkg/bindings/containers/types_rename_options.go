package containers

import (
	"net/url"
	"reflect"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *RenameOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *RenameOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Name != nil {
		params.Set("name", *o.Name)
	}

	return params, nil
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
