package manifest

import (
	"encoding/json"
	"fmt"

	"github.com/containers/image/v5/types"
	"github.com/containers/libtrust"
	digest "github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// FIXME: Should we just use docker/distribution and docker/docker implementations directly?

// FIXME(runcom, mitr): should we have a mediatype pkg??
const (
	// DockerV2Schema1MediaType MIME type represents Docker manifest schema 1
	DockerV2Schema1MediaType = "application/vnd.docker.distribution.manifest.v1+json"
	// DockerV2Schema1MediaType MIME type represents Docker manifest schema 1 with a JWS signature
	DockerV2Schema1SignedMediaType = "application/vnd.docker.distribution.manifest.v1+prettyjws"
	// DockerV2Schema2MediaType MIME type represents Docker manifest schema 2
	DockerV2Schema2MediaType = "application/vnd.docker.distribution.manifest.v2+json"
	// DockerV2Schema2ConfigMediaType is the MIME type used for schema 2 config blobs.
	DockerV2Schema2ConfigMediaType = "application/vnd.docker.container.image.v1+json"
	// DockerV2Schema2LayerMediaType is the MIME type used for schema 2 layers.
	DockerV2Schema2LayerMediaType = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	// DockerV2SchemaLayerMediaTypeUncompressed is the mediaType used for uncompressed layers.
	DockerV2SchemaLayerMediaTypeUncompressed = "application/vnd.docker.image.rootfs.diff.tar"
	// DockerV2ListMediaType MIME type represents Docker manifest schema 2 list
	DockerV2ListMediaType = "application/vnd.docker.distribution.manifest.list.v2+json"
	// DockerV2Schema2ForeignLayerMediaType is the MIME type used for schema 2 foreign layers.
	DockerV2Schema2ForeignLayerMediaType = "application/vnd.docker.image.rootfs.foreign.diff.tar"
	// DockerV2Schema2ForeignLayerMediaType is the MIME type used for gzippped schema 2 foreign layers.
	DockerV2Schema2ForeignLayerMediaTypeGzip = "application/vnd.docker.image.rootfs.foreign.diff.tar.gzip"
)

// SupportedSchema2MediaType checks if the specified string is a supported Docker v2s2 media type.
func SupportedSchema2MediaType(m string) error {
	switch m {
	case DockerV2ListMediaType, DockerV2Schema1MediaType, DockerV2Schema1SignedMediaType, DockerV2Schema2ConfigMediaType, DockerV2Schema2ForeignLayerMediaType, DockerV2Schema2ForeignLayerMediaTypeGzip, DockerV2Schema2LayerMediaType, DockerV2Schema2MediaType, DockerV2SchemaLayerMediaTypeUncompressed:
		return nil
	default:
		return fmt.Errorf("unsupported docker v2s2 media type: %q", m)
	}
}

// DefaultRequestedManifestMIMETypes is a list of MIME types a types.ImageSource
// should request from the backend unless directed otherwise.
var DefaultRequestedManifestMIMETypes = []string{
	imgspecv1.MediaTypeImageManifest,
	DockerV2Schema2MediaType,
	DockerV2Schema1SignedMediaType,
	DockerV2Schema1MediaType,
	DockerV2ListMediaType,
	imgspecv1.MediaTypeImageIndex,
}

// Manifest is an interface for parsing, modifying image manifests in isolation.
// Callers can either use this abstract interface without understanding the details of the formats,
// or instantiate a specific implementation (e.g. manifest.OCI1) and access the public members
// directly.
//
// See types.Image for functionality not limited to manifests, including format conversions and config parsing.
// This interface is similar to, but not strictly equivalent to, the equivalent methods in types.Image.
type Manifest interface {
	// ConfigInfo returns a complete BlobInfo for the separate config object, or a BlobInfo{Digest:""} if there isn't a separate object.
	ConfigInfo() types.BlobInfo
	// LayerInfos returns a list of LayerInfos of layers referenced by this image, in order (the root layer first, and then successive layered layers).
	// The Digest field is guaranteed to be provided; Size may be -1.
	// WARNING: The list may contain duplicates, and they are semantically relevant.
	LayerInfos() []LayerInfo
	// UpdateLayerInfos replaces the original layers with the specified BlobInfos (size+digest+urls), in order (the root layer first, and then successive layered layers)
	UpdateLayerInfos(layerInfos []types.BlobInfo) error

	// ImageID computes an ID which can uniquely identify this image by its contents, irrespective
	// of which (of possibly more than one simultaneously valid) reference was used to locate the
	// image, and unchanged by whether or how the layers are compressed.  The result takes the form
	// of the hexadecimal portion of a digest.Digest.
	ImageID(diffIDs []digest.Digest) (string, error)

	// Inspect returns various information for (skopeo inspect) parsed from the manifest,
	// incorporating information from a configuration blob returned by configGetter, if
	// the underlying image format is expected to include a configuration blob.
	Inspect(configGetter func(types.BlobInfo) ([]byte, error)) (*types.ImageInspectInfo, error)

	// Serialize returns the manifest in a blob format.
	// NOTE: Serialize() does not in general reproduce the original blob if this object was loaded from one, even if no modifications were made!
	Serialize() ([]byte, error)
}

// LayerInfo is an extended version of types.BlobInfo for low-level users of Manifest.LayerInfos.
type LayerInfo struct {
	types.BlobInfo
	EmptyLayer bool // The layer is an “empty”/“throwaway” one, and may or may not be physically represented in various transport / storage systems.  false if the manifest type does not have the concept.
}

// GuessMIMEType guesses MIME type of a manifest and returns it _if it is recognized_, or "" if unknown or unrecognized.
// FIXME? We should, in general, prefer out-of-band MIME type instead of blindly parsing the manifest,
// but we may not have such metadata available (e.g. when the manifest is a local file).
func GuessMIMEType(manifest []byte) string {
	// A subset of manifest fields; the rest is silently ignored by json.Unmarshal.
	// Also docker/distribution/manifest.Versioned.
	meta := struct {
		MediaType     string      `json:"mediaType"`
		SchemaVersion int         `json:"schemaVersion"`
		Signatures    interface{} `json:"signatures"`
	}{}
	if err := json.Unmarshal(manifest, &meta); err != nil {
		return ""
	}

	switch meta.MediaType {
	case DockerV2Schema2MediaType, DockerV2ListMediaType: // A recognized type.
		return meta.MediaType
	}
	// this is the only way the function can return DockerV2Schema1MediaType, and recognizing that is essential for stripping the JWS signatures = computing the correct manifest digest.
	switch meta.SchemaVersion {
	case 1:
		if meta.Signatures != nil {
			return DockerV2Schema1SignedMediaType
		}
		return DockerV2Schema1MediaType
	case 2:
		// best effort to understand if this is an OCI image since mediaType
		// isn't in the manifest for OCI anymore
		// for docker v2s2 meta.MediaType should have been set. But given the data, this is our best guess.
		ociMan := struct {
			Config struct {
				MediaType string `json:"mediaType"`
			} `json:"config"`
		}{}
		if err := json.Unmarshal(manifest, &ociMan); err != nil {
			return ""
		}
		if ociMan.Config.MediaType == imgspecv1.MediaTypeImageConfig {
			return imgspecv1.MediaTypeImageManifest
		}
		ociIndex := struct {
			Manifests []imgspecv1.Descriptor `json:"manifests"`
		}{}
		if err := json.Unmarshal(manifest, &ociIndex); err != nil {
			return ""
		}
		if len(ociIndex.Manifests) != 0 {
			if ociMan.Config.MediaType == "" {
				return imgspecv1.MediaTypeImageIndex
			}
			return ociMan.Config.MediaType
		}
		return DockerV2Schema2MediaType
	}
	return ""
}

// Digest returns the a digest of a docker manifest, with any necessary implied transformations like stripping v1s1 signatures.
func Digest(manifest []byte) (digest.Digest, error) {
	if GuessMIMEType(manifest) == DockerV2Schema1SignedMediaType {
		sig, err := libtrust.ParsePrettySignature(manifest, "signatures")
		if err != nil {
			return "", err
		}
		manifest, err = sig.Payload()
		if err != nil {
			// Coverage: This should never happen, libtrust's Payload() can fail only if joseBase64UrlDecode() fails, on a string
			// that libtrust itself has josebase64UrlEncode()d
			return "", err
		}
	}

	return digest.FromBytes(manifest), nil
}

// MatchesDigest returns true iff the manifest matches expectedDigest.
// Error may be set if this returns false.
// Note that this is not doing ConstantTimeCompare; by the time we get here, the cryptographic signature must already have been verified,
// or we are not using a cryptographic channel and the attacker can modify the digest along with the manifest blob.
func MatchesDigest(manifest []byte, expectedDigest digest.Digest) (bool, error) {
	// This should eventually support various digest types.
	actualDigest, err := Digest(manifest)
	if err != nil {
		return false, err
	}
	return expectedDigest == actualDigest, nil
}

// AddDummyV2S1Signature adds an JWS signature with a temporary key (i.e. useless) to a v2s1 manifest.
// This is useful to make the manifest acceptable to a Docker Registry (even though nothing needs or wants the JWS signature).
func AddDummyV2S1Signature(manifest []byte) ([]byte, error) {
	key, err := libtrust.GenerateECP256PrivateKey()
	if err != nil {
		return nil, err // Coverage: This can fail only if rand.Reader fails.
	}

	js, err := libtrust.NewJSONSignature(manifest)
	if err != nil {
		return nil, err
	}
	if err := js.Sign(key); err != nil { // Coverage: This can fail basically only if rand.Reader fails.
		return nil, err
	}
	return js.PrettySignature("signatures")
}

// MIMETypeIsMultiImage returns true if mimeType is a list of images
func MIMETypeIsMultiImage(mimeType string) bool {
	return mimeType == DockerV2ListMediaType || mimeType == imgspecv1.MediaTypeImageIndex
}

// MIMETypeSupportsEncryption returns true if the mimeType supports encryption
func MIMETypeSupportsEncryption(mimeType string) bool {
	return mimeType == imgspecv1.MediaTypeImageManifest
}

// NormalizedMIMEType returns the effective MIME type of a manifest MIME type returned by a server,
// centralizing various workarounds.
func NormalizedMIMEType(input string) string {
	switch input {
	// "application/json" is a valid v2s1 value per https://github.com/docker/distribution/blob/master/docs/spec/manifest-v2-1.md .
	// This works for now, when nothing else seems to return "application/json"; if that were not true, the mapping/detection might
	// need to happen within the ImageSource.
	case "application/json":
		return DockerV2Schema1SignedMediaType
	case DockerV2Schema1MediaType, DockerV2Schema1SignedMediaType,
		imgspecv1.MediaTypeImageManifest,
		imgspecv1.MediaTypeImageIndex,
		DockerV2Schema2MediaType,
		DockerV2ListMediaType:
		return input
	default:
		// If it's not a recognized manifest media type, or we have failed determining the type, we'll try one last time
		// to deserialize using v2s1 as per https://github.com/docker/distribution/blob/master/manifests.go#L108
		// and https://github.com/docker/distribution/blob/master/manifest/schema1/manifest.go#L50
		//
		// Crane registries can also return "text/plain", or pretty much anything else depending on a file extension “recognized” in the tag.
		// This makes no real sense, but it happens
		// because requests for manifests are
		// redirected to a content distribution
		// network which is configured that way. See https://bugzilla.redhat.com/show_bug.cgi?id=1389442
		return DockerV2Schema1SignedMediaType
	}
}

// FromBlob returns a Manifest instance for the specified manifest blob and the corresponding MIME type
func FromBlob(manblob []byte, mt string) (Manifest, error) {
	nmt := NormalizedMIMEType(mt)
	switch nmt {
	case DockerV2Schema1MediaType, DockerV2Schema1SignedMediaType:
		return Schema1FromManifest(manblob)
	case imgspecv1.MediaTypeImageManifest:
		return OCI1FromManifest(manblob)
	case DockerV2Schema2MediaType:
		return Schema2FromManifest(manblob)
	case DockerV2ListMediaType, imgspecv1.MediaTypeImageIndex:
		return nil, fmt.Errorf("Treating manifest lists as individual manifests is not implemented")
	}
	// Note that this may not be reachable, NormalizedMIMEType has a default for unknown values.
	return nil, fmt.Errorf("Unimplemented manifest MIME type %s (normalized as %s)", mt, nmt)
}
