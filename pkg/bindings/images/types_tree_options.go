package images

import (
	"net/url"
	"reflect"
	"strconv"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

/*
This file is generated automatically by go generate.  Do not edit.

Created 2020-12-15 15:22:46.954579136 -0600 CST m=+0.000248704
*/

// Changed
func (o *TreeOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *TreeOptions) ToParams() (url.Values, error) {
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
		f := s.Field(i)
		if reflect.Ptr == f.Kind() {
			f = f.Elem()
		}
		switch f.Kind() {
		case reflect.Bool:
			params.Set(fieldName, strconv.FormatBool(f.Bool()))
		case reflect.String:
			params.Set(fieldName, f.String())
		case reflect.Int, reflect.Int64:
			// f.Int() is always an int64
			params.Set(fieldName, strconv.FormatInt(f.Int(), 10))
		case reflect.Slice:
			typ := reflect.TypeOf(f.Interface()).Elem()
			slice := reflect.MakeSlice(reflect.SliceOf(typ), f.Len(), f.Cap())
			switch typ.Kind() {
			case reflect.String:
				s, ok := slice.Interface().([]string)
				if !ok {
					return nil, errors.New("failed to convert to string slice")
				}
				for _, val := range s {
					params.Add(fieldName, val)
				}
			default:
				return nil, errors.Errorf("unknown slice type %s", f.Kind().String())
			}
		case reflect.Map:
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
		default:
			return nil, errors.Errorf("unknown type %s", f.Kind().String())
		}
	}
	return params, nil
}

// WithWhatRequires
func (o *TreeOptions) WithWhatRequires(value bool) *TreeOptions {
	v := &value
	o.WhatRequires = v
	return o
}

// GetWhatRequires
func (o *TreeOptions) GetWhatRequires() bool {
	var whatRequires bool
	if o.WhatRequires == nil {
		return whatRequires
	}
	return *o.WhatRequires
}
