package network

import (
	"net/url"
	"reflect"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ConnectOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *ConnectOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Aliases != nil {
		for _, val := range o.Aliases {
			params.Add("aliases", val)
		}
	}

	return params, nil
}

// WithAliases
func (o *ConnectOptions) WithAliases(value []string) *ConnectOptions {
	v := value
	o.Aliases = v
	return o
}

// GetAliases
func (o *ConnectOptions) GetAliases() []string {
	var aliases []string
	if o.Aliases == nil {
		return aliases
	}
	return o.Aliases
}
