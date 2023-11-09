package storage

import (
	"fmt"

	"github.com/containers/storage/types"
)

// ParseIDMapping takes idmappings and subuid and subgid maps and returns a storage mapping
func ParseIDMapping(UIDMapSlice, GIDMapSlice []string, subUIDMap, subGIDMap string) (*types.IDMappingOptions, error) {
	return types.ParseIDMapping(UIDMapSlice, GIDMapSlice, subUIDMap, subGIDMap)
}

// DefaultStoreOptions returns the default storage options for containers
func DefaultStoreOptions() (types.StoreOptions, error) {
	return types.DefaultStoreOptions()
}

func validateMountOptions(mountOptions []string) error {
	var Empty struct{}
	// Add invalid options for ImageMount() here.
	invalidOptions := map[string]struct{}{
		"rw": Empty,
	}

	for _, opt := range mountOptions {
		if _, ok := invalidOptions[opt]; ok {
			return fmt.Errorf(" %q option not supported", opt)
		}
	}
	return nil
}

func applyNameOperation(oldNames []string, opParameters []string, op updateNameOperation) ([]string, error) {
	var result []string
	switch op {
	case setNames:
		// ignore all old names and just return new names
		result = opParameters
	case removeNames:
		// remove given names from old names
		result = make([]string, 0, len(oldNames))
		for _, name := range oldNames {
			// only keep names in final result which do not intersect with input names
			// basically `result = oldNames - opParameters`
			nameShouldBeRemoved := false
			for _, opName := range opParameters {
				if name == opName {
					nameShouldBeRemoved = true
				}
			}
			if !nameShouldBeRemoved {
				result = append(result, name)
			}
		}
	case addNames:
		result = make([]string, 0, len(opParameters)+len(oldNames))
		result = append(result, opParameters...)
		result = append(result, oldNames...)
	default:
		return result, errInvalidUpdateNameOperation
	}
	return dedupeStrings(result), nil
}
