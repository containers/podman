package events

import (
	"strings"
	"time"

	"github.com/containers/libpod/pkg/util"
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
	filterSplit := strings.Split(filter, "=")
	if len(filterSplit) != 2 {
		return "", "", errors.Errorf("%s is an invalid filter", filter)
	}
	return filterSplit[0], filterSplit[1], nil
}

func generateEventOptions(filters []string, since, until string) ([]EventFilter, error) {
	var options []EventFilter
	for _, filter := range filters {
		key, val, err := parseFilter(filter)
		if err != nil {
			return nil, err
		}
		funcFilter, err := generateEventFilter(key, val)
		if err != nil {
			return nil, err
		}
		options = append(options, funcFilter)
	}

	if len(since) > 0 {
		timeSince, err := util.ParseInputTime(since)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to convert since time of %s", since)
		}
		options = append(options, generateEventSinceOption(timeSince))
	}

	if len(until) > 0 {
		timeUntil, err := util.ParseInputTime(until)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to convert until time of %s", until)
		}
		options = append(options, generateEventUntilOption(timeUntil))
	}
	return options, nil
}
