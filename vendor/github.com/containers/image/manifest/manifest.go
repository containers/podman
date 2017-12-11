package manifest

import (
	"encoding/json"

	"github.com/docker/libtrust"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// FIXME: Should we just use docker/distribution and docker/docker implementations directly?

// FIXME(runcom, mitr): should we havea mediatype pkg??
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
	// DockerV2ListMediaType MIME type represents Docker manifest schema 2 list
	DockerV2ListMediaType = "application/vnd.docker.distribution.manifest.list.v2+json"
	// DockerV2Schema2ForeignLayerMediaType is the MIME type used for schema 2 foreign layers.
	DockerV2Schema2ForeignLayerMediaType = "application/vnd.docker.image.rootfs.foreign.diff.tar.gzip"
)

// DefaultRequestedManifestMIMETypes is a list of MIME types a types.ImageSource
// should request from the backend unless directed otherwise.
var DefaultRequestedManifestMIMETypes = []string{
	imgspecv1.MediaTypeImageManifest,
	DockerV2Schema2MediaType,
	DockerV2Schema1SignedMediaType,
	DockerV2Schema1MediaType,
	DockerV2ListMediaType,
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
			Layers []imgspecv1.Descriptor `json:"layers"`
		}{}
		if err := json.Unmarshal(manifest, &ociMan); err != nil {
			return ""
		}
		if ociMan.Config.MediaType == imgspecv1.MediaTypeImageConfig && len(ociMan.Layers) != 0 {
			return imgspecv1.MediaTypeImageManifest
		}
		ociIndex := struct {
			Manifests []imgspecv1.Descriptor `json:"manifests"`
		}{}
		if err := json.Unmarshal(manifest, &ociIndex); err != nil {
			return ""
		}
		if len(ociIndex.Manifests) != 0 && ociIndex.Manifests[0].MediaType == imgspecv1.MediaTypeImageManifest {
			return imgspecv1.MediaTypeImageIndex
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
	return mimeType == DockerV2ListMediaType
}
