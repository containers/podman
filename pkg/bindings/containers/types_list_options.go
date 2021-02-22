package containers

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ListOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *ListOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
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
