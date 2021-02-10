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
func (o *PruneOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *PruneOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.All != nil {
		params.Set("all", strconv.FormatBool(*o.All))
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

	if o.Volumes != nil {
		params.Set("volumes", strconv.FormatBool(*o.Volumes))
	}

	return params, nil
}

// WithAll
func (o *PruneOptions) WithAll(value bool) *PruneOptions {
	v := &value
	o.All = v
	return o
}

// GetAll
func (o *PruneOptions) GetAll() bool {
	var all bool
	if o.All == nil {
		return all
	}
	return *o.All
}

// WithFilters
func (o *PruneOptions) WithFilters(value map[string][]string) *PruneOptions {
	v := value
	o.Filters = v
	return o
}

// GetFilters
func (o *PruneOptions) GetFilters() map[string][]string {
	var filters map[string][]string
	if o.Filters == nil {
		return filters
	}
	return o.Filters
}

// WithVolumes
func (o *PruneOptions) WithVolumes(value bool) *PruneOptions {
	v := &value
	o.Volumes = v
	return o
}

// GetVolumes
func (o *PruneOptions) GetVolumes() bool {
	var volumes bool
	if o.Volumes == nil {
		return volumes
	}
	return *o.Volumes
}
