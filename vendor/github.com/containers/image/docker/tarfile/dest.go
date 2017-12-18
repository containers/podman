package tarfile

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/internal/tmpdir"
	"github.com/containers/image/manifest"
	"github.com/containers/image/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Destination is a partial implementation of types.ImageDestination for writing to an io.Writer.
type Destination struct {
	writer  io.Writer
	tar     *tar.Writer
	repoTag string
	// Other state.
	blobs map[digest.Digest]types.BlobInfo // list of already-sent blobs
}

// NewDestination returns a tarfile.Destination for the specified io.Writer.
func NewDestination(dest io.Writer, ref reference.NamedTagged) *Destination {
	// For github.com/docker/docker consumers, this works just as well as
	//   refString := ref.String()
	// because when reading the RepoTags strings, github.com/docker/docker/reference
	// normalizes both of them to the same value.
	//
	// Doing it this way to include the normalized-out `docker.io[/library]` does make
	// a difference for github.com/projectatomic/docker consumers, with the
	// “Add --add-registry and --block-registry options to docker daemon” patch.
	// These consumers treat reference strings which include a hostname and reference
	// strings without a hostname differently.
	//
	// Using the host name here is more explicit about the intent, and it has the same
	// effect as (docker pull) in projectatomic/docker, which tags the result using
	// a hostname-qualified reference.
	// See https://github.com/containers/image/issues/72 for a more detailed
	// analysis and explanation.
	refString := fmt.Sprintf("%s:%s", ref.Name(), ref.Tag())
	return &Destination{
		writer:  dest,
		tar:     tar.NewWriter(dest),
		repoTag: refString,
		blobs:   make(map[digest.Digest]types.BlobInfo),
	}
}

// SupportedManifestMIMETypes tells which manifest mime types the destination supports
// If an empty slice or nil it's returned, then any mime type can be tried to upload
func (d *Destination) SupportedManifestMIMETypes() []string {
	return []string{
		manifest.DockerV2Schema2MediaType, // We rely on the types.Image.UpdatedImage schema conversion capabilities.
	}
}

// SupportsSignatures returns an error (to be displayed to the user) if the destination certainly can't store signatures.
// Note: It is still possible for PutSignatures to fail if SupportsSignatures returns nil.
func (d *Destination) SupportsSignatures() error {
	return errors.Errorf("Storing signatures for docker tar files is not supported")
}

// ShouldCompressLayers returns true iff it is desirable to compress layer blobs written to this destination.
func (d *Destination) ShouldCompressLayers() bool {
	return false
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *Destination) AcceptsForeignLayerURLs() bool {
	return false
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime OS. False otherwise.
func (d *Destination) MustMatchRuntimeOS() bool {
	return false
}

// PutBlob writes contents of stream and returns data representing the result (with all data filled in).
// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
// inputInfo.Size is the expected length of stream, if known.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *Destination) PutBlob(stream io.Reader, inputInfo types.BlobInfo) (types.BlobInfo, error) {
	if inputInfo.Digest.String() == "" {
		return types.BlobInfo{}, errors.Errorf("Can not stream a blob with unknown digest to docker tarfile")
	}

	ok, size, err := d.HasBlob(inputInfo)
	if err != nil {
		return types.BlobInfo{}, err
	}
	if ok {
		return types.BlobInfo{Digest: inputInfo.Digest, Size: size}, nil
	}

	if inputInfo.Size == -1 { // Ouch, we need to stream the blob into a temporary file just to determine the size.
		logrus.Debugf("docker tarfile: input with unknown size, streaming to disk first ...")
		streamCopy, err := ioutil.TempFile(tmpdir.TemporaryDirectoryForBigFiles(), "docker-tarfile-blob")
		if err != nil {
			return types.BlobInfo{}, err
		}
		defer os.Remove(streamCopy.Name())
		defer streamCopy.Close()

		size, err := io.Copy(streamCopy, stream)
		if err != nil {
			return types.BlobInfo{}, err
		}
		_, err = streamCopy.Seek(0, os.SEEK_SET)
		if err != nil {
			return types.BlobInfo{}, err
		}
		inputInfo.Size = size // inputInfo is a struct, so we are only modifying our copy.
		stream = streamCopy
		logrus.Debugf("... streaming done")
	}

	digester := digest.Canonical.Digester()
	tee := io.TeeReader(stream, digester.Hash())
	if err := d.sendFile(inputInfo.Digest.String(), inputInfo.Size, tee); err != nil {
		return types.BlobInfo{}, err
	}
	d.blobs[inputInfo.Digest] = types.BlobInfo{Digest: digester.Digest(), Size: inputInfo.Size}
	return types.BlobInfo{Digest: digester.Digest(), Size: inputInfo.Size}, nil
}

// HasBlob returns true iff the image destination already contains a blob with
// the matching digest which can be reapplied using ReapplyBlob.  Unlike
// PutBlob, the digest can not be empty.  If HasBlob returns true, the size of
// the blob must also be returned.  If the destination does not contain the
// blob, or it is unknown, HasBlob ordinarily returns (false, -1, nil); it
// returns a non-nil error only on an unexpected failure.
func (d *Destination) HasBlob(info types.BlobInfo) (bool, int64, error) {
	if info.Digest == "" {
		return false, -1, errors.Errorf("Can not check for a blob with unknown digest")
	}
	if blob, ok := d.blobs[info.Digest]; ok {
		return true, blob.Size, nil
	}
	return false, -1, nil
}

// ReapplyBlob informs the image destination that a blob for which HasBlob
// previously returned true would have been passed to PutBlob if it had
// returned false.  Like HasBlob and unlike PutBlob, the digest can not be
// empty.  If the blob is a filesystem layer, this signifies that the changes
// it describes need to be applied again when composing a filesystem tree.
func (d *Destination) ReapplyBlob(info types.BlobInfo) (types.BlobInfo, error) {
	return info, nil
}

// PutManifest writes manifest to the destination.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *Destination) PutManifest(m []byte) error {
	// We do not bother with types.ManifestTypeRejectedError; our .SupportedManifestMIMETypes() above is already providing only one alternative,
	// so the caller trying a different manifest kind would be pointless.
	var man manifest.Schema2
	if err := json.Unmarshal(m, &man); err != nil {
		return errors.Wrap(err, "Error parsing manifest")
	}
	if man.SchemaVersion != 2 || man.MediaType != manifest.DockerV2Schema2MediaType {
		return errors.Errorf("Unsupported manifest type, need a Docker schema 2 manifest")
	}

	layerPaths := []string{}
	for _, l := range man.LayersDescriptors {
		layerPaths = append(layerPaths, l.Digest.String())
	}

	items := []ManifestItem{{
		Config:       man.ConfigDescriptor.Digest.String(),
		RepoTags:     []string{d.repoTag},
		Layers:       layerPaths,
		Parent:       "",
		LayerSources: nil,
	}}
	itemsBytes, err := json.Marshal(&items)
	if err != nil {
		return err
	}

	// FIXME? Do we also need to support the legacy format?
	return d.sendFile(manifestFileName, int64(len(itemsBytes)), bytes.NewReader(itemsBytes))
}

type tarFI struct {
	path string
	size int64
}

func (t *tarFI) Name() string {
	return t.path
}
func (t *tarFI) Size() int64 {
	return t.size
}
func (t *tarFI) Mode() os.FileMode {
	return 0444
}
func (t *tarFI) ModTime() time.Time {
	return time.Unix(0, 0)
}
func (t *tarFI) IsDir() bool {
	return false
}
func (t *tarFI) Sys() interface{} {
	return nil
}

// sendFile sends a file into the tar stream.
func (d *Destination) sendFile(path string, expectedSize int64, stream io.Reader) error {
	hdr, err := tar.FileInfoHeader(&tarFI{path: path, size: expectedSize}, "")
	if err != nil {
		return nil
	}
	logrus.Debugf("Sending as tar file %s", path)
	if err := d.tar.WriteHeader(hdr); err != nil {
		return err
	}
	size, err := io.Copy(d.tar, stream)
	if err != nil {
		return err
	}
	if size != expectedSize {
		return errors.Errorf("Size mismatch when copying %s, expected %d, got %d", path, expectedSize, size)
	}
	return nil
}

// PutSignatures adds the given signatures to the docker tarfile (currently not
// supported). MUST be called after PutManifest (signatures reference manifest
// contents)
func (d *Destination) PutSignatures(signatures [][]byte) error {
	if len(signatures) != 0 {
		return errors.Errorf("Storing signatures for docker tar files is not supported")
	}
	return nil
}

// Commit finishes writing data to the underlying io.Writer.
// It is the caller's responsibility to close it, if necessary.
func (d *Destination) Commit() error {
	return d.tar.Close()
}
