package image

import (
	"github.com/containers/image/v5/internal/image"
	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/internal/unparsedimage"
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

// unparsedWithRef wraps a private.UnparsedImage, claiming another replacementRef
type unparsedWithRef struct {
	private.UnparsedImage
	ref types.ImageReference
}

func (uwr *unparsedWithRef) Reference() types.ImageReference {
	return uwr.ref
}

// UnparsedInstanceWithReference returns a types.UnparsedImage for wrappedInstance which claims to be a replacementRef.
// This is useful for combining image data with other reference values, e.g. to check signatures on a locally-pulled image
// based on a remote-registry policy.
func UnparsedInstanceWithReference(wrappedInstance types.UnparsedImage, replacementRef types.ImageReference) types.UnparsedImage {
	return &unparsedWithRef{
		UnparsedImage: unparsedimage.FromPublic(wrappedInstance),
		ref:           replacementRef,
	}
}
