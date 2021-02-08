package manifests

import (
	"errors"
	"net/url"
	"reflect"
	"strings"

	"github.com/containers/podman/v2/pkg/bindings/util"
	jsoniter "github.com/json-iterator/go"
)

/*
This file is generated automatically by go generate.  Do not edit.
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
