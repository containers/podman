package layout

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	digest "github.com/opencontainers/go-digest"
	imgspec "github.com/opencontainers/image-spec/specs-go"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ociImageDestination struct {
	ref           ociReference
	index         imgspecv1.Index
	sharedBlobDir string
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
		}
	}

	d := &ociImageDestination{ref: ref, index: *index}
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

func (d *ociImageDestination) SupportedManifestMIMETypes() []string {
	return []string{
		imgspecv1.MediaTypeImageManifest,
	}
}

// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures.
// Note: It is still possible for PutSignatures to fail if SupportsSignatures returns nil.
func (d *ociImageDestination) SupportsSignatures(ctx context.Context) error {
	return errors.Errorf("Pushing signatures for OCI images is not supported")
}

func (d *ociImageDestination) DesiredLayerCompression() types.LayerCompression {
	return types.Compress
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *ociImageDestination) AcceptsForeignLayerURLs() bool {
	return true
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime OS. False otherwise.
func (d *ociImageDestination) MustMatchRuntimeOS() bool {
	return false
}

// PutBlob writes contents of stream and returns data representing the result (with all data filled in).
// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
// inputInfo.Size is the expected length of stream, if known.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *ociImageDestination) PutBlob(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, isConfig bool) (types.BlobInfo, error) {
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

// HasBlob returns true iff the image destination already contains a blob with the matching digest which can be reapplied using ReapplyBlob.
// Unlike PutBlob, the digest can not be empty.  If HasBlob returns true, the size of the blob must also be returned.
// If the destination does not contain the blob, or it is unknown, HasBlob ordinarily returns (false, -1, nil);
// it returns a non-nil error only on an unexpected failure.
func (d *ociImageDestination) HasBlob(ctx context.Context, info types.BlobInfo) (bool, int64, error) {
	if info.Digest == "" {
		return false, -1, errors.Errorf(`"Can not check for a blob with unknown digest`)
	}
	blobPath, err := d.ref.blobPath(info.Digest, d.sharedBlobDir)
	if err != nil {
		return false, -1, err
	}
	finfo, err := os.Stat(blobPath)
	if err != nil && os.IsNotExist(err) {
		return false, -1, nil
	}
	if err != nil {
		return false, -1, err
	}
	return true, finfo.Size(), nil
}

func (d *ociImageDestination) ReapplyBlob(ctx context.Context, info types.BlobInfo) (types.BlobInfo, error) {
	return info, nil
}

// PutManifest writes manifest to the destination.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *ociImageDestination) PutManifest(ctx context.Context, m []byte) error {
	digest, err := manifest.Digest(m)
	if err != nil {
		return err
	}
	desc := imgspecv1.Descriptor{}
	desc.Digest = digest
	// TODO(runcom): beaware and add support for OCI manifest list
	desc.MediaType = imgspecv1.MediaTypeImageManifest
	desc.Size = int64(len(m))

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

	if d.ref.image != "" {
		annotations := make(map[string]string)
		annotations["org.opencontainers.image.ref.name"] = d.ref.image
		desc.Annotations = annotations
	}
	desc.Platform = &imgspecv1.Platform{
		Architecture: runtime.GOARCH,
		OS:           runtime.GOOS,
	}
	d.addManifest(&desc)

	return nil
}

func (d *ociImageDestination) addManifest(desc *imgspecv1.Descriptor) {
	for i, manifest := range d.index.Manifests {
		if manifest.Annotations["org.opencontainers.image.ref.name"] == desc.Annotations["org.opencontainers.image.ref.name"] {
			// TODO Should there first be a cleanup based on the descriptor we are going to replace?
			d.index.Manifests[i] = *desc
			return
		}
	}
	d.index.Manifests = append(d.index.Manifests, *desc)
}

func (d *ociImageDestination) PutSignatures(ctx context.Context, signatures [][]byte) error {
	if len(signatures) != 0 {
		return errors.Errorf("Pushing signatures for OCI images is not supported")
	}
	return nil
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *ociImageDestination) Commit(ctx context.Context) error {
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
