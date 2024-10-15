//go:build !linux

package storage

import (
	"errors"

	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/types"
)

func (s *store) getAutoUserNS(_ *types.AutoUserNsOptions, _ *Image, _ rwLayerStore, _ []roLayerStore) ([]idtools.IDMap, []idtools.IDMap, error) {
	return nil, nil, errors.New("user namespaces are not supported on this platform")
}
