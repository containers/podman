package containers

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
func (o *ListOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *ListOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.All != nil {
		params.Set("all", strconv.FormatBool(*o.All))
	}

	if o.External != nil {
		params.Set("external", strconv.FormatBool(*o.External))
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

	if o.Last != nil {
		params.Set("last", strconv.FormatInt(int64(*o.Last), 10))
	}

	if o.Namespace != nil {
		params.Set("namespace", strconv.FormatBool(*o.Namespace))
	}

	if o.Size != nil {
		params.Set("size", strconv.FormatBool(*o.Size))
	}

	if o.Sync != nil {
		params.Set("sync", strconv.FormatBool(*o.Sync))
	}

	return params, nil
}

// WithAll
func (o *ListOptions) WithAll(value bool) *ListOptions {
	v := &value
	o.All = v
	return o
}

// GetAll
func (o *ListOptions) GetAll() bool {
	var all bool
	if o.All == nil {
		return all
	}
	return *o.All
}

// WithExternal
func (o *ListOptions) WithExternal(value bool) *ListOptions {
	v := &value
	o.External = v
	return o
}

// GetExternal
func (o *ListOptions) GetExternal() bool {
	var external bool
	if o.External == nil {
		return external
	}
	return *o.External
}

// WithFilters
func (o *ListOptions) WithFilters(value map[string][]string) *ListOptions {
	v := value
	o.Filters = v
	return o
}

// GetFilters
func (o *ListOptions) GetFilters() map[string][]string {
	var filters map[string][]string
	if o.Filters == nil {
		return filters
	}
	return o.Filters
}

// WithLast
func (o *ListOptions) WithLast(value int) *ListOptions {
	v := &value
	o.Last = v
	return o
}

// GetLast
func (o *ListOptions) GetLast() int {
	var last int
	if o.Last == nil {
		return last
	}
	return *o.Last
}

// WithNamespace
func (o *ListOptions) WithNamespace(value bool) *ListOptions {
	v := &value
	o.Namespace = v
	return o
}

// GetNamespace
func (o *ListOptions) GetNamespace() bool {
	var namespace bool
	if o.Namespace == nil {
		return namespace
	}
	return *o.Namespace
}

// WithSize
func (o *ListOptions) WithSize(value bool) *ListOptions {
	v := &value
	o.Size = v
	return o
}

// GetSize
func (o *ListOptions) GetSize() bool {
	var size bool
	if o.Size == nil {
		return size
	}
	return *o.Size
}

// WithSync
func (o *ListOptions) WithSync(value bool) *ListOptions {
	v := &value
	o.Sync = v
	return o
}

// GetSync
func (o *ListOptions) GetSync() bool {
	var sync bool
	if o.Sync == nil {
		return sync
	}
	return *o.Sync
}
