package system

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
func (o *EventsOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *EventsOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
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

	if o.Since != nil {
		params.Set("since", *o.Since)
	}

	if o.Stream != nil {
		params.Set("stream", strconv.FormatBool(*o.Stream))
	}

	if o.Until != nil {
		params.Set("until", *o.Until)
	}

	return params, nil
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
