package image

import (
	"context"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// UnparsedImage implements types.UnparsedImage .
// An UnparsedImage is a pair of (ImageSource, instance digest); it can represent either a manifest list or a single image instance.
type UnparsedImage struct {
	src            types.ImageSource
	instanceDigest *digest.Digest
	cachedManifest []byte // A private cache for Manifest(); nil if not yet known.
	// A private cache for Manifest(), may be the empty string if guessing failed.
	// Valid iff cachedManifest is not nil.
	cachedManifestMIMEType string
	cachedSignatures       [][]byte // A private cache for Signatures(); nil if not yet known.
}

// UnparsedInstance returns a types.UnparsedImage implementation for (source, instanceDigest).
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list).
//
// The UnparsedImage must not be used after the underlying ImageSource is Close()d.
func UnparsedInstance(src types.ImageSource, instanceDigest *digest.Digest) *UnparsedImage {
	return &UnparsedImage{
		src:            src,
		instanceDigest: instanceDigest,
	}
}

// Reference returns the reference used to set up this source, _as specified by the user_
// (not as the image itself, or its underlying storage, claims).  This can be used e.g. to determine which public keys are trusted for this image.
func (i *UnparsedImage) Reference() types.ImageReference {
	// Note that this does not depend on instanceDigest; e.g. all instances within a manifest list need to be signed with the manifest list identity.
	return i.src.Reference()
}

// Manifest is like ImageSource.GetManifest, but the result is cached; it is OK to call this however often you need.
func (i *UnparsedImage) Manifest() ([]byte, string, error) {
	if i.cachedManifest == nil {
		m, mt, err := i.src.GetManifest(i.instanceDigest)
		if err != nil {
			return nil, "", err
		}

		// ImageSource.GetManifest does not do digest verification, but we do;
		// this immediately protects also any user of types.Image.
		if digest, haveDigest := i.expectedManifestDigest(); haveDigest {
			matches, err := manifest.MatchesDigest(m, digest)
			if err != nil {
				return nil, "", errors.Wrap(err, "Error computing manifest digest")
			}
			if !matches {
				return nil, "", errors.Errorf("Manifest does not match provided manifest digest %s", digest)
			}
		}

		i.cachedManifest = m
		i.cachedManifestMIMEType = mt
	}
	return i.cachedManifest, i.cachedManifestMIMEType, nil
}

// expectedManifestDigest returns a the expected value of the manifest digest, and an indicator whether it is known.
// The bool return value seems redundant with digest != ""; it is used explicitly
// to refuse (unexpected) situations when the digest exists but is "".
func (i *UnparsedImage) expectedManifestDigest() (digest.Digest, bool) {
	if i.instanceDigest != nil {
		return *i.instanceDigest, true
	}
	ref := i.Reference().DockerReference()
	if ref != nil {
		if canonical, ok := ref.(reference.Canonical); ok {
			return canonical.Digest(), true
		}
	}
	return "", false
}

// Signatures is like ImageSource.GetSignatures, but the result is cached; it is OK to call this however often you need.
func (i *UnparsedImage) Signatures(ctx context.Context) ([][]byte, error) {
	if i.cachedSignatures == nil {
		sigs, err := i.src.GetSignatures(ctx, i.instanceDigest)
		if err != nil {
			return nil, err
		}
		i.cachedSignatures = sigs
	}
	return i.cachedSignatures, nil
}

// LayerInfosForCopy returns an updated set of layer blob information which may not match the manifest.
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (i *UnparsedImage) LayerInfosForCopy() []types.BlobInfo {
	return i.src.LayerInfosForCopy()
}
