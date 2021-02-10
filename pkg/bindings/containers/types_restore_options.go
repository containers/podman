package containers

import (
	"net/url"
	"reflect"
	"strconv"
)

/*
This file is generated automatically by go generate.  Do not edit.
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

	if o.IgnoreRootfs != nil {
		params.Set("ignorerootfs", strconv.FormatBool(*o.IgnoreRootfs))
	}

	if o.IgnoreStaticIP != nil {
		params.Set("ignorestaticip", strconv.FormatBool(*o.IgnoreStaticIP))
	}

	if o.IgnoreStaticMAC != nil {
		params.Set("ignorestaticmac", strconv.FormatBool(*o.IgnoreStaticMAC))
	}

	if o.ImportAchive != nil {
		params.Set("importachive", *o.ImportAchive)
	}

	if o.Keep != nil {
		params.Set("keep", strconv.FormatBool(*o.Keep))
	}

	if o.Name != nil {
		params.Set("name", *o.Name)
	}

	if o.TCPEstablished != nil {
		params.Set("tcpestablished", strconv.FormatBool(*o.TCPEstablished))
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
