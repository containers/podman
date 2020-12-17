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

Created 2020-12-18 13:33:18.861536405 -0600 CST m=+0.000300026
*/

// Changed
func (o *RestoreOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *RestoreOptions) ToParams() (url.Values, error) {
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

// WithIgnoreRootfs
func (o *RestoreOptions) WithIgnoreRootfs(value bool) *RestoreOptions {
	v := &value
	o.IgnoreRootfs = v
	return o
}

// GetIgnoreRootfs
func (o *RestoreOptions) GetIgnoreRootfs() bool {
	var ignoreRootfs bool
	if o.IgnoreRootfs == nil {
		return ignoreRootfs
	}
	return *o.IgnoreRootfs
}

// WithIgnoreStaticIP
func (o *RestoreOptions) WithIgnoreStaticIP(value bool) *RestoreOptions {
	v := &value
	o.IgnoreStaticIP = v
	return o
}

// GetIgnoreStaticIP
func (o *RestoreOptions) GetIgnoreStaticIP() bool {
	var ignoreStaticIP bool
	if o.IgnoreStaticIP == nil {
		return ignoreStaticIP
	}
	return *o.IgnoreStaticIP
}

// WithIgnoreStaticMAC
func (o *RestoreOptions) WithIgnoreStaticMAC(value bool) *RestoreOptions {
	v := &value
	o.IgnoreStaticMAC = v
	return o
}

// GetIgnoreStaticMAC
func (o *RestoreOptions) GetIgnoreStaticMAC() bool {
	var ignoreStaticMAC bool
	if o.IgnoreStaticMAC == nil {
		return ignoreStaticMAC
	}
	return *o.IgnoreStaticMAC
}

// WithImportAchive
func (o *RestoreOptions) WithImportAchive(value string) *RestoreOptions {
	v := &value
	o.ImportAchive = v
	return o
}

// GetImportAchive
func (o *RestoreOptions) GetImportAchive() string {
	var importAchive string
	if o.ImportAchive == nil {
		return importAchive
	}
	return *o.ImportAchive
}

// WithKeep
func (o *RestoreOptions) WithKeep(value bool) *RestoreOptions {
	v := &value
	o.Keep = v
	return o
}

// GetKeep
func (o *RestoreOptions) GetKeep() bool {
	var keep bool
	if o.Keep == nil {
		return keep
	}
	return *o.Keep
}

// WithName
func (o *RestoreOptions) WithName(value string) *RestoreOptions {
	v := &value
	o.Name = v
	return o
}

// GetName
func (o *RestoreOptions) GetName() string {
	var name string
	if o.Name == nil {
		return name
	}
	return *o.Name
}

// WithTCPEstablished
func (o *RestoreOptions) WithTCPEstablished(value bool) *RestoreOptions {
	v := &value
	o.TCPEstablished = v
	return o
}

// GetTCPEstablished
func (o *RestoreOptions) GetTCPEstablished() bool {
	var tCPEstablished bool
	if o.TCPEstablished == nil {
		return tCPEstablished
	}
	return *o.TCPEstablished
}
