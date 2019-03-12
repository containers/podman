package shared

import (
	"fmt"
	"strings"
	"time"

	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
)

func generateEventFilter(filter, filterValue string) (func(e *events.Event) bool, error) {
	switch strings.ToUpper(filter) {
	case "CONTAINER":
		return func(e *events.Event) bool {
			if e.Type != events.Container {
				return false
			}
			if e.Name == filterValue {
				return true
			}
			return strings.HasPrefix(e.ID, filterValue)
		}, nil
	case "EVENT", "STATUS":
		return func(e *events.Event) bool {
			return fmt.Sprintf("%s", e.Status) == filterValue
		}, nil
	case "IMAGE":
		return func(e *events.Event) bool {
			if e.Type != events.Image {
				return false
			}
			if e.Name == filterValue {
				return true
			}
			return strings.HasPrefix(e.ID, filterValue)
		}, nil
	case "POD":
		return func(e *events.Event) bool {
			if e.Type != events.Pod {
				return false
			}
			if e.Name == filterValue {
				return true
			}
			return strings.HasPrefix(e.ID, filterValue)
		}, nil
	case "VOLUME":
		return func(e *events.Event) bool {
			if e.Type != events.Volume {
				return false
			}
			return strings.HasPrefix(e.ID, filterValue)
		}, nil
	case "TYPE":
		return func(e *events.Event) bool {
			return fmt.Sprintf("%s", e.Type) == filterValue
		}, nil
	}
	return nil, errors.Errorf("%s is an invalid filter", filter)
}

func generateEventSinceOption(timeSince time.Time) func(e *events.Event) bool {
	return func(e *events.Event) bool {
		return e.Time.After(timeSince)
	}
}

func generateEventUntilOption(timeUntil time.Time) func(e *events.Event) bool {
	return func(e *events.Event) bool {
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

func GenerateEventOptions(filters []string, since, until string) ([]events.EventFilter, error) {
	var options []events.EventFilter
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
