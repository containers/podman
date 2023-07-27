package openshift

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/blobinfocache"
	"github.com/containers/image/v5/internal/imagedestination"
	"github.com/containers/image/v5/internal/imagedestination/impl"
	"github.com/containers/image/v5/internal/imagedestination/stubs"
	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/internal/set"
	"github.com/containers/image/v5/internal/signature"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	"golang.org/x/exp/slices"
)

type openshiftImageDestination struct {
	impl.Compat
	stubs.AlwaysSupportsSignatures

	client *openshiftClient
	docker private.ImageDestination // The docker/distribution API endpoint
	// State
	imageStreamImageName string // "" if not yet known
}

// newImageDestination creates a new ImageDestination for the specified reference.
func newImageDestination(ctx context.Context, sys *types.SystemContext, ref openshiftReference) (private.ImageDestination, error) {
	client, err := newOpenshiftClient(ref)
	if err != nil {
		return nil, err
	}

	// FIXME: Should this always use a digest, not a tag? Uploading to Docker by tag requires the tag _inside_ the manifest to match,
	// i.e. a single signed image cannot be available under multiple tags.  But with types.ImageDestination, we don't know
	// the manifest digest at this point.
	dockerRefString := fmt.Sprintf("//%s/%s/%s:%s", reference.Domain(client.ref.dockerReference), client.ref.namespace, client.ref.stream, client.ref.dockerReference.Tag())
	dockerRef, err := docker.ParseReference(dockerRefString)
	if err != nil {
		return nil, err
	}
	docker, err := dockerRef.NewImageDestination(ctx, sys)
	if err != nil {
		return nil, err
	}

	d := &openshiftImageDestination{
		client: client,
		docker: imagedestination.FromPublic(docker),
	}
	d.Compat = impl.AddCompat(d)
	return d, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *openshiftImageDestination) Reference() types.ImageReference {
	return d.client.ref
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *openshiftImageDestination) Close() error {
	return d.docker.Close()
}

func (d *openshiftImageDestination) SupportedManifestMIMETypes() []string {
	return d.docker.SupportedManifestMIMETypes()
}

func (d *openshiftImageDestination) DesiredLayerCompression() types.LayerCompression {
	return types.Compress
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *openshiftImageDestination) AcceptsForeignLayerURLs() bool {
	return true
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime architecture and OS. False otherwise.
func (d *openshiftImageDestination) MustMatchRuntimeOS() bool {
	return false
}

// IgnoresEmbeddedDockerReference returns true iff the destination does not care about Image.EmbeddedDockerReferenceConflicts(),
// and would prefer to receive an unmodified manifest instead of one modified for the destination.
// Does not make a difference if Reference().DockerReference() is nil.
func (d *openshiftImageDestination) IgnoresEmbeddedDockerReference() bool {
	return d.docker.IgnoresEmbeddedDockerReference()
}

// HasThreadSafePutBlob indicates whether PutBlob can be executed concurrently.
func (d *openshiftImageDestination) HasThreadSafePutBlob() bool {
	return false
}

// SupportsPutBlobPartial returns true if PutBlobPartial is supported.
func (d *openshiftImageDestination) SupportsPutBlobPartial() bool {
	return d.docker.SupportsPutBlobPartial()
}

// PutBlobWithOptions writes contents of stream and returns data representing the result.
// inputInfo.Digest can be optionally provided if known; if provided, and stream is read to the end without error, the digest MUST match the stream contents.
// inputInfo.Size is the expected length of stream, if known.
// inputInfo.MediaType describes the blob format, if known.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlobWithOptions MUST 1) fail, and 2) delete any data stored so far.
func (d *openshiftImageDestination) PutBlobWithOptions(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, options private.PutBlobOptions) (private.UploadedBlob, error) {
	return d.docker.PutBlobWithOptions(ctx, stream, inputInfo, options)
}

// PutBlobPartial attempts to create a blob using the data that is already present
// at the destination. chunkAccessor is accessed in a non-sequential way to retrieve the missing chunks.
// It is available only if SupportsPutBlobPartial().
// Even if SupportsPutBlobPartial() returns true, the call can fail, in which case the caller
// should fall back to PutBlobWithOptions.
func (d *openshiftImageDestination) PutBlobPartial(ctx context.Context, chunkAccessor private.BlobChunkAccessor, srcInfo types.BlobInfo, cache blobinfocache.BlobInfoCache2) (private.UploadedBlob, error) {
	return d.docker.PutBlobPartial(ctx, chunkAccessor, srcInfo, cache)
}

// TryReusingBlobWithOptions checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If the blob has been successfully reused, returns (true, info, nil).
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
func (d *openshiftImageDestination) TryReusingBlobWithOptions(ctx context.Context, info types.BlobInfo, options private.TryReusingBlobOptions) (bool, private.ReusedBlob, error) {
	return d.docker.TryReusingBlobWithOptions(ctx, info, options)
}

// PutManifest writes manifest to the destination.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *openshiftImageDestination) PutManifest(ctx context.Context, m []byte, instanceDigest *digest.Digest) error {
	if instanceDigest == nil {
		manifestDigest, err := manifest.Digest(m)
		if err != nil {
			return err
		}
		d.imageStreamImageName = manifestDigest.String()
	}
	return d.docker.PutManifest(ctx, m, instanceDigest)
}

// PutSignaturesWithFormat writes a set of signatures to the destination.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to write or overwrite the signatures for
// (when the primary manifest is a manifest list); this should always be nil if the primary manifest is not a manifest list.
// MUST be called after PutManifest (signatures may reference manifest contents).
func (d *openshiftImageDestination) PutSignaturesWithFormat(ctx context.Context, signatures []signature.Signature, instanceDigest *digest.Digest) error {
	var imageStreamImageName string
	if instanceDigest == nil {
		if d.imageStreamImageName == "" {
			return errors.New("Internal error: Unknown manifest digest, can't add signatures")
		}
		imageStreamImageName = d.imageStreamImageName
	} else {
		imageStreamImageName = instanceDigest.String()
	}

	// Because image signatures are a shared resource in Atomic Registry, the default upload
	// always adds signatures.  Eventually we should also allow removing signatures.

	if len(signatures) == 0 {
		return nil // No need to even read the old state.
	}

	image, err := d.client.getImage(ctx, imageStreamImageName)
	if err != nil {
		return err
	}
	existingSigNames := set.New[string]()
	for _, sig := range image.Signatures {
		existingSigNames.Add(sig.objectMeta.Name)
	}

	for _, newSigWithFormat := range signatures {
		newSigSimple, ok := newSigWithFormat.(signature.SimpleSigning)
		if !ok {
			return signature.UnsupportedFormatError(newSigWithFormat)
		}
		newSig := newSigSimple.UntrustedSignature()

		if slices.ContainsFunc(image.Signatures, func(existingSig imageSignature) bool {
			return existingSig.Type == imageSignatureTypeAtomic && bytes.Equal(existingSig.Content, newSig)
		}) {
			continue
		}

		// The API expect us to invent a new unique name. This is racy, but hopefully good enough.
		var signatureName string
		for {
			randBytes := make([]byte, 16)
			n, err := rand.Read(randBytes)
			if err != nil || n != 16 {
				return fmt.Errorf("generating random signature len %d: %w", n, err)
			}
			signatureName = fmt.Sprintf("%s@%032x", imageStreamImageName, randBytes)
			if !existingSigNames.Contains(signatureName) {
				break
			}
		}
		// Note: This does absolutely no kind/version checking or conversions.
		sig := imageSignature{
			typeMeta: typeMeta{
				Kind:       "ImageSignature",
				APIVersion: "v1",
			},
			objectMeta: objectMeta{Name: signatureName},
			Type:       imageSignatureTypeAtomic,
			Content:    newSig,
		}
		body, err := json.Marshal(sig)
		if err != nil {
			return err
		}
		_, err = d.client.doRequest(ctx, http.MethodPost, "/oapi/v1/imagesignatures", body)
		if err != nil {
			return err
		}
	}

	return nil
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// unparsedToplevel contains data about the top-level manifest of the source (which may be a single-arch image or a manifest list
// if PutManifest was only called for the single-arch image with instanceDigest == nil), primarily to allow lookups by the
// original manifest list digest, if desired.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *openshiftImageDestination) Commit(ctx context.Context, unparsedToplevel types.UnparsedImage) error {
	return d.docker.Commit(ctx, unparsedToplevel)
}
