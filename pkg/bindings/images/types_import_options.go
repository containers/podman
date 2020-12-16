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

Created 2020-12-15 15:22:47.943587389 -0600 CST m=+0.000238222
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
