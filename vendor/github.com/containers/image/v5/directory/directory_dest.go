package directory

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/containers/image/v5/internal/putblobdigest"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const version = "Directory Transport Version: 1.1\n"

// ErrNotContainerImageDir indicates that the directory doesn't match the expected contents of a directory created
// using the 'dir' transport
var ErrNotContainerImageDir = errors.New("not a containers image directory, don't want to overwrite important data")

type dirImageDestination struct {
	ref                     dirReference
	desiredLayerCompression types.LayerCompression
}

// newImageDestination returns an ImageDestination for writing to a directory.
func newImageDestination(sys *types.SystemContext, ref dirReference) (types.ImageDestination, error) {
	desiredLayerCompression := types.PreserveOriginal
	if sys != nil {
		if sys.DirForceCompress {
			desiredLayerCompression = types.Compress

			if sys.DirForceDecompress {
				return nil, errors.Errorf("Cannot compress and decompress at the same time")
			}
		}
		if sys.DirForceDecompress {
			desiredLayerCompression = types.Decompress
		}
	}
	d := &dirImageDestination{ref: ref, desiredLayerCompression: desiredLayerCompression}

	// If directory exists check if it is empty
	// if not empty, check whether the contents match that of a container image directory and overwrite the contents
	// if the contents don't match throw an error
	dirExists, err := pathExists(d.ref.resolvedPath)
	if err != nil {
		return nil, errors.Wrapf(err, "checking for path %q", d.ref.resolvedPath)
	}
	if dirExists {
		isEmpty, err := isDirEmpty(d.ref.resolvedPath)
		if err != nil {
			return nil, err
		}

		if !isEmpty {
			versionExists, err := pathExists(d.ref.versionPath())
			if err != nil {
				return nil, errors.Wrapf(err, "checking if path exists %q", d.ref.versionPath())
			}
			if versionExists {
				contents, err := ioutil.ReadFile(d.ref.versionPath())
				if err != nil {
					return nil, err
				}
				// check if contents of version file is what we expect it to be
				if string(contents) != version {
					return nil, ErrNotContainerImageDir
				}
			} else {
				return nil, ErrNotContainerImageDir
			}
			// delete directory contents so that only one image is in the directory at a time
			if err = removeDirContents(d.ref.resolvedPath); err != nil {
				return nil, errors.Wrapf(err, "erasing contents in %q", d.ref.resolvedPath)
			}
			logrus.Debugf("overwriting existing container image directory %q", d.ref.resolvedPath)
		}
	} else {
		// create directory if it doesn't exist
		if err := os.MkdirAll(d.ref.resolvedPath, 0755); err != nil {
			return nil, errors.Wrapf(err, "unable to create directory %q", d.ref.resolvedPath)
		}
	}
	// create version file
	err = ioutil.WriteFile(d.ref.versionPath(), []byte(version), 0644)
	if err != nil {
		return nil, errors.Wrapf(err, "creating version file %q", d.ref.versionPath())
	}
	return d, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (d *dirImageDestination) Reference() types.ImageReference {
	return d.ref
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *dirImageDestination) Close() error {
	return nil
}

func (d *dirImageDestination) SupportedManifestMIMETypes() []string {
	return nil
}

// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures.
// Note: It is still possible for PutSignatures to fail if SupportsSignatures returns nil.
func (d *dirImageDestination) SupportsSignatures(ctx context.Context) error {
	return nil
}

func (d *dirImageDestination) DesiredLayerCompression() types.LayerCompression {
	return d.desiredLayerCompression
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *dirImageDestination) AcceptsForeignLayerURLs() bool {
	return false
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime architecture and OS. False otherwise.
func (d *dirImageDestination) MustMatchRuntimeOS() bool {
	return false
}

// IgnoresEmbeddedDockerReference returns true iff the destination does not care about Image.EmbeddedDockerReferenceConflicts(),
// and would prefer to receive an unmodified manifest instead of one modified for the destination.
// Does not make a difference if Reference().DockerReference() is nil.
func (d *dirImageDestination) IgnoresEmbeddedDockerReference() bool {
	return false // N/A, DockerReference() returns nil.
}

// HasThreadSafePutBlob indicates whether PutBlob can be executed concurrently.
func (d *dirImageDestination) HasThreadSafePutBlob() bool {
	return false
}

// PutBlob writes contents of stream and returns data representing the result (with all data filled in).
// inputInfo.Digest can be optionally provided if known; if provided, and stream is read to the end without error, the digest MUST match the stream contents.
// inputInfo.Size is the expected length of stream, if known.
// May update cache.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *dirImageDestination) PutBlob(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, cache types.BlobInfoCache, isConfig bool) (types.BlobInfo, error) {
	blobFile, err := ioutil.TempFile(d.ref.path, "dir-put-blob")
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

	digester, stream := putblobdigest.DigestIfCanonicalUnknown(stream, inputInfo)
	// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
	size, err := io.Copy(blobFile, stream)
	if err != nil {
		return types.BlobInfo{}, err
	}
	blobDigest := digester.Digest()
	if inputInfo.Size != -1 && size != inputInfo.Size {
		return types.BlobInfo{}, errors.Errorf("Size mismatch when copying %s, expected %d, got %d", blobDigest, inputInfo.Size, size)
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

	blobPath := d.ref.layerPath(blobDigest)
	// need to explicitly close the file, since a rename won't otherwise not work on Windows
	blobFile.Close()
	explicitClosed = true
	if err := os.Rename(blobFile.Name(), blobPath); err != nil {
		return types.BlobInfo{}, err
	}
	succeeded = true
	return types.BlobInfo{Digest: blobDigest, Size: size}, nil
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
func (d *dirImageDestination) TryReusingBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	if info.Digest == "" {
		return false, types.BlobInfo{}, errors.Errorf(`"Can not check for a blob with unknown digest`)
	}
	blobPath := d.ref.layerPath(info.Digest)
	finfo, err := os.Stat(blobPath)
	if err != nil && os.IsNotExist(err) {
		return false, types.BlobInfo{}, nil
	}
	if err != nil {
		return false, types.BlobInfo{}, err
	}
	return true, types.BlobInfo{Digest: info.Digest, Size: finfo.Size()}, nil
}

// PutManifest writes manifest to the destination.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to write the manifest for (when
// the primary manifest is a manifest list); this should always be nil if the primary manifest is not a manifest list.
// It is expected but not enforced that the instanceDigest, when specified, matches the digest of `manifest` as generated
// by `manifest.Digest()`.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *dirImageDestination) PutManifest(ctx context.Context, manifest []byte, instanceDigest *digest.Digest) error {
	return ioutil.WriteFile(d.ref.manifestPath(instanceDigest), manifest, 0644)
}

// PutSignatures writes a set of signatures to the destination.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to write or overwrite the signatures for
// (when the primary manifest is a manifest list); this should always be nil if the primary manifest is not a manifest list.
func (d *dirImageDestination) PutSignatures(ctx context.Context, signatures [][]byte, instanceDigest *digest.Digest) error {
	for i, sig := range signatures {
		if err := ioutil.WriteFile(d.ref.signaturePath(i, instanceDigest), sig, 0644); err != nil {
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
func (d *dirImageDestination) Commit(context.Context, types.UnparsedImage) error {
	return nil
}

// returns true if path exists
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// returns true if directory is empty
func isDirEmpty(path string) (bool, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(files) == 0, nil
}

// deletes the contents of a directory
func removeDirContents(path string) error {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := os.RemoveAll(filepath.Join(path, file.Name())); err != nil {
			return err
		}
	}
	return nil
}
