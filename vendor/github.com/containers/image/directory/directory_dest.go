package directory

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const version = "Directory Transport Version: 1.1\n"

// ErrNotContainerImageDir indicates that the directory doesn't match the expected contents of a directory created
// using the 'dir' transport
var ErrNotContainerImageDir = errors.New("not a containers image directory, don't want to overwrite important data")

type dirImageDestination struct {
	ref      dirReference
	compress bool
}

// newImageDestination returns an ImageDestination for writing to a directory.
func newImageDestination(ref dirReference, compress bool) (types.ImageDestination, error) {
	d := &dirImageDestination{ref: ref, compress: compress}

	// If directory exists check if it is empty
	// if not empty, check whether the contents match that of a container image directory and overwrite the contents
	// if the contents don't match throw an error
	dirExists, err := pathExists(d.ref.resolvedPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error checking for path %q", d.ref.resolvedPath)
	}
	if dirExists {
		isEmpty, err := isDirEmpty(d.ref.resolvedPath)
		if err != nil {
			return nil, err
		}

		if !isEmpty {
			versionExists, err := pathExists(d.ref.versionPath())
			if err != nil {
				return nil, errors.Wrapf(err, "error checking if path exists %q", d.ref.versionPath())
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
				return nil, errors.Wrapf(err, "error erasing contents in %q", d.ref.resolvedPath)
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
		return nil, errors.Wrapf(err, "error creating version file %q", d.ref.versionPath())
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
func (d *dirImageDestination) SupportsSignatures() error {
	return nil
}

// ShouldCompressLayers returns true iff it is desirable to compress layer blobs written to this destination.
func (d *dirImageDestination) ShouldCompressLayers() bool {
	return d.compress
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *dirImageDestination) AcceptsForeignLayerURLs() bool {
	return false
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime OS. False otherwise.
func (d *dirImageDestination) MustMatchRuntimeOS() bool {
	return false
}

// PutBlob writes contents of stream and returns data representing the result (with all data filled in).
// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
// inputInfo.Size is the expected length of stream, if known.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *dirImageDestination) PutBlob(stream io.Reader, inputInfo types.BlobInfo) (types.BlobInfo, error) {
	blobFile, err := ioutil.TempFile(d.ref.path, "dir-put-blob")
	if err != nil {
		return types.BlobInfo{}, err
	}
	succeeded := false
	defer func() {
		blobFile.Close()
		if !succeeded {
			os.Remove(blobFile.Name())
		}
	}()

	digester := digest.Canonical.Digester()
	tee := io.TeeReader(stream, digester.Hash())

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
	if err := blobFile.Chmod(0644); err != nil {
		return types.BlobInfo{}, err
	}
	blobPath := d.ref.layerPath(computedDigest)
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
func (d *dirImageDestination) HasBlob(info types.BlobInfo) (bool, int64, error) {
	if info.Digest == "" {
		return false, -1, errors.Errorf(`"Can not check for a blob with unknown digest`)
	}
	blobPath := d.ref.layerPath(info.Digest)
	finfo, err := os.Stat(blobPath)
	if err != nil && os.IsNotExist(err) {
		return false, -1, nil
	}
	if err != nil {
		return false, -1, err
	}
	return true, finfo.Size(), nil
}

func (d *dirImageDestination) ReapplyBlob(info types.BlobInfo) (types.BlobInfo, error) {
	return info, nil
}

// PutManifest writes manifest to the destination.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *dirImageDestination) PutManifest(manifest []byte) error {
	return ioutil.WriteFile(d.ref.manifestPath(), manifest, 0644)
}

func (d *dirImageDestination) PutSignatures(signatures [][]byte) error {
	for i, sig := range signatures {
		if err := ioutil.WriteFile(d.ref.signaturePath(i), sig, 0644); err != nil {
			return err
		}
	}
	return nil
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *dirImageDestination) Commit() error {
	return nil
}

// returns true if path exists
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if err != nil && os.IsNotExist(err) {
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
