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

Created 2020-12-18 13:33:18.714853285 -0600 CST m=+0.000319103
*/

// Changed
func (o *CheckpointOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *CheckpointOptions) ToParams() (url.Values, error) {
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

// WithExport
func (o *CheckpointOptions) WithExport(value string) *CheckpointOptions {
	v := &value
	o.Export = v
	return o
}

// GetExport
func (o *CheckpointOptions) GetExport() string {
	var export string
	if o.Export == nil {
		return export
	}
	return *o.Export
}

// WithIgnoreRootfs
func (o *CheckpointOptions) WithIgnoreRootfs(value bool) *CheckpointOptions {
	v := &value
	o.IgnoreRootfs = v
	return o
}

// GetIgnoreRootfs
func (o *CheckpointOptions) GetIgnoreRootfs() bool {
	var ignoreRootfs bool
	if o.IgnoreRootfs == nil {
		return ignoreRootfs
	}
	return *o.IgnoreRootfs
}

// WithKeep
func (o *CheckpointOptions) WithKeep(value bool) *CheckpointOptions {
	v := &value
	o.Keep = v
	return o
}

// GetKeep
func (o *CheckpointOptions) GetKeep() bool {
	var keep bool
	if o.Keep == nil {
		return keep
	}
	return *o.Keep
}

// WithLeaveRunning
func (o *CheckpointOptions) WithLeaveRunning(value bool) *CheckpointOptions {
	v := &value
	o.LeaveRunning = v
	return o
}

// GetLeaveRunning
func (o *CheckpointOptions) GetLeaveRunning() bool {
	var leaveRunning bool
	if o.LeaveRunning == nil {
		return leaveRunning
	}
	return *o.LeaveRunning
}

// WithTCPEstablished
func (o *CheckpointOptions) WithTCPEstablished(value bool) *CheckpointOptions {
	v := &value
	o.TCPEstablished = v
	return o
}

// GetTCPEstablished
func (o *CheckpointOptions) GetTCPEstablished() bool {
	var tCPEstablished bool
	if o.TCPEstablished == nil {
		return tCPEstablished
	}
	return *o.TCPEstablished
}
