package layout

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/containers/image/v5/internal/imagedestination/impl"
	"github.com/containers/image/v5/internal/imagedestination/stubs"
	"github.com/containers/image/v5/internal/manifest"
	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/internal/putblobdigest"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
	imgspec "github.com/opencontainers/image-spec/specs-go"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type ociImageDestination struct {
	impl.Compat
	impl.PropertyMethodsInitialize
	stubs.NoPutBlobPartialInitialize
	stubs.NoSignaturesInitialize

	ref           ociReference
	index         imgspecv1.Index
	sharedBlobDir string
}

// newImageDestination returns an ImageDestination for writing to an existing directory.
func newImageDestination(sys *types.SystemContext, ref ociReference) (private.ImageDestination, error) {
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

	desiredLayerCompression := types.Compress
	if sys != nil && sys.OCIAcceptUncompressedLayers {
		desiredLayerCompression = types.PreserveOriginal
	}

	d := &ociImageDestination{
		PropertyMethodsInitialize: impl.PropertyMethods(impl.Properties{
			SupportedManifestMIMETypes: []string{
				imgspecv1.MediaTypeImageManifest,
				imgspecv1.MediaTypeImageIndex,
			},
			DesiredLayerCompression:        desiredLayerCompression,
			AcceptsForeignLayerURLs:        true,
			MustMatchRuntimeOS:             false,
			IgnoresEmbeddedDockerReference: false, // N/A, DockerReference() returns nil.
			HasThreadSafePutBlob:           true,
		}),
		NoPutBlobPartialInitialize: stubs.NoPutBlobPartial(ref),
		NoSignaturesInitialize:     stubs.NoSignatures("Pushing signatures for OCI images is not supported"),

		ref:   ref,
		index: *index,
	}
	d.Compat = impl.AddCompat(d)
	if sys != nil {
		d.sharedBlobDir = sys.OCISharedBlobDirPath
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

// PutBlobWithOptions writes contents of stream and returns data representing the result.
// inputInfo.Digest can be optionally provided if known; if provided, and stream is read to the end without error, the digest MUST match the stream contents.
// inputInfo.Size is the expected length of stream, if known.
// inputInfo.MediaType describes the blob format, if known.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlobWithOptions MUST 1) fail, and 2) delete any data stored so far.
func (d *ociImageDestination) PutBlobWithOptions(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, options private.PutBlobOptions) (private.UploadedBlob, error) {
	blobFile, err := os.CreateTemp(d.ref.dir, "oci-put-blob")
	if err != nil {
		return private.UploadedBlob{}, err
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

	digester, stream := putblobdigest.DigestIfCanonicalUnknown(stream, inputInfo)
	// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
	size, err := io.Copy(blobFile, stream)
	if err != nil {
		return private.UploadedBlob{}, err
	}
	blobDigest := digester.Digest()
	if inputInfo.Size != -1 && size != inputInfo.Size {
		return private.UploadedBlob{}, fmt.Errorf("Size mismatch when copying %s, expected %d, got %d", blobDigest, inputInfo.Size, size)
	}
	if err := blobFile.Sync(); err != nil {
		return private.UploadedBlob{}, err
	}

	// On POSIX systems, blobFile was created with mode 0600, so we need to make it readable.
	// On Windows, the “permissions of newly created files” argument to syscall.Open is
	// ignored and the file is already readable; besides, blobFile.Chmod, i.e. syscall.Fchmod,
	// always fails on Windows.
	if runtime.GOOS != "windows" {
		if err := blobFile.Chmod(0644); err != nil {
			return private.UploadedBlob{}, err
		}
	}

	blobPath, err := d.ref.blobPath(blobDigest, d.sharedBlobDir)
	if err != nil {
		return private.UploadedBlob{}, err
	}
	if err := ensureParentDirectoryExists(blobPath); err != nil {
		return private.UploadedBlob{}, err
	}

	// need to explicitly close the file, since a rename won't otherwise not work on Windows
	blobFile.Close()
	explicitClosed = true
	if err := os.Rename(blobFile.Name(), blobPath); err != nil {
		return private.UploadedBlob{}, err
	}
	succeeded = true
	return private.UploadedBlob{Digest: blobDigest, Size: size}, nil
}

// TryReusingBlobWithOptions checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If the blob has been successfully reused, returns (true, info, nil).
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
func (d *ociImageDestination) TryReusingBlobWithOptions(ctx context.Context, info types.BlobInfo, options private.TryReusingBlobOptions) (bool, private.ReusedBlob, error) {
	if info.Digest == "" {
		return false, private.ReusedBlob{}, errors.New("Can not check for a blob with unknown digest")
	}
	blobPath, err := d.ref.blobPath(info.Digest, d.sharedBlobDir)
	if err != nil {
		return false, private.ReusedBlob{}, err
	}
	finfo, err := os.Stat(blobPath)
	if err != nil && os.IsNotExist(err) {
		return false, private.ReusedBlob{}, nil
	}
	if err != nil {
		return false, private.ReusedBlob{}, err
	}

	return true, private.ReusedBlob{Digest: info.Digest, Size: finfo.Size()}, nil
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
	if err := os.WriteFile(blobPath, m, 0644); err != nil {
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

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// unparsedToplevel contains data about the top-level manifest of the source (which may be a single-arch image or a manifest list
// if PutManifest was only called for the single-arch image with instanceDigest == nil), primarily to allow lookups by the
// original manifest list digest, if desired.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *ociImageDestination) Commit(context.Context, types.UnparsedImage) error {
	if err := os.WriteFile(d.ref.ociLayoutPath(), []byte(`{"imageLayoutVersion": "1.0.0"}`), 0644); err != nil {
		return err
	}
	indexJSON, err := json.Marshal(d.index)
	if err != nil {
		return err
	}
	return os.WriteFile(d.ref.indexPath(), indexJSON, 0644)
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
