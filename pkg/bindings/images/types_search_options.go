package images

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *SearchOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *SearchOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithAuthfile
func (o *SearchOptions) WithAuthfile(value string) *SearchOptions {
	v := &value
	o.Authfile = v
	return o
}

// GetAuthfile
func (o *SearchOptions) GetAuthfile() string {
	var authfile string
	if o.Authfile == nil {
		return authfile
	}
	return *o.Authfile
}

// WithFilters
func (o *SearchOptions) WithFilters(value map[string][]string) *SearchOptions {
	v := value
	o.Filters = v
	return o
}

// GetFilters
func (o *SearchOptions) GetFilters() map[string][]string {
	var filters map[string][]string
	if o.Filters == nil {
		return filters
	}
	return o.Filters
}

// WithLimit
func (o *SearchOptions) WithLimit(value int) *SearchOptions {
	v := &value
	o.Limit = v
	return o
}

// GetLimit
func (o *SearchOptions) GetLimit() int {
	var limit int
	if o.Limit == nil {
		return limit
	}
	return *o.Limit
}

// WithNoTrunc
func (o *SearchOptions) WithNoTrunc(value bool) *SearchOptions {
	v := &value
	o.NoTrunc = v
	return o
}

// GetNoTrunc
func (o *SearchOptions) GetNoTrunc() bool {
	var noTrunc bool
	if o.NoTrunc == nil {
		return noTrunc
	}
	return *o.NoTrunc
}

// WithSkipTLSVerify
func (o *SearchOptions) WithSkipTLSVerify(value bool) *SearchOptions {
	v := &value
	o.SkipTLSVerify = v
	return o
}

// GetSkipTLSVerify
func (o *SearchOptions) GetSkipTLSVerify() bool {
	var skipTLSVerify bool
	if o.SkipTLSVerify == nil {
		return skipTLSVerify
	}
	return *o.SkipTLSVerify
}

// WithListTags
func (o *SearchOptions) WithListTags(value bool) *SearchOptions {
	v := &value
	o.ListTags = v
	return o
}

// GetListTags
func (o *SearchOptions) GetListTags() bool {
	var listTags bool
	if o.ListTags == nil {
		return listTags
	}
	return *o.ListTags
}
