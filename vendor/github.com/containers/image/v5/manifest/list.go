package manifest

import (
	"fmt"

	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	// SupportedListMIMETypes is a list of the manifest list types that we know how to
	// read/manipulate/write.
	SupportedListMIMETypes = []string{
		DockerV2ListMediaType,
		imgspecv1.MediaTypeImageIndex,
	}
)

// List is an interface for parsing, modifying lists of image manifests.
// Callers can either use this abstract interface without understanding the details of the formats,
// or instantiate a specific implementation (e.g. manifest.OCI1Index) and access the public members
// directly.
type List interface {
	// MIMEType returns the MIME type of this particular manifest list.
	MIMEType() string

	// Instances returns a list of the manifests that this list knows of, other than its own.
	Instances() []digest.Digest

	// Update information about the list's instances.  The length of the passed-in slice must
	// match the length of the list of instances which the list already contains, and every field
	// must be specified.
	UpdateInstances([]ListUpdate) error

	// Instance returns the size and MIME type of a particular instance in the list.
	Instance(digest.Digest) (ListUpdate, error)

	// ChooseInstance selects which manifest is most appropriate for the platform described by the
	// SystemContext, or for the current platform if the SystemContext doesn't specify any details.
	ChooseInstance(ctx *types.SystemContext) (digest.Digest, error)

	// Serialize returns the list in a blob format.
	// NOTE: Serialize() does not in general reproduce the original blob if this object was loaded
	// from, even if no modifications were made!
	Serialize() ([]byte, error)

	// ConvertToMIMEType returns the list rebuilt to the specified MIME type, or an error.
	ConvertToMIMEType(mimeType string) (List, error)

	// Clone returns a deep copy of this list and its contents.
	Clone() List
}

// ListUpdate includes the fields which a List's UpdateInstances() method will modify.
type ListUpdate struct {
	Digest    digest.Digest
	Size      int64
	MediaType string
}

// ListFromBlob parses a list of manifests.
func ListFromBlob(manifest []byte, manifestMIMEType string) (List, error) {
	normalized := NormalizedMIMEType(manifestMIMEType)
	switch normalized {
	case DockerV2ListMediaType:
		return Schema2ListFromManifest(manifest)
	case imgspecv1.MediaTypeImageIndex:
		return OCI1IndexFromManifest(manifest)
	case DockerV2Schema1MediaType, DockerV2Schema1SignedMediaType, imgspecv1.MediaTypeImageManifest, DockerV2Schema2MediaType:
		return nil, fmt.Errorf("Treating single images as manifest lists is not implemented")
	}
	return nil, fmt.Errorf("Unimplemented manifest list MIME type %s (normalized as %s)", manifestMIMEType, normalized)
}

// ConvertListToMIMEType converts the passed-in manifest list to a manifest
// list of the specified type.
func ConvertListToMIMEType(list List, manifestMIMEType string) (List, error) {
	return list.ConvertToMIMEType(manifestMIMEType)
}
