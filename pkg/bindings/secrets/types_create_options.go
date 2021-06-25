package secrets

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *CreateOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *CreateOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithName
func (o *CreateOptions) WithName(value string) *CreateOptions {
	v := &value
	o.Name = v
	return o
}

// GetName
func (o *CreateOptions) GetName() string {
	var name string
	if o.Name == nil {
		return name
	}
	return *o.Name
}

// WithDriver
func (o *CreateOptions) WithDriver(value string) *CreateOptions {
	v := &value
	o.Driver = v
	return o
}

// GetDriver
func (o *CreateOptions) GetDriver() string {
	var driver string
	if o.Driver == nil {
		return driver
	}
	return *o.Driver
}

// WithDriverOpts
func (o *CreateOptions) WithDriverOpts(value map[string]string) *CreateOptions {
	v := value
	o.DriverOpts = v
	return o
}

// GetDriverOpts
func (o *CreateOptions) GetDriverOpts() map[string]string {
	var driverOpts map[string]string
	if o.DriverOpts == nil {
		return driverOpts
	}
	return o.DriverOpts
}
