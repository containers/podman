package util

import (
	"time"

	"github.com/containers/podman/v3/pkg/timetype"
	"github.com/pkg/errors"
)

// ComputeUntilTimestamp extracts unitil timestamp from filters
func ComputeUntilTimestamp(filter string, filterValues []string) (time.Time, error) {
	invalid := time.Time{}
	if len(filterValues) != 1 {
		return invalid, errors.Errorf("specify exactly one timestamp for %s", filter)
	}
	ts, err := timetype.GetTimestamp(filterValues[0], time.Now())
	if err != nil {
		return invalid, err
	}
	seconds, nanoseconds, err := timetype.ParseTimestamps(ts, 0)
	if err != nil {
		return invalid, err
	}
	return time.Unix(seconds, nanoseconds), nil
}
