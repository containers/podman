package images

import (
	"net/url"
	"reflect"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *TagOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *TagOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	return params, nil
}
