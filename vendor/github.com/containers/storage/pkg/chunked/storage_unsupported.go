//go:build !linux
// +build !linux

package chunked

import (
	"context"
	"errors"

	storage "github.com/containers/storage"
	graphdriver "github.com/containers/storage/drivers"
)

// GetDiffer returns a differ than can be used with ApplyDiffWithDiffer.
func GetDiffer(ctx context.Context, store storage.Store, blobSize int64, annotations map[string]string, iss ImageSourceSeekable) (graphdriver.Differ, error) {
	return nil, errors.New("format not supported on this system")
}
