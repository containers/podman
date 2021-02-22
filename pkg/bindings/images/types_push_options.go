package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *PushOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *PushOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithAll
func (o *PushOptions) WithAll(value bool) *PushOptions {
	v := &value
	o.All = v
	return o
}

// GetAll
func (o *PushOptions) GetAll() bool {
	var all bool
	if o.All == nil {
		return all
	}
	return *o.All
}

// WithAuthfile
func (o *PushOptions) WithAuthfile(value string) *PushOptions {
	v := &value
	o.Authfile = v
	return o
}

// GetAuthfile
func (o *PushOptions) GetAuthfile() string {
	var authfile string
	if o.Authfile == nil {
		return authfile
	}
	return *o.Authfile
}

// WithCompress
func (o *PushOptions) WithCompress(value bool) *PushOptions {
	v := &value
	o.Compress = v
	return o
}

// GetCompress
func (o *PushOptions) GetCompress() bool {
	var compress bool
	if o.Compress == nil {
		return compress
	}
	return *o.Compress
}

// WithFormat
func (o *PushOptions) WithFormat(value string) *PushOptions {
	v := &value
	o.Format = v
	return o
}

// GetFormat
func (o *PushOptions) GetFormat() string {
	var format string
	if o.Format == nil {
		return format
	}
	return *o.Format
}

// WithPassword
func (o *PushOptions) WithPassword(value string) *PushOptions {
	v := &value
	o.Password = v
	return o
}

// GetPassword
func (o *PushOptions) GetPassword() string {
	var password string
	if o.Password == nil {
		return password
	}
	return *o.Password
}

// WithSkipTLSVerify
func (o *PushOptions) WithSkipTLSVerify(value bool) *PushOptions {
	v := &value
	o.SkipTLSVerify = v
	return o
}

// GetSkipTLSVerify
func (o *PushOptions) GetSkipTLSVerify() bool {
	var skipTLSVerify bool
	if o.SkipTLSVerify == nil {
		return skipTLSVerify
	}
	return *o.SkipTLSVerify
}

// WithUsername
func (o *PushOptions) WithUsername(value string) *PushOptions {
	v := &value
	o.Username = v
	return o
}

// GetUsername
func (o *PushOptions) GetUsername() string {
	var username string
	if o.Username == nil {
		return username
	}
	return *o.Username
}
