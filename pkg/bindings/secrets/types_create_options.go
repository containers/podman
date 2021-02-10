package secrets

import (
	"net/url"
	"reflect"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *CreateOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *CreateOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Driver != nil {
		params.Set("driver", *o.Driver)
	}

	if o.Name != nil {
		params.Set("name", *o.Name)
	}

	return params, nil
}

// WithDriver
func (o *CreateOptions) WithDriver(value string) *CreateOptions {
	v := &value
	o.Driver = v
	return o
}

// GetDriver
func (o *CreateOptions) GetDriver() string {
	var driver string
	if o.Driver == nil {
		return driver
	}
	return *o.Driver
}

// WithName
func (o *CreateOptions) WithName(value string) *CreateOptions {
	v := &value
	o.Name = v
	return o
}

// GetName
func (o *CreateOptions) GetName() string {
	var name string
	if o.Name == nil {
		return name
	}
	return *o.Name
}
