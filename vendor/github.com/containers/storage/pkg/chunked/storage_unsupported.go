// +build !linux

package chunked

import (
	"context"

	storage "github.com/containers/storage"
	graphdriver "github.com/containers/storage/drivers"
	"github.com/pkg/errors"
)

// GetDiffer returns a differ than can be used with ApplyDiffWithDiffer.
func GetDiffer(ctx context.Context, store storage.Store, blobSize int64, annotations map[string]string, iss ImageSourceSeekable) (graphdriver.Differ, error) {
	return nil, errors.New("format not supported on this architecture")
}
