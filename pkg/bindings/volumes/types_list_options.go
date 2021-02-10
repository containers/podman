package volumes

import (
	"net/url"
	"reflect"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ListOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *ListOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Filters != nil {
		lower := make(map[string][]string, len(o.Filters))
		for key, val := range o.Filters {
			lower[strings.ToLower(key)] = val
		}
		s, err := jsoniter.ConfigCompatibleWithStandardLibrary.MarshalToString(lower)
		if err != nil {
			return nil, err
		}
		params.Set("filters", s)
	}

	return params, nil
}

// WithFilters
func (o *ListOptions) WithFilters(value map[string][]string) *ListOptions {
	v := value
	o.Filters = v
	return o
}

// GetFilters
func (o *ListOptions) GetFilters() map[string][]string {
	var filters map[string][]string
	if o.Filters == nil {
		return filters
	}
	return o.Filters
}
