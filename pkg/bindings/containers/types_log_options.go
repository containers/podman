package containers

import (
	"net/url"
	"reflect"
	"strconv"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

/*
This file is generated automatically by go generate.  Do not edit.

Created 2020-12-18 13:33:18.273264471 -0600 CST m=+0.000274536
*/

// Changed
func (o *LogOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *LogOptions) ToParams() (url.Values, error) {
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
		case reflect.Uint, reflect.Uint64:
			// f.Uint() is always an uint64
			params.Set(fieldName, strconv.FormatUint(f.Uint(), 10))
		case reflect.Slice:
			typ := reflect.TypeOf(f.Interface()).Elem()
			switch typ.Kind() {
			case reflect.String:
				sl := f.Slice(0, f.Len())
				s, ok := sl.Interface().([]string)
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
		}
	}
	return params, nil
}

// WithFollow
func (o *LogOptions) WithFollow(value bool) *LogOptions {
	v := &value
	o.Follow = v
	return o
}

// GetFollow
func (o *LogOptions) GetFollow() bool {
	var follow bool
	if o.Follow == nil {
		return follow
	}
	return *o.Follow
}

// WithSince
func (o *LogOptions) WithSince(value string) *LogOptions {
	v := &value
	o.Since = v
	return o
}

// GetSince
func (o *LogOptions) GetSince() string {
	var since string
	if o.Since == nil {
		return since
	}
	return *o.Since
}

// WithStderr
func (o *LogOptions) WithStderr(value bool) *LogOptions {
	v := &value
	o.Stderr = v
	return o
}

// GetStderr
func (o *LogOptions) GetStderr() bool {
	var stderr bool
	if o.Stderr == nil {
		return stderr
	}
	return *o.Stderr
}

// WithStdout
func (o *LogOptions) WithStdout(value bool) *LogOptions {
	v := &value
	o.Stdout = v
	return o
}

// GetStdout
func (o *LogOptions) GetStdout() bool {
	var stdout bool
	if o.Stdout == nil {
		return stdout
	}
	return *o.Stdout
}

// WithTail
func (o *LogOptions) WithTail(value string) *LogOptions {
	v := &value
	o.Tail = v
	return o
}

// GetTail
func (o *LogOptions) GetTail() string {
	var tail string
	if o.Tail == nil {
		return tail
	}
	return *o.Tail
}

// WithTimestamps
func (o *LogOptions) WithTimestamps(value bool) *LogOptions {
	v := &value
	o.Timestamps = v
	return o
}

// GetTimestamps
func (o *LogOptions) GetTimestamps() bool {
	var timestamps bool
	if o.Timestamps == nil {
		return timestamps
	}
	return *o.Timestamps
}

// WithUntil
func (o *LogOptions) WithUntil(value string) *LogOptions {
	v := &value
	o.Until = v
	return o
}

// GetUntil
func (o *LogOptions) GetUntil() string {
	var until string
	if o.Until == nil {
		return until
	}
	return *o.Until
}
