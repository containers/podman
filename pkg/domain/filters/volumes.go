//go:build !remote

package filters

import (
	"fmt"
	"strings"
	"time"

	"github.com/containers/common/pkg/filters"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/util"
)

func GenerateVolumeFilters(filter string, filterValues []string, runtime *libpod.Runtime) (libpod.VolumeFilter, error) {
	switch filter {
	case "after", "since":
		return createAfterFilterVolumeFunction(filterValues, runtime)
	case "name":
		return func(v *libpod.Volume) bool {
			return util.StringMatchRegexSlice(v.Name(), filterValues)
		}, nil
	case "driver":
		return func(v *libpod.Volume) bool {
			for _, val := range filterValues {
				if v.Driver() == val {
					return true
				}
			}
			return false
		}, nil
	case "scope":
		return func(v *libpod.Volume) bool {
			for _, val := range filterValues {
				if v.Scope() == val {
					return true
				}
			}
			return false
		}, nil
	case "label":
		return func(v *libpod.Volume) bool {
			return filters.MatchLabelFilters(filterValues, v.Labels())
		}, nil
	case "label!":
		return func(v *libpod.Volume) bool {
			return !filters.MatchLabelFilters(filterValues, v.Labels())
		}, nil
	case "opt":
		return func(v *libpod.Volume) bool {
			for _, val := range filterValues {
				filterKey, filterVal, _ := strings.Cut(val, "=")

				for labelKey, labelValue := range v.Options() {
					if labelKey == filterKey && (filterVal == "" || labelValue == filterVal) {
						return true
					}
				}
			}
			return false
		}, nil
	case "until":
		return createUntilFilterVolumeFunction(filterValues)
	case "dangling":
		for _, val := range filterValues {
			switch strings.ToLower(val) {
			case "true", "1", "false", "0":
			default:
				return nil, fmt.Errorf("%q is not a valid value for the \"dangling\" filter - must be true or false", val)
			}
		}
		return func(v *libpod.Volume) bool {
			for _, val := range filterValues {
				dangling, err := v.IsDangling()
				if err != nil {
					return false
				}

				invert := false
				switch strings.ToLower(val) {
				case "false", "0":
					// Dangling=false requires that we
					// invert the result of IsDangling.
					invert = true
				}
				if invert {
					dangling = !dangling
				}
				if dangling {
					return true
				}
			}
			return false
		}, nil
	}
	return nil, fmt.Errorf("%q is an invalid volume filter", filter)
}

func GeneratePruneVolumeFilters(filter string, filterValues []string, runtime *libpod.Runtime) (libpod.VolumeFilter, error) {
	switch filter {
	case "after", "since":
		return createAfterFilterVolumeFunction(filterValues, runtime)
	case "label":
		return func(v *libpod.Volume) bool {
			return filters.MatchLabelFilters(filterValues, v.Labels())
		}, nil
	case "label!":
		return func(v *libpod.Volume) bool {
			return !filters.MatchLabelFilters(filterValues, v.Labels())
		}, nil
	case "until":
		return createUntilFilterVolumeFunction(filterValues)
	}
	return nil, fmt.Errorf("%q is an invalid volume filter", filter)
}

func createUntilFilterVolumeFunction(filterValues []string) (libpod.VolumeFilter, error) {
	until, err := filters.ComputeUntilTimestamp(filterValues)
	if err != nil {
		return nil, err
	}
	return func(v *libpod.Volume) bool {
		if !until.IsZero() && v.CreatedTime().Before(until) {
			return true
		}
		return false
	}, nil
}

func createAfterFilterVolumeFunction(filterValues []string, runtime *libpod.Runtime) (libpod.VolumeFilter, error) {
	var createTime time.Time
	for _, filterValue := range filterValues {
		vol, err := runtime.LookupVolume(filterValue)
		if err != nil {
			return nil, err
		}
		if createTime.IsZero() || createTime.After(vol.CreatedTime()) {
			createTime = vol.CreatedTime()
		}
	}
	return func(v *libpod.Volume) bool {
		return createTime.Before(v.CreatedTime())
	}, nil
}
