package tarfile

import (
	"context"
	"io"

	internal "github.com/containers/image/v5/docker/internal/tarfile"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
)

// Destination is a partial implementation of types.ImageDestination for writing to an io.Writer.
type Destination struct {
	internal *internal.Destination
	archive  *internal.Writer
}

// NewDestination returns a tarfile.Destination for the specified io.Writer.
// Deprecated: please use NewDestinationWithContext instead
func NewDestination(dest io.Writer, ref reference.NamedTagged) *Destination {
	return NewDestinationWithContext(nil, dest, ref)
}

// NewDestinationWithContext returns a tarfile.Destination for the specified io.Writer.
func NewDestinationWithContext(sys *types.SystemContext, dest io.Writer, ref reference.NamedTagged) *Destination {
	archive := internal.NewWriter(dest)
	return &Destination{
		internal: internal.NewDestination(sys, archive, ref),
		archive:  archive,
	}
}

// AddRepoTags adds the specified tags to the destination's repoTags.
func (d *Destination) AddRepoTags(tags []reference.NamedTagged) {
	d.internal.AddRepoTags(tags)
}

// SupportedManifestMIMETypes tells which manifest mime types the destination supports
// If an empty slice or nil it's returned, then any mime type can be tried to upload
func (d *Destination) SupportedManifestMIMETypes() []string {
	return d.internal.SupportedManifestMIMETypes()
}

// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures.
// Note: It is still possible for PutSignatures to fail if SupportsSignatures returns nil.
func (d *Destination) SupportsSignatures(ctx context.Context) error {
	return d.internal.SupportsSignatures(ctx)
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *Destination) AcceptsForeignLayerURLs() bool {
	return d.internal.AcceptsForeignLayerURLs()
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime architecture and OS. False otherwise.
func (d *Destination) MustMatchRuntimeOS() bool {
	return d.internal.MustMatchRuntimeOS()
}

// IgnoresEmbeddedDockerReference returns true iff the destination does not care about Image.EmbeddedDockerReferenceConflicts(),
// and would prefer to receive an unmodified manifest instead of one modified for the destination.
// Does not make a difference if Reference().DockerReference() is nil.
func (d *Destination) IgnoresEmbeddedDockerReference() bool {
	return d.internal.IgnoresEmbeddedDockerReference()
}

// HasThreadSafePutBlob indicates whether PutBlob can be executed concurrently.
func (d *Destination) HasThreadSafePutBlob() bool {
	return d.internal.HasThreadSafePutBlob()
}

// PutBlob writes contents of stream and returns data representing the result (with all data filled in).
// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
// inputInfo.Size is the expected length of stream, if known.
// May update cache.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *Destination) PutBlob(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, cache types.BlobInfoCache, isConfig bool) (types.BlobInfo, error) {
	return d.internal.PutBlob(ctx, stream, inputInfo, cache, isConfig)
}

// TryReusingBlob checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If canSubstitute, TryReusingBlob can use an equivalent equivalent of the desired blob; in that case the returned info may not match the input.
// If the blob has been successfully reused, returns (true, info, nil); info must contain at least a digest and size.
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
// May use and/or update cache.
func (d *Destination) TryReusingBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	return d.internal.TryReusingBlob(ctx, info, cache, canSubstitute)
}

// PutManifest writes manifest to the destination.
// The instanceDigest value is expected to always be nil, because this transport does not support manifest lists, so
// there can be no secondary manifests.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *Destination) PutManifest(ctx context.Context, m []byte, instanceDigest *digest.Digest) error {
	return d.internal.PutManifest(ctx, m, instanceDigest)
}

// PutSignatures would add the given signatures to the docker tarfile (currently not supported).
// The instanceDigest value is expected to always be nil, because this transport does not support manifest lists, so
// there can be no secondary manifests.  MUST be called after PutManifest (signatures reference manifest contents).
func (d *Destination) PutSignatures(ctx context.Context, signatures [][]byte, instanceDigest *digest.Digest) error {
	return d.internal.PutSignatures(ctx, signatures, instanceDigest)
}

// Commit finishes writing data to the underlying io.Writer.
// It is the caller's responsibility to close it, if necessary.
func (d *Destination) Commit(ctx context.Context) error {
	return d.archive.Close()
}
