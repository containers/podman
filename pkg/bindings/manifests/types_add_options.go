package manifests

import (
	"net/url"
	"reflect"
	"strconv"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

/*
This file is generated automatically by go generate.  Do not edit.

Created 2020-12-18 15:57:55.92237379 -0600 CST m=+0.000150701
*/

// Changed
func (o *AddOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *AddOptions) ToParams() (url.Values, error) {
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

// WithAll
func (o *AddOptions) WithAll(value bool) *AddOptions {
	v := &value
	o.All = v
	return o
}

// GetAll
func (o *AddOptions) GetAll() bool {
	var all bool
	if o.All == nil {
		return all
	}
	return *o.All
}

// WithAnnotation
func (o *AddOptions) WithAnnotation(value map[string]string) *AddOptions {
	v := value
	o.Annotation = v
	return o
}

// GetAnnotation
func (o *AddOptions) GetAnnotation() map[string]string {
	var annotation map[string]string
	if o.Annotation == nil {
		return annotation
	}
	return o.Annotation
}

// WithArch
func (o *AddOptions) WithArch(value string) *AddOptions {
	v := &value
	o.Arch = v
	return o
}

// GetArch
func (o *AddOptions) GetArch() string {
	var arch string
	if o.Arch == nil {
		return arch
	}
	return *o.Arch
}

// WithFeatures
func (o *AddOptions) WithFeatures(value []string) *AddOptions {
	v := value
	o.Features = v
	return o
}

// GetFeatures
func (o *AddOptions) GetFeatures() []string {
	var features []string
	if o.Features == nil {
		return features
	}
	return o.Features
}

// WithImages
func (o *AddOptions) WithImages(value []string) *AddOptions {
	v := value
	o.Images = v
	return o
}

// GetImages
func (o *AddOptions) GetImages() []string {
	var images []string
	if o.Images == nil {
		return images
	}
	return o.Images
}

// WithOS
func (o *AddOptions) WithOS(value string) *AddOptions {
	v := &value
	o.OS = v
	return o
}

// GetOS
func (o *AddOptions) GetOS() string {
	var oS string
	if o.OS == nil {
		return oS
	}
	return *o.OS
}

// WithOSVersion
func (o *AddOptions) WithOSVersion(value string) *AddOptions {
	v := &value
	o.OSVersion = v
	return o
}

// GetOSVersion
func (o *AddOptions) GetOSVersion() string {
	var oSVersion string
	if o.OSVersion == nil {
		return oSVersion
	}
	return *o.OSVersion
}

// WithVariant
func (o *AddOptions) WithVariant(value string) *AddOptions {
	v := &value
	o.Variant = v
	return o
}

// GetVariant
func (o *AddOptions) GetVariant() string {
	var variant string
	if o.Variant == nil {
		return variant
	}
	return *o.Variant
}
