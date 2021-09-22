package util

import (
	"strings"

	"github.com/containers/podman/v3/libpod/network/types"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/pkg/errors"
)

func GenerateNetworkFilters(filters map[string][]string) ([]types.FilterFunc, error) {
	filterFuncs := make([]types.FilterFunc, 0, len(filters))
	for key, filterValues := range filters {
		filterFunc, err := createFilterFuncs(key, filterValues)
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, filterFunc)
	}
	return filterFuncs, nil
}

func createFilterFuncs(key string, filterValues []string) (types.FilterFunc, error) {
	switch strings.ToLower(key) {
	case "name":
		// matches one name, regex allowed
		return func(net types.Network) bool {
			return util.StringMatchRegexSlice(net.Name, filterValues)
		}, nil

	case "driver":
		// matches network driver
		return func(net types.Network) bool {
			return util.StringInSlice(net.Driver, filterValues)
		}, nil

	case "id":
		// matches part of one id
		return func(net types.Network) bool {
			return util.StringMatchRegexSlice(net.ID, filterValues)
		}, nil

		// TODO: add dns enabled, internal filter
	}
	return createPruneFilterFuncs(key, filterValues)
}

func GenerateNetworkPruneFilters(filters map[string][]string) ([]types.FilterFunc, error) {
	filterFuncs := make([]types.FilterFunc, 0, len(filters))
	for key, filterValues := range filters {
		filterFunc, err := createPruneFilterFuncs(key, filterValues)
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, filterFunc)
	}
	return filterFuncs, nil
}

func createPruneFilterFuncs(key string, filterValues []string) (types.FilterFunc, error) {
	switch strings.ToLower(key) {
	case "label":
		// matches all labels
		return func(net types.Network) bool {
			return util.MatchLabelFilters(filterValues, net.Labels)
		}, nil

	case "until":
		until, err := util.ComputeUntilTimestamp(filterValues)
		if err != nil {
			return nil, err
		}
		return func(net types.Network) bool {
			return net.Created.Before(until)
		}, nil
	default:
		return nil, errors.Errorf("invalid filter %q", key)
	}
}
