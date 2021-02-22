package containers

import (
	"net/url"

	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *WaitOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *WaitOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
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
