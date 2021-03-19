package storage

import (
	"fmt"

	"github.com/containers/storage/types"
)

// ParseIDMapping takes idmappings and subuid and subgid maps and returns a storage mapping
func ParseIDMapping(UIDMapSlice, GIDMapSlice []string, subUIDMap, subGIDMap string) (*types.IDMappingOptions, error) {
	return types.ParseIDMapping(UIDMapSlice, GIDMapSlice, subUIDMap, subGIDMap)
}

// GetRootlessRuntimeDir returns the runtime directory when running as non root
func GetRootlessRuntimeDir(rootlessUID int) (string, error) {
	return types.GetRootlessRuntimeDir(rootlessUID)
}

// DefaultStoreOptionsAutoDetectUID returns the default storage ops for containers
func DefaultStoreOptionsAutoDetectUID() (types.StoreOptions, error) {
	return types.DefaultStoreOptionsAutoDetectUID()
}

// DefaultStoreOptions returns the default storage ops for containers
func DefaultStoreOptions(rootless bool, rootlessUID int) (types.StoreOptions, error) {
	return types.DefaultStoreOptions(rootless, rootlessUID)
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
