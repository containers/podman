package generate

import (
	"net/url"
	"reflect"
	"strconv"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

/*
This file is generated automatically by go generate.  Do not edit.

Created 2020-12-17 09:35:23.663318384 -0600 CST m=+0.000158454
*/

// Changed
func (o *SystemdOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *SystemdOptions) ToParams() (url.Values, error) {
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
		}
	}
	return params, nil
}

// WithUseName
func (o *SystemdOptions) WithUseName(value bool) *SystemdOptions {
	v := &value
	o.UseName = v
	return o
}

// GetUseName
func (o *SystemdOptions) GetUseName() bool {
	var useName bool
	if o.UseName == nil {
		return useName
	}
	return *o.UseName
}

// WithNew
func (o *SystemdOptions) WithNew(value bool) *SystemdOptions {
	v := &value
	o.New = v
	return o
}

// GetNew
func (o *SystemdOptions) GetNew() bool {
	var new bool
	if o.New == nil {
		return new
	}
	return *o.New
}

// WithRestartPolicy
func (o *SystemdOptions) WithRestartPolicy(value string) *SystemdOptions {
	v := &value
	o.RestartPolicy = v
	return o
}

// GetRestartPolicy
func (o *SystemdOptions) GetRestartPolicy() string {
	var restartPolicy string
	if o.RestartPolicy == nil {
		return restartPolicy
	}
	return *o.RestartPolicy
}

// WithStopTimeout
func (o *SystemdOptions) WithStopTimeout(value uint) *SystemdOptions {
	v := &value
	o.StopTimeout = v
	return o
}

// GetStopTimeout
func (o *SystemdOptions) GetStopTimeout() uint {
	var stopTimeout uint
	if o.StopTimeout == nil {
		return stopTimeout
	}
	return *o.StopTimeout
}

// WithContainerPrefix
func (o *SystemdOptions) WithContainerPrefix(value string) *SystemdOptions {
	v := &value
	o.ContainerPrefix = v
	return o
}

// GetContainerPrefix
func (o *SystemdOptions) GetContainerPrefix() string {
	var containerPrefix string
	if o.ContainerPrefix == nil {
		return containerPrefix
	}
	return *o.ContainerPrefix
}

// WithPodPrefix
func (o *SystemdOptions) WithPodPrefix(value string) *SystemdOptions {
	v := &value
	o.PodPrefix = v
	return o
}

// GetPodPrefix
func (o *SystemdOptions) GetPodPrefix() string {
	var podPrefix string
	if o.PodPrefix == nil {
		return podPrefix
	}
	return *o.PodPrefix
}

// WithSeparator
func (o *SystemdOptions) WithSeparator(value string) *SystemdOptions {
	v := &value
	o.Separator = v
	return o
}

// GetSeparator
func (o *SystemdOptions) GetSeparator() string {
	var separator string
	if o.Separator == nil {
		return separator
	}
	return *o.Separator
}
