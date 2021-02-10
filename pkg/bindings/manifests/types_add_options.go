package manifests

import (
	"net/url"
	"reflect"
	"strconv"
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

	if o.All != nil {
		params.Set("all", strconv.FormatBool(*o.All))
	}

	if o.Annotation != nil {
		panic("*** GENERATOR DOESN'T IMPLEMENT THIS YET ***")
	}

	if o.Arch != nil {
		params.Set("arch", *o.Arch)
	}

	if o.Features != nil {
		for _, val := range o.Features {
			params.Add("features", val)
		}
	}

	if o.Images != nil {
		for _, val := range o.Images {
			params.Add("images", val)
		}
	}

	if o.OS != nil {
		params.Set("os", *o.OS)
	}

	if o.OSVersion != nil {
		params.Set("osversion", *o.OSVersion)
	}

	if o.Variant != nil {
		params.Set("variant", *o.Variant)
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
