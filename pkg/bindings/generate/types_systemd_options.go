package generate

import (
	"net/url"
	"reflect"
	"strconv"
)

/*
This file is generated automatically by go generate.  Do not edit.
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

	if o.UseName != nil {
		params.Set("usename", strconv.FormatBool(*o.UseName))
	}

	if o.New != nil {
		params.Set("new", strconv.FormatBool(*o.New))
	}

	if o.RestartPolicy != nil {
		params.Set("restartpolicy", *o.RestartPolicy)
	}

	if o.StopTimeout != nil {
		params.Set("stoptimeout", strconv.FormatInt(int64(*o.StopTimeout), 10))
	}

	if o.ContainerPrefix != nil {
		params.Set("containerprefix", *o.ContainerPrefix)
	}

	if o.PodPrefix != nil {
		params.Set("podprefix", *o.PodPrefix)
	}

	if o.Separator != nil {
		params.Set("separator", *o.Separator)
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
