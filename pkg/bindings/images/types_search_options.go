package images

import (
	"net/url"
	"reflect"
	"strings"

	"github.com/containers/podman/v2/pkg/bindings/util"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *SearchOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *SearchOptions) ToParams() (url.Values, error) {
	params := url.Values{}
	if o == nil {
		return params, nil
	}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	s := reflect.ValueOf(o)
	if reflect.Ptr == s.Kind() {
		s = s.Elem()
	}
	sType := s.Type()
	for i := 0; i < s.NumField(); i++ {
		fieldName := sType.Field(i).Name
		if !o.Changed(fieldName) {
			continue
		}
		fieldName = strings.ToLower(fieldName)
		f := s.Field(i)
		if reflect.Ptr == f.Kind() {
			f = f.Elem()
		}
		switch {
		case util.IsSimpleType(f):
			params.Set(fieldName, util.SimpleTypeToParam(f))
		case f.Kind() == reflect.Slice:
			for i := 0; i < f.Len(); i++ {
				elem := f.Index(i)
				if util.IsSimpleType(elem) {
					params.Add(fieldName, util.SimpleTypeToParam(elem))
				} else {
					return nil, errors.New("slices must contain only simple types")
				}
			}
		case f.Kind() == reflect.Map:
			lowerCaseKeys := make(map[string][]string)
			iter := f.MapRange()
			for iter.Next() {
				lowerCaseKeys[iter.Key().Interface().(string)] = iter.Value().Interface().([]string)

			}
			s, err := json.MarshalToString(lowerCaseKeys)
			if err != nil {
				return nil, err
			}

			params.Set(fieldName, s)
		}

	}
	return params, nil
}

// WithAuthfile
func (o *SearchOptions) WithAuthfile(value string) *SearchOptions {
	v := &value
	o.Authfile = v
	return o
}

// GetAuthfile
func (o *SearchOptions) GetAuthfile() string {
	var authfile string
	if o.Authfile == nil {
		return authfile
	}
	return *o.Authfile
}

// WithFilters
func (o *SearchOptions) WithFilters(value map[string][]string) *SearchOptions {
	v := value
	o.Filters = v
	return o
}

// GetFilters
func (o *SearchOptions) GetFilters() map[string][]string {
	var filters map[string][]string
	if o.Filters == nil {
		return filters
	}
	return o.Filters
}

// WithLimit
func (o *SearchOptions) WithLimit(value int) *SearchOptions {
	v := &value
	o.Limit = v
	return o
}

// GetLimit
func (o *SearchOptions) GetLimit() int {
	var limit int
	if o.Limit == nil {
		return limit
	}
	return *o.Limit
}

// WithNoTrunc
func (o *SearchOptions) WithNoTrunc(value bool) *SearchOptions {
	v := &value
	o.NoTrunc = v
	return o
}

// GetNoTrunc
func (o *SearchOptions) GetNoTrunc() bool {
	var noTrunc bool
	if o.NoTrunc == nil {
		return noTrunc
	}
	return *o.NoTrunc
}

// WithSkipTLSVerify
func (o *SearchOptions) WithSkipTLSVerify(value bool) *SearchOptions {
	v := &value
	o.SkipTLSVerify = v
	return o
}

// GetSkipTLSVerify
func (o *SearchOptions) GetSkipTLSVerify() bool {
	var skipTLSVerify bool
	if o.SkipTLSVerify == nil {
		return skipTLSVerify
	}
	return *o.SkipTLSVerify
}

// WithListTags
func (o *SearchOptions) WithListTags(value bool) *SearchOptions {
	v := &value
	o.ListTags = v
	return o
}

// GetListTags
func (o *SearchOptions) GetListTags() bool {
	var listTags bool
	if o.ListTags == nil {
		return listTags
	}
	return *o.ListTags
}
