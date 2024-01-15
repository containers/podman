package toc

import (
	"github.com/containers/storage/pkg/chunked/internal"
	digest "github.com/opencontainers/go-digest"
)

// tocJSONDigestAnnotation is the annotation key for the digest of the estargz
// TOC JSON.
// It is defined in github.com/containerd/stargz-snapshotter/estargz as TOCJSONDigestAnnotation
// Duplicate it here to avoid a dependency on the package.
const tocJSONDigestAnnotation = "containerd.io/snapshot/stargz/toc.digest"

// GetTOCDigest returns the digest of the TOC as recorded in the annotations.
// This function retrieves a digest that represents the content of a
// table of contents (TOC) from the image's annotations.
// This is an experimental feature and may be changed/removed in the future.
func GetTOCDigest(annotations map[string]string) (*digest.Digest, error) {
	if contentDigest, ok := annotations[tocJSONDigestAnnotation]; ok {
		d, err := digest.Parse(contentDigest)
		if err != nil {
			return nil, err
		}
		return &d, nil
	}
	if contentDigest, ok := annotations[internal.ManifestChecksumKey]; ok {
		d, err := digest.Parse(contentDigest)
		if err != nil {
			return nil, err
		}
		return &d, nil
	}
	return nil, nil
}
