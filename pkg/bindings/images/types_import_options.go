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
