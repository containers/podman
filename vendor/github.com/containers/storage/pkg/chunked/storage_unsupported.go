//go:build !linux
// +build !linux

package chunked

import (
	"context"
	"errors"

	storage "github.com/containers/storage"
	graphdriver "github.com/containers/storage/drivers"
	digest "github.com/opencontainers/go-digest"
)

// GetDiffer returns a differ than can be used with ApplyDiffWithDiffer.
func GetDiffer(ctx context.Context, store storage.Store, blobSize int64, annotations map[string]string, iss ImageSourceSeekable) (graphdriver.Differ, error) {
	return nil, errors.New("format not supported on this system")
}

// GetTOCDigest returns the digest of the TOC as recorded in the annotations.
// This is an experimental feature and may be changed/removed in the future.
func GetTOCDigest(annotations map[string]string) (*digest.Digest, error) {
	return nil, errors.New("format not supported on this system")
}
