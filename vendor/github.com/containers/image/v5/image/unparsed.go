package image

import (
	"github.com/containers/image/v5/internal/image"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
)

// UnparsedImage implements types.UnparsedImage .
// An UnparsedImage is a pair of (ImageSource, instance digest); it can represent either a manifest list or a single image instance.
type UnparsedImage = image.UnparsedImage

// UnparsedInstance returns a types.UnparsedImage implementation for (source, instanceDigest).
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list).
//
// The UnparsedImage must not be used after the underlying ImageSource is Close()d.
func UnparsedInstance(src types.ImageSource, instanceDigest *digest.Digest) *UnparsedImage {
	return image.UnparsedInstance(src, instanceDigest)
}
