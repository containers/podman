package system

import (
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *EventsOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *EventsOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithFilters
func (o *EventsOptions) WithFilters(value map[string][]string) *EventsOptions {
	v := value
	o.Filters = v
	return o
}

// GetFilters
func (o *EventsOptions) GetFilters() map[string][]string {
	var filters map[string][]string
	if o.Filters == nil {
		return filters
	}
	return o.Filters
}

// WithSince
func (o *EventsOptions) WithSince(value string) *EventsOptions {
	v := &value
	o.Since = v
	return o
}

// GetSince
func (o *EventsOptions) GetSince() string {
	var since string
	if o.Since == nil {
		return since
	}
	return *o.Since
}

// WithStream
func (o *EventsOptions) WithStream(value bool) *EventsOptions {
	v := &value
	o.Stream = v
	return o
}

// GetStream
func (o *EventsOptions) GetStream() bool {
	var stream bool
	if o.Stream == nil {
		return stream
	}
	return *o.Stream
}

// WithUntil
func (o *EventsOptions) WithUntil(value string) *EventsOptions {
	v := &value
	o.Until = v
	return o
}

// GetUntil
func (o *EventsOptions) GetUntil() string {
	var until string
	if o.Until == nil {
		return until
	}
	return *o.Until
}
