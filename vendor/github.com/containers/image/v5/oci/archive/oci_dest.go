package archive

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/containers/image/v5/internal/blobinfocache"
	"github.com/containers/image/v5/internal/imagedestination"
	"github.com/containers/image/v5/internal/imagedestination/impl"
	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/internal/signature"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/archive"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

type ociArchiveImageDestination struct {
	impl.Compat

	ref          ociArchiveReference
	unpackedDest private.ImageDestination
	tempDirRef   tempDirOCIRef
}

// newImageDestination returns an ImageDestination for writing to an existing directory.
func newImageDestination(ctx context.Context, sys *types.SystemContext, ref ociArchiveReference) (private.ImageDestination, error) {
	tempDirRef, err := createOCIRef(sys, ref.image)
	if err != nil {
		return nil, fmt.Errorf("creating oci reference: %w", err)
	}
	unpackedDest, err := tempDirRef.ociRefExtracted.NewImageDestination(ctx, sys)
	if err != nil {
		if err := tempDirRef.deleteTempDir(); err != nil {
			return nil, fmt.Errorf("deleting temp directory %q: %w", tempDirRef.tempDirectory, err)
		}
		return nil, err
	}
	d := &ociArchiveImageDestination{
		ref:          ref,
		unpackedDest: imagedestination.FromPublic(unpackedDest),
		tempDirRef:   tempDirRef,
	}
	d.Compat = impl.AddCompat(d)
	return d, nil
}

// Reference returns the reference used to set up this destination.
func (d *ociArchiveImageDestination) Reference() types.ImageReference {
	return d.ref
}

// Close removes resources associated with an initialized ImageDestination, if any
// Close deletes the temp directory of the oci-archive image
func (d *ociArchiveImageDestination) Close() error {
	defer func() {
		err := d.tempDirRef.deleteTempDir()
		logrus.Debugf("Error deleting temporary directory: %v", err)
	}()
	return d.unpackedDest.Close()
}

func (d *ociArchiveImageDestination) SupportedManifestMIMETypes() []string {
	return d.unpackedDest.SupportedManifestMIMETypes()
}

// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures
func (d *ociArchiveImageDestination) SupportsSignatures(ctx context.Context) error {
	return d.unpackedDest.SupportsSignatures(ctx)
}

func (d *ociArchiveImageDestination) DesiredLayerCompression() types.LayerCompression {
	return d.unpackedDest.DesiredLayerCompression()
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *ociArchiveImageDestination) AcceptsForeignLayerURLs() bool {
	return d.unpackedDest.AcceptsForeignLayerURLs()
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime architecture and OS. False otherwise
func (d *ociArchiveImageDestination) MustMatchRuntimeOS() bool {
	return d.unpackedDest.MustMatchRuntimeOS()
}

// IgnoresEmbeddedDockerReference returns true iff the destination does not care about Image.EmbeddedDockerReferenceConflicts(),
// and would prefer to receive an unmodified manifest instead of one modified for the destination.
// Does not make a difference if Reference().DockerReference() is nil.
func (d *ociArchiveImageDestination) IgnoresEmbeddedDockerReference() bool {
	return d.unpackedDest.IgnoresEmbeddedDockerReference()
}

// HasThreadSafePutBlob indicates whether PutBlob can be executed concurrently.
func (d *ociArchiveImageDestination) HasThreadSafePutBlob() bool {
	return false
}

// SupportsPutBlobPartial returns true if PutBlobPartial is supported.
func (d *ociArchiveImageDestination) SupportsPutBlobPartial() bool {
	return d.unpackedDest.SupportsPutBlobPartial()
}

// PutBlobWithOptions writes contents of stream and returns data representing the result.
// inputInfo.Digest can be optionally provided if known; if provided, and stream is read to the end without error, the digest MUST match the stream contents.
// inputInfo.Size is the expected length of stream, if known.
// inputInfo.MediaType describes the blob format, if known.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *ociArchiveImageDestination) PutBlobWithOptions(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, options private.PutBlobOptions) (types.BlobInfo, error) {
	return d.unpackedDest.PutBlobWithOptions(ctx, stream, inputInfo, options)
}

// PutBlobPartial attempts to create a blob using the data that is already present
// at the destination. chunkAccessor is accessed in a non-sequential way to retrieve the missing chunks.
// It is available only if SupportsPutBlobPartial().
// Even if SupportsPutBlobPartial() returns true, the call can fail, in which case the caller
// should fall back to PutBlobWithOptions.
func (d *ociArchiveImageDestination) PutBlobPartial(ctx context.Context, chunkAccessor private.BlobChunkAccessor, srcInfo types.BlobInfo, cache blobinfocache.BlobInfoCache2) (types.BlobInfo, error) {
	return d.unpackedDest.PutBlobPartial(ctx, chunkAccessor, srcInfo, cache)
}

// TryReusingBlobWithOptions checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If the blob has been successfully reused, returns (true, info, nil); info must contain at least a digest and size, and may
// include CompressionOperation and CompressionAlgorithm fields to indicate that a change to the compression type should be
// reflected in the manifest that will be written.
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
func (d *ociArchiveImageDestination) TryReusingBlobWithOptions(ctx context.Context, info types.BlobInfo, options private.TryReusingBlobOptions) (bool, types.BlobInfo, error) {
	return d.unpackedDest.TryReusingBlobWithOptions(ctx, info, options)
}

// PutManifest writes the manifest to the destination.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to overwrite the manifest for (when
// the primary manifest is a manifest list); this should always be nil if the primary manifest is not a manifest list.
// It is expected but not enforced that the instanceDigest, when specified, matches the digest of `manifest` as generated
// by `manifest.Digest()`.
func (d *ociArchiveImageDestination) PutManifest(ctx context.Context, m []byte, instanceDigest *digest.Digest) error {
	return d.unpackedDest.PutManifest(ctx, m, instanceDigest)
}

// PutSignaturesWithFormat writes a set of signatures to the destination.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to write or overwrite the signatures for
// (when the primary manifest is a manifest list); this should always be nil if the primary manifest is not a manifest list.
// MUST be called after PutManifest (signatures may reference manifest contents).
func (d *ociArchiveImageDestination) PutSignaturesWithFormat(ctx context.Context, signatures []signature.Signature, instanceDigest *digest.Digest) error {
	return d.unpackedDest.PutSignaturesWithFormat(ctx, signatures, instanceDigest)
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted
// unparsedToplevel contains data about the top-level manifest of the source (which may be a single-arch image or a manifest list
// if PutManifest was only called for the single-arch image with instanceDigest == nil), primarily to allow lookups by the
// original manifest list digest, if desired.
// after the directory is made, it is tarred up into a file and the directory is deleted
func (d *ociArchiveImageDestination) Commit(ctx context.Context, unparsedToplevel types.UnparsedImage) error {
	if err := d.unpackedDest.Commit(ctx, unparsedToplevel); err != nil {
		return fmt.Errorf("storing image %q: %w", d.ref.image, err)
	}

	// path of directory to tar up
	src := d.tempDirRef.tempDirectory
	// path to save tarred up file
	dst := d.ref.resolvedFile
	return tarDirectory(src, dst)
}

// tar converts the directory at src and saves it to dst
func tarDirectory(src, dst string) error {
	// input is a stream of bytes from the archive of the directory at path
	input, err := archive.Tar(src, archive.Uncompressed)
	if err != nil {
		return fmt.Errorf("retrieving stream of bytes from %q: %w", src, err)
	}

	// creates the tar file
	outFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating tar file %q: %w", dst, err)
	}
	defer outFile.Close()

	// copies the contents of the directory to the tar file
	// TODO: This can take quite some time, and should ideally be cancellable using a context.Context.
	_, err = io.Copy(outFile, input)

	return err
}
