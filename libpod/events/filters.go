package events

import (
	"strings"
	"time"

	"github.com/containers/podman/v3/pkg/util"
	"github.com/pkg/errors"
)

func generateEventFilter(filter, filterValue string) (func(e *Event) bool, error) {
	switch strings.ToUpper(filter) {
	case "CONTAINER":
		return func(e *Event) bool {
			if e.Type != Container {
				return false
			}
			if e.Name == filterValue {
				return true
			}
			return strings.HasPrefix(e.ID, filterValue)
		}, nil
	case "EVENT", "STATUS":
		return func(e *Event) bool {
			return string(e.Status) == filterValue
		}, nil
	case "IMAGE":
		return func(e *Event) bool {
			if e.Type != Image {
				return false
			}
			if e.Name == filterValue {
				return true
			}
			return strings.HasPrefix(e.ID, filterValue)
		}, nil
	case "POD":
		return func(e *Event) bool {
			if e.Type != Pod {
				return false
			}
			if e.Name == filterValue {
				return true
			}
			return strings.HasPrefix(e.ID, filterValue)
		}, nil
	case "VOLUME":
		return func(e *Event) bool {
			if e.Type != Volume {
				return false
			}
			return strings.HasPrefix(e.ID, filterValue)
		}, nil
	case "TYPE":
		return func(e *Event) bool {
			return string(e.Type) == filterValue
		}, nil

	case "LABEL":
		return func(e *Event) bool {
			var found bool
			// iterate labels and see if we match a key and value
			for eventKey, eventValue := range e.Attributes {
				filterValueSplit := strings.SplitN(filterValue, "=", 2)
				// if the filter isn't right, just return false
				if len(filterValueSplit) < 2 {
					return false
				}
				if eventKey == filterValueSplit[0] && eventValue == filterValueSplit[1] {
					found = true
					break
				}
			}
			return found
		}, nil
	}
	return nil, errors.Errorf("%s is an invalid filter", filter)
}

func generateEventSinceOption(timeSince time.Time) func(e *Event) bool {
	return func(e *Event) bool {
		return e.Time.After(timeSince)
	}
}

func generateEventUntilOption(timeUntil time.Time) func(e *Event) bool {
	return func(e *Event) bool {
		return e.Time.Before(timeUntil)
	}
}

func parseFilter(filter string) (string, string, error) {
	filterSplit := strings.SplitN(filter, "=", 2)
	if len(filterSplit) != 2 {
		return "", "", errors.Errorf("%s is an invalid filter", filter)
	}
	return filterSplit[0], filterSplit[1], nil
}

// applyFilters applies the EventFilter slices in sequence.  Filters under the
// same key are disjunctive while each key must match (conjuctive).
func applyFilters(event *Event, filterMap map[string][]EventFilter) bool {
	for _, filters := range filterMap {
		success := false
		for _, filter := range filters {
			if filter(event) {
				success = true
				break
			}
		}
		if !success {
			return false
		}
	}
	return true
}

// generateEventFilter parses the specified filters into a filter map that can
// later on be used to filter events.  Keys are conjunctive, values are
// disjunctive.
func generateEventFilters(filters []string, since, until string) (map[string][]EventFilter, error) {
	filterMap := make(map[string][]EventFilter)
	for _, filter := range filters {
		key, val, err := parseFilter(filter)
		if err != nil {
			return nil, err
		}
		filterFunc, err := generateEventFilter(key, val)
		if err != nil {
			return nil, err
		}
		filterSlice := filterMap[key]
		filterSlice = append(filterSlice, filterFunc)
		filterMap[key] = filterSlice
	}

	if len(since) > 0 {
		timeSince, err := util.ParseInputTime(since)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to convert since time of %s", since)
		}
		filterFunc := generateEventSinceOption(timeSince)
		filterMap["since"] = []EventFilter{filterFunc}
	}

	if len(until) > 0 {
		timeUntil, err := util.ParseInputTime(until)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to convert until time of %s", until)
		}
		filterFunc := generateEventUntilOption(timeUntil)
		filterMap["until"] = []EventFilter{filterFunc}
	}
	return filterMap, nil
}
