//go:build !containers_image_storage_stub
// +build !containers_image_storage_stub

package storage

import (
	"context"

	"github.com/containers/image/v5/internal/image"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	digest "github.com/opencontainers/go-digest"
)

var (
	// ErrNoSuchImage is returned when we attempt to access an image which
	// doesn't exist in the storage area.
	ErrNoSuchImage = storage.ErrNotAnImage
)

type storageImageCloser struct {
	types.ImageCloser
	size int64
}

// manifestBigDataKey returns a key suitable for recording a manifest with the specified digest using storage.Store.ImageBigData and related functions.
// If a specific manifest digest is explicitly requested by the user, the key returned by this function should be used preferably;
// for compatibility, if a manifest is not available under this key, check also storage.ImageDigestBigDataKey
func manifestBigDataKey(digest digest.Digest) string {
	return storage.ImageDigestManifestBigDataNamePrefix + "-" + digest.String()
}

// signatureBigDataKey returns a key suitable for recording the signatures associated with the manifest with the specified digest using storage.Store.ImageBigData and related functions.
// If a specific manifest digest is explicitly requested by the user, the key returned by this function should be used preferably;
func signatureBigDataKey(digest digest.Digest) string {
	return "signature-" + digest.Encoded()
}

// Size() returns the previously-computed size of the image, with no error.
func (s *storageImageCloser) Size() (int64, error) {
	return s.size, nil
}

// newImage creates an image that also knows its size
func newImage(ctx context.Context, sys *types.SystemContext, s storageReference) (types.ImageCloser, error) {
	src, err := newImageSource(sys, s)
	if err != nil {
		return nil, err
	}
	img, err := image.FromSource(ctx, sys, src)
	if err != nil {
		return nil, err
	}
	size, err := src.getSize()
	if err != nil {
		return nil, err
	}
	return &storageImageCloser{ImageCloser: img, size: size}, nil
}
