package libpod

import (
	"strings"
	"time"
)

type (
	// Filterable defines items to be filtered for CLI and Varlink
	Filterable interface{}
)

// Filter defines type of function returned to perform constraint checking
type Filter func(f Filterable) bool

// BeforeFilter used to filter containers created before target time
func BeforeFilter(target time.Time) (Filter, error) {
	return func(f Filterable) bool {
		if m, ok := f.(interface{ CreatedTime() time.Time }); ok {
			return target.After(m.CreatedTime())
		} else if m, ok := f.(interface{ Created() time.Time }); ok {
			return target.After(m.Created())
		} else {
			return false
		}
	}, nil
}

// SinceFilter used to filterable containers created since target time
func SinceFilter(target time.Time) (Filter, error) {
	return func(f Filterable) bool {
		if m, ok := f.(interface{ CreatedTime() time.Time }); ok {
			return m.CreatedTime().Before(target)
		} else if m, ok := f.(interface{ Created() time.Time }); ok {
			return m.Created().Before(target)
		} else {
			return false
		}
	}, nil
}

// IDFilter used to filter on target ID
func IDFilter(target string) (Filter, error) {
	return func(f Filterable) bool {
		if m, ok := f.(interface{ ID() string }); ok {
			return strings.Contains(m.ID(), target)
		}
		return false
	}, nil
}

// LabelFilter allows you to filterable by images labels key and/or value
func LabelFilter(target string) (Filter, error) {
	return func(f Filterable) bool {
		if m, ok := f.(interface{ Labels() map[string]string }); ok {
			var targets = strings.Split(target, "=")
			var key = targets[0]
			var value = ""
			if len(targets) > 1 {
				value = targets[1]
			}

			labels := m.Labels()
			if len(strings.TrimSpace(labels[key])) > 0 && len(strings.TrimSpace(value)) == 0 {
				return true
			}
			return labels[key] == value
		}
		return false
	}, nil
}

// NameFilter used to filter on target's name
func NameFilter(target string) (Filter, error) {
	return func(f Filterable) bool {
		if m, ok := f.(interface{ Name() string }); ok {
			return strings.Contains(m.Name(), target)
		}
		return false
	}, nil
}
