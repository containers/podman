package layout

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
	imgspec "github.com/opencontainers/image-spec/specs-go"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ociImageDestination struct {
	ref                      ociReference
	index                    imgspecv1.Index
	sharedBlobDir            string
	acceptUncompressedLayers bool
}

// newImageDestination returns an ImageDestination for writing to an existing directory.
func newImageDestination(sys *types.SystemContext, ref ociReference) (types.ImageDestination, error) {
	var index *imgspecv1.Index
	if indexExists(ref) {
		var err error
		index, err = ref.getIndex()
		if err != nil {
			return nil, err
		}
	} else {
		index = &imgspecv1.Index{
			Versioned: imgspec.Versioned{
				SchemaVersion: 2,
			},
			Annotations: make(map[string]string),
		}
	}

	d := &ociImageDestination{ref: ref, index: *index}
	if sys != nil {
		d.sharedBlobDir = sys.OCISharedBlobDirPath
		d.acceptUncompressedLayers = sys.OCIAcceptUncompressedLayers
	}

	if err := ensureDirectoryExists(d.ref.dir); err != nil {
		return nil, err
	}
	// Per the OCI image specification, layouts MUST have a "blobs" subdirectory,
	// but it MAY be empty (e.g. if we never end up calling PutBlob)
	// https://github.com/opencontainers/image-spec/blame/7c889fafd04a893f5c5f50b7ab9963d5d64e5242/image-layout.md#L19
	if err := ensureDirectoryExists(filepath.Join(d.ref.dir, "blobs")); err != nil {
		return nil, err
	}
	return d, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *ociImageDestination) Reference() types.ImageReference {
	return d.ref
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *ociImageDestination) Close() error {
	return nil
}

func (d *ociImageDestination) SupportedManifestMIMETypes() []string {
	return []string{
		imgspecv1.MediaTypeImageManifest,
		imgspecv1.MediaTypeImageIndex,
	}
}

// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures.
// Note: It is still possible for PutSignatures to fail if SupportsSignatures returns nil.
func (d *ociImageDestination) SupportsSignatures(ctx context.Context) error {
	return errors.Errorf("Pushing signatures for OCI images is not supported")
}

func (d *ociImageDestination) DesiredLayerCompression() types.LayerCompression {
	if d.acceptUncompressedLayers {
		return types.PreserveOriginal
	}
	return types.Compress
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *ociImageDestination) AcceptsForeignLayerURLs() bool {
	return true
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime architecture and OS. False otherwise.
func (d *ociImageDestination) MustMatchRuntimeOS() bool {
	return false
}

// IgnoresEmbeddedDockerReference returns true iff the destination does not care about Image.EmbeddedDockerReferenceConflicts(),
// and would prefer to receive an unmodified manifest instead of one modified for the destination.
// Does not make a difference if Reference().DockerReference() is nil.
func (d *ociImageDestination) IgnoresEmbeddedDockerReference() bool {
	return false // N/A, DockerReference() returns nil.
}

// HasThreadSafePutBlob indicates whether PutBlob can be executed concurrently.
func (d *ociImageDestination) HasThreadSafePutBlob() bool {
	return false
}

// PutBlob writes contents of stream and returns data representing the result.
// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
// inputInfo.Size is the expected length of stream, if known.
// inputInfo.MediaType describes the blob format, if known.
// May update cache.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *ociImageDestination) PutBlob(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, cache types.BlobInfoCache, isConfig bool) (types.BlobInfo, error) {
	blobFile, err := ioutil.TempFile(d.ref.dir, "oci-put-blob")
	if err != nil {
		return types.BlobInfo{}, err
	}
	succeeded := false
	explicitClosed := false
	defer func() {
		if !explicitClosed {
			blobFile.Close()
		}
		if !succeeded {
			os.Remove(blobFile.Name())
		}
	}()

	digester := digest.Canonical.Digester()
	tee := io.TeeReader(stream, digester.Hash())

	// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
	size, err := io.Copy(blobFile, tee)
	if err != nil {
		return types.BlobInfo{}, err
	}
	computedDigest := digester.Digest()
	if inputInfo.Size != -1 && size != inputInfo.Size {
		return types.BlobInfo{}, errors.Errorf("Size mismatch when copying %s, expected %d, got %d", computedDigest, inputInfo.Size, size)
	}
	if err := blobFile.Sync(); err != nil {
		return types.BlobInfo{}, err
	}

	// On POSIX systems, blobFile was created with mode 0600, so we need to make it readable.
	// On Windows, the “permissions of newly created files” argument to syscall.Open is
	// ignored and the file is already readable; besides, blobFile.Chmod, i.e. syscall.Fchmod,
	// always fails on Windows.
	if runtime.GOOS != "windows" {
		if err := blobFile.Chmod(0644); err != nil {
			return types.BlobInfo{}, err
		}
	}

	blobPath, err := d.ref.blobPath(computedDigest, d.sharedBlobDir)
	if err != nil {
		return types.BlobInfo{}, err
	}
	if err := ensureParentDirectoryExists(blobPath); err != nil {
		return types.BlobInfo{}, err
	}

	// need to explicitly close the file, since a rename won't otherwise not work on Windows
	blobFile.Close()
	explicitClosed = true
	if err := os.Rename(blobFile.Name(), blobPath); err != nil {
		return types.BlobInfo{}, err
	}
	succeeded = true
	return types.BlobInfo{Digest: computedDigest, Size: size}, nil
}

// TryReusingBlob checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If canSubstitute, TryReusingBlob can use an equivalent equivalent of the desired blob; in that case the returned info may not match the input.
// If the blob has been successfully reused, returns (true, info, nil); info must contain at least a digest and size, and may
// include CompressionOperation and CompressionAlgorithm fields to indicate that a change to the compression type should be
// reflected in the manifest that will be written.
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
// May use and/or update cache.
func (d *ociImageDestination) TryReusingBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	if info.Digest == "" {
		return false, types.BlobInfo{}, errors.Errorf(`"Can not check for a blob with unknown digest`)
	}
	blobPath, err := d.ref.blobPath(info.Digest, d.sharedBlobDir)
	if err != nil {
		return false, types.BlobInfo{}, err
	}
	finfo, err := os.Stat(blobPath)
	if err != nil && os.IsNotExist(err) {
		return false, types.BlobInfo{}, nil
	}
	if err != nil {
		return false, types.BlobInfo{}, err
	}

	return true, types.BlobInfo{Digest: info.Digest, Size: finfo.Size()}, nil
}

// PutManifest writes a manifest to the destination.  Per our list of supported manifest MIME types,
// this should be either an OCI manifest (possibly converted to this format by the caller) or index,
// neither of which we'll need to modify further.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to overwrite the manifest for (when
// the primary manifest is a manifest list); this should always be nil if the primary manifest is not a manifest list.
// It is expected but not enforced that the instanceDigest, when specified, matches the digest of `manifest` as generated
// by `manifest.Digest()`.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *ociImageDestination) PutManifest(ctx context.Context, m []byte, instanceDigest *digest.Digest) error {
	var digest digest.Digest
	var err error
	if instanceDigest != nil {
		digest = *instanceDigest
	} else {
		digest, err = manifest.Digest(m)
		if err != nil {
			return err
		}
	}

	blobPath, err := d.ref.blobPath(digest, d.sharedBlobDir)
	if err != nil {
		return err
	}
	if err := ensureParentDirectoryExists(blobPath); err != nil {
		return err
	}
	if err := ioutil.WriteFile(blobPath, m, 0644); err != nil {
		return err
	}

	if instanceDigest != nil {
		return nil
	}

	// If we had platform information, we'd build an imgspecv1.Platform structure here.

	// Start filling out the descriptor for this entry
	desc := imgspecv1.Descriptor{}
	desc.Digest = digest
	desc.Size = int64(len(m))
	if d.ref.image != "" {
		desc.Annotations = make(map[string]string)
		desc.Annotations[imgspecv1.AnnotationRefName] = d.ref.image
	}

	// If we knew the MIME type, we wouldn't have to guess here.
	desc.MediaType = manifest.GuessMIMEType(m)

	d.addManifest(&desc)

	return nil
}

func (d *ociImageDestination) addManifest(desc *imgspecv1.Descriptor) {
	// If the new entry has a name, remove any conflicting names which we already have.
	if desc.Annotations != nil && desc.Annotations[imgspecv1.AnnotationRefName] != "" {
		// The name is being set on a new entry, so remove any older ones that had the same name.
		// We might be storing an index and all of its component images, and we'll want to attach
		// the name to the last one, which is the index.
		for i, manifest := range d.index.Manifests {
			if manifest.Annotations[imgspecv1.AnnotationRefName] == desc.Annotations[imgspecv1.AnnotationRefName] {
				delete(d.index.Manifests[i].Annotations, imgspecv1.AnnotationRefName)
				break
			}
		}
	}
	// If it has the same digest as another entry in the index, we already overwrote the file,
	// so just pick up the other information.
	for i, manifest := range d.index.Manifests {
		if manifest.Digest == desc.Digest && manifest.Annotations[imgspecv1.AnnotationRefName] == "" {
			// Replace it completely.
			d.index.Manifests[i] = *desc
			return
		}
	}
	// It's a new entry to be added to the index.
	d.index.Manifests = append(d.index.Manifests, *desc)
}

// PutSignatures would add the given signatures to the oci layout (currently not supported).
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to write or overwrite the signatures for
// (when the primary manifest is a manifest list); this should always be nil if the primary manifest is not a manifest list.
func (d *ociImageDestination) PutSignatures(ctx context.Context, signatures [][]byte, instanceDigest *digest.Digest) error {
	if len(signatures) != 0 {
		return errors.Errorf("Pushing signatures for OCI images is not supported")
	}
	return nil
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *ociImageDestination) Commit(context.Context, types.UnparsedImage) error {
	if err := ioutil.WriteFile(d.ref.ociLayoutPath(), []byte(`{"imageLayoutVersion": "1.0.0"}`), 0644); err != nil {
		return err
	}
	indexJSON, err := json.Marshal(d.index)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(d.ref.indexPath(), indexJSON, 0644)
}

func ensureDirectoryExists(path string) error {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

// ensureParentDirectoryExists ensures the parent of the supplied path exists.
func ensureParentDirectoryExists(path string) error {
	return ensureDirectoryExists(filepath.Dir(path))
}

// indexExists checks whether the index location specified in the OCI reference exists.
// The implementation is opinionated, since in case of unexpected errors false is returned
func indexExists(ref ociReference) bool {
	_, err := os.Stat(ref.indexPath())
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}
