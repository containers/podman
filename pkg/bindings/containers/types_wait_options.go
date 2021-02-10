package containers

import (
	"net/url"
	"reflect"
	"strconv"

	"github.com/containers/podman/v2/libpod/define"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *WaitOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *WaitOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Condition != nil {
		for _, val := range o.Condition {
			params.Add("condition", strconv.FormatInt(int64(val), 10))
		}
	}

	if o.Interval != nil {
		params.Set("interval", *o.Interval)
	}

	return params, nil
}

// WithCondition
func (o *WaitOptions) WithCondition(value []define.ContainerStatus) *WaitOptions {
	v := value
	o.Condition = v
	return o
}

// GetCondition
func (o *WaitOptions) GetCondition() []define.ContainerStatus {
	var condition []define.ContainerStatus
	if o.Condition == nil {
		return condition
	}
	return o.Condition
}

// WithInterval
func (o *WaitOptions) WithInterval(value string) *WaitOptions {
	v := &value
	o.Interval = v
	return o
}

// GetInterval
func (o *WaitOptions) GetInterval() string {
	var interval string
	if o.Interval == nil {
		return interval
	}
	return *o.Interval
}
