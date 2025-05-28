//go:build !linux

package chunked

import (
	"context"
	"errors"

	storage "github.com/containers/storage"
	graphdriver "github.com/containers/storage/drivers"
	digest "github.com/opencontainers/go-digest"
)

// NewDiffer returns a differ than can be used with [Store.PrepareStagedLayer].
// The caller must call Close() on the returned Differ.
func NewDiffer(ctx context.Context, store storage.Store, blobDigest digest.Digest, blobSize int64, annotations map[string]string, iss ImageSourceSeekable) (graphdriver.Differ, error) {
	return nil, newErrFallbackToOrdinaryLayerDownload(errors.New("format not supported on this system"))
}
