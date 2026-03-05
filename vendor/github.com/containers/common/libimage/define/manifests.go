package define

import (
	"github.com/containers/image/v5/manifest"
)

// ManifestListDescriptor references a platform-specific manifest.
// Contains exclusive field like `annotations` which is only present in
// OCI spec and not in docker image spec.
type ManifestListDescriptor struct {
	manifest.Schema2Descriptor
	Platform manifest.Schema2PlatformSpec `json:"platform"`
	// Annotations contains arbitrary metadata for the image index.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ManifestListData is a list of platform-specific manifests, specifically used to
// generate output struct for `podman manifest inspect`. Reason for maintaining and
// having this type is to ensure we can have a common type which contains exclusive
// fields from both Docker manifest format and OCI manifest format.
type ManifestListData struct {
	SchemaVersion int                      `json:"schemaVersion"`
	MediaType     string                   `json:"mediaType"`
	Manifests     []ManifestListDescriptor `json:"manifests"`
	// Annotations contains arbitrary metadata for the image index.
	Annotations map[string]string `json:"annotations,omitempty"`
}
