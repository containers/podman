package images

import (
	"net/url"
	"reflect"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *SearchOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *SearchOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Authfile != nil {
		params.Set("authfile", *o.Authfile)
	}

	if o.Filters != nil {
		lower := make(map[string][]string, len(o.Filters))
		for key, val := range o.Filters {
			lower[strings.ToLower(key)] = val
		}
		s, err := jsoniter.ConfigCompatibleWithStandardLibrary.MarshalToString(lower)
		if err != nil {
			return nil, err
		}
		params.Set("filters", s)
	}

	if o.Limit != nil {
		params.Set("limit", strconv.FormatInt(int64(*o.Limit), 10))
	}

	if o.NoTrunc != nil {
		params.Set("notrunc", strconv.FormatBool(*o.NoTrunc))
	}

	if o.SkipTLSVerify != nil {
		params.Set("skiptlsverify", strconv.FormatBool(*o.SkipTLSVerify))
	}

	if o.ListTags != nil {
		params.Set("listtags", strconv.FormatBool(*o.ListTags))
	}

	return params, nil
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
