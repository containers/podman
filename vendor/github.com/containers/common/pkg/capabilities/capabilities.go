package capabilities

// Copyright 2013-2018 Docker, Inc.

// NOTE: this package has been copied from github.com/docker/docker but been
//       changed significantly to fit the needs of libpod.

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/syndtr/gocapability/capability"
)

var (
	// Used internally and populated during init().
	capabilityList []string

	// Used internally and populated during init().
	capsList []capability.Cap

	// ErrUnknownCapability is thrown when an unknown capability is processed.
	ErrUnknownCapability = errors.New("unknown capability")

	// ContainerImageLabels - label can indicate the required
	// capabilities required by containers to run the container image.
	ContainerImageLabels = []string{"io.containers.capabilities"}
)

// All is a special value used to add/drop all known capabilities.
// Useful on the CLI for `--cap-add=all` etc.
const All = "ALL"

func getCapName(c capability.Cap) string {
	return "CAP_" + strings.ToUpper(c.String())
}

func init() {
	last := capability.CAP_LAST_CAP
	// hack for RHEL6 which has no /proc/sys/kernel/cap_last_cap
	if last == capability.Cap(63) {
		last = capability.CAP_BLOCK_SUSPEND
	}
	for _, cap := range capability.List() {
		if cap > last {
			continue
		}
		capsList = append(capsList, cap)
		capabilityList = append(capabilityList, getCapName(cap))
		sort.Strings(capabilityList)
	}
}

var (
	boundingSetOnce sync.Once
	boundingSetRet  []string
	boundingSetErr  error
)

// BoundingSet returns the capabilities in the current bounding set
func BoundingSet() ([]string, error) {
	boundingSetOnce.Do(func() {
		currentCaps, err := capability.NewPid2(0)
		if err != nil {
			boundingSetErr = err
			return
		}
		err = currentCaps.Load()
		if err != nil {
			boundingSetErr = err
			return
		}
		var r []string
		for _, c := range capsList {
			if !currentCaps.Get(capability.BOUNDING, c) {
				continue
			}
			r = append(r, getCapName(c))
		}
		boundingSetRet = r
		sort.Strings(boundingSetRet)
		boundingSetErr = err
	})
	return boundingSetRet, boundingSetErr
}

// AllCapabilities returns all known capabilities.
func AllCapabilities() []string {
	return capabilityList
}

// NormalizeCapabilities normalizes caps by adding a "CAP_" prefix (if not yet
// present).
func NormalizeCapabilities(caps []string) ([]string, error) {
	normalized := make([]string, 0, len(caps))
	for _, c := range caps {
		c = strings.ToUpper(c)
		if c == All {
			normalized = append(normalized, c)
			continue
		}
		if !strings.HasPrefix(c, "CAP_") {
			c = "CAP_" + c
		}
		if !slices.Contains(capabilityList, c) {
			return nil, fmt.Errorf("%q: %w", c, ErrUnknownCapability)
		}
		normalized = append(normalized, c)
	}
	sort.Strings(normalized)
	return normalized, nil
}

// ValidateCapabilities validates if caps only contains valid capabilities.
func ValidateCapabilities(caps []string) error {
	for _, c := range caps {
		if !slices.Contains(capabilityList, c) {
			return fmt.Errorf("%q: %w", c, ErrUnknownCapability)
		}
	}
	return nil
}

// MergeCapabilities computes a set of capabilities by adding capabilities
// to or dropping them from base.
//
// Note that:
// "ALL" in capAdd adds returns known capabilities
// "All" in capDrop returns only the capabilities specified in capAdd
func MergeCapabilities(base, adds, drops []string) ([]string, error) {
	// Normalize the base capabilities
	base, err := NormalizeCapabilities(base)
	if err != nil {
		return nil, err
	}
	if len(adds) == 0 && len(drops) == 0 {
		// Nothing to tweak; we're done
		return base, nil
	}
	capDrop, err := NormalizeCapabilities(drops)
	if err != nil {
		return nil, err
	}
	capAdd, err := NormalizeCapabilities(adds)
	if err != nil {
		return nil, err
	}

	if slices.Contains(capDrop, All) {
		if slices.Contains(capAdd, All) {
			return nil, errors.New("adding all caps and removing all caps not allowed")
		}
		// "Drop" all capabilities; return what's in capAdd instead
		sort.Strings(capAdd)
		return capAdd, nil
	}

	if slices.Contains(capAdd, All) {
		base, err = BoundingSet()
		if err != nil {
			return nil, err
		}
		capAdd = []string{}
	} else {
		for _, add := range capAdd {
			if slices.Contains(capDrop, add) {
				return nil, fmt.Errorf("capability %q cannot be dropped and added", add)
			}
		}
	}

	for _, drop := range capDrop {
		if slices.Contains(capAdd, drop) {
			return nil, fmt.Errorf("capability %q cannot be dropped and added", drop)
		}
	}

	caps := make([]string, 0, len(base)+len(capAdd))
	// Drop any capabilities in capDrop that are in base
	for _, cap := range base {
		if slices.Contains(capDrop, cap) {
			continue
		}
		caps = append(caps, cap)
	}

	// Add any capabilities in capAdd that are not in base
	for _, cap := range capAdd {
		if slices.Contains(base, cap) {
			continue
		}
		caps = append(caps, cap)
	}
	sort.Strings(caps)
	return caps, nil
}
