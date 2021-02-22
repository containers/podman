package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *PullOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *PullOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithAllTags
func (o *PullOptions) WithAllTags(value bool) *PullOptions {
	v := &value
	o.AllTags = v
	return o
}

// GetAllTags
func (o *PullOptions) GetAllTags() bool {
	var allTags bool
	if o.AllTags == nil {
		return allTags
	}
	return *o.AllTags
}

// WithArch
func (o *PullOptions) WithArch(value string) *PullOptions {
	v := &value
	o.Arch = v
	return o
}

// GetArch
func (o *PullOptions) GetArch() string {
	var arch string
	if o.Arch == nil {
		return arch
	}
	return *o.Arch
}

// WithAuthfile
func (o *PullOptions) WithAuthfile(value string) *PullOptions {
	v := &value
	o.Authfile = v
	return o
}

// GetAuthfile
func (o *PullOptions) GetAuthfile() string {
	var authfile string
	if o.Authfile == nil {
		return authfile
	}
	return *o.Authfile
}

// WithOS
func (o *PullOptions) WithOS(value string) *PullOptions {
	v := &value
	o.OS = v
	return o
}

// GetOS
func (o *PullOptions) GetOS() string {
	var oS string
	if o.OS == nil {
		return oS
	}
	return *o.OS
}

// WithPassword
func (o *PullOptions) WithPassword(value string) *PullOptions {
	v := &value
	o.Password = v
	return o
}

// GetPassword
func (o *PullOptions) GetPassword() string {
	var password string
	if o.Password == nil {
		return password
	}
	return *o.Password
}

// WithQuiet
func (o *PullOptions) WithQuiet(value bool) *PullOptions {
	v := &value
	o.Quiet = v
	return o
}

// GetQuiet
func (o *PullOptions) GetQuiet() bool {
	var quiet bool
	if o.Quiet == nil {
		return quiet
	}
	return *o.Quiet
}

// WithSkipTLSVerify
func (o *PullOptions) WithSkipTLSVerify(value bool) *PullOptions {
	v := &value
	o.SkipTLSVerify = v
	return o
}

// GetSkipTLSVerify
func (o *PullOptions) GetSkipTLSVerify() bool {
	var skipTLSVerify bool
	if o.SkipTLSVerify == nil {
		return skipTLSVerify
	}
	return *o.SkipTLSVerify
}

// WithUsername
func (o *PullOptions) WithUsername(value string) *PullOptions {
	v := &value
	o.Username = v
	return o
}

// GetUsername
func (o *PullOptions) GetUsername() string {
	var username string
	if o.Username == nil {
		return username
	}
	return *o.Username
}

// WithVariant
func (o *PullOptions) WithVariant(value string) *PullOptions {
	v := &value
	o.Variant = v
	return o
}

// GetVariant
func (o *PullOptions) GetVariant() string {
	var variant string
	if o.Variant == nil {
		return variant
	}
	return *o.Variant
}
