package containers

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

// WithAll
func (o *ListOptions) WithAll(value bool) *ListOptions {
	v := &value
	o.All = v
	return o
}

// GetAll
func (o *ListOptions) GetAll() bool {
	var all bool
	if o.All == nil {
		return all
	}
	return *o.All
}

// WithExternal
func (o *ListOptions) WithExternal(value bool) *ListOptions {
	v := &value
	o.External = v
	return o
}

// GetExternal
func (o *ListOptions) GetExternal() bool {
	var external bool
	if o.External == nil {
		return external
	}
	return *o.External
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

// WithLast
func (o *ListOptions) WithLast(value int) *ListOptions {
	v := &value
	o.Last = v
	return o
}

// GetLast
func (o *ListOptions) GetLast() int {
	var last int
	if o.Last == nil {
		return last
	}
	return *o.Last
}

// WithNamespace
func (o *ListOptions) WithNamespace(value bool) *ListOptions {
	v := &value
	o.Namespace = v
	return o
}

// GetNamespace
func (o *ListOptions) GetNamespace() bool {
	var namespace bool
	if o.Namespace == nil {
		return namespace
	}
	return *o.Namespace
}

// WithSize
func (o *ListOptions) WithSize(value bool) *ListOptions {
	v := &value
	o.Size = v
	return o
}

// GetSize
func (o *ListOptions) GetSize() bool {
	var size bool
	if o.Size == nil {
		return size
	}
	return *o.Size
}

// WithSync
func (o *ListOptions) WithSync(value bool) *ListOptions {
	v := &value
	o.Sync = v
	return o
}

// GetSync
func (o *ListOptions) GetSync() bool {
	var sync bool
	if o.Sync == nil {
		return sync
	}
	return *o.Sync
}
