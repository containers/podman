package manifest

import "fmt"

// NonImageArtifactError (detected via errors.As) is used when asking for an image-specific operation
// on an object which is not a “container image” in the standard sense (e.g. an OCI artifact)
//
// This is publicly visible as c/image/manifest.NonImageArtifactError (but we don’t provide a public constructor)
type NonImageArtifactError struct {
	// Callers should not be blindly calling image-specific operations and only checking MIME types
	// on failure; if they care about the artifact type, they should check before using it.
	// If they blindly assume an image, they don’t really need this value; just a type check
	// is sufficient for basic "we can only pull images" UI.
	//
	// Also, there are fairly widespread “artifacts” which nevertheless use imgspecv1.MediaTypeImageConfig,
	// e.g. https://github.com/sigstore/cosign/blob/main/specs/SIGNATURE_SPEC.md , which could cause the callers
	// to complain about a non-image artifact with the correct MIME type; we should probably add some other kind of
	// type discrimination, _and_ somehow make it available in the API, if we expect API callers to make decisions
	// based on that kind of data.
	//
	// So, let’s not expose this until a specific need is identified.
	mimeType string
}

// NewNonImageArtifactError returns a NonImageArtifactError about an artifact with mimeType.
func NewNonImageArtifactError(mimeType string) error {
	return NonImageArtifactError{mimeType: mimeType}
}

func (e NonImageArtifactError) Error() string {
	return fmt.Sprintf("unsupported image-specific operation on artifact with type %q", e.mimeType)
}
