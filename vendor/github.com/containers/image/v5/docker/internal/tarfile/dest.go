package tarfile

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/iolimits"
	"github.com/containers/image/v5/internal/tmpdir"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Destination is a partial implementation of types.ImageDestination for writing to an io.Writer.
type Destination struct {
	archive  *Writer
	repoTags []reference.NamedTagged
	// Other state.
	config []byte
	sysCtx *types.SystemContext
}

// NewDestination returns a tarfile.Destination adding images to the specified Writer.
func NewDestination(sys *types.SystemContext, archive *Writer, ref reference.NamedTagged) *Destination {
	repoTags := []reference.NamedTagged{}
	if ref != nil {
		repoTags = append(repoTags, ref)
	}
	return &Destination{
		archive:  archive,
		repoTags: repoTags,
		sysCtx:   sys,
	}
}

// AddRepoTags adds the specified tags to the destination's repoTags.
func (d *Destination) AddRepoTags(tags []reference.NamedTagged) {
	d.repoTags = append(d.repoTags, tags...)
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
func (d *Destination) SupportsSignatures(ctx context.Context) error {
	return errors.Errorf("Storing signatures for docker tar files is not supported")
}

// AcceptsForeignLayerURLs returns false iff foreign layers in manifest should be actually
// uploaded to the image destination, true otherwise.
func (d *Destination) AcceptsForeignLayerURLs() bool {
	return false
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime architecture and OS. False otherwise.
func (d *Destination) MustMatchRuntimeOS() bool {
	return false
}

// IgnoresEmbeddedDockerReference returns true iff the destination does not care about Image.EmbeddedDockerReferenceConflicts(),
// and would prefer to receive an unmodified manifest instead of one modified for the destination.
// Does not make a difference if Reference().DockerReference() is nil.
func (d *Destination) IgnoresEmbeddedDockerReference() bool {
	return false // N/A, we only accept schema2 images where EmbeddedDockerReferenceConflicts() is always false.
}

// HasThreadSafePutBlob indicates whether PutBlob can be executed concurrently.
func (d *Destination) HasThreadSafePutBlob() bool {
	// The code _is_ actually thread-safe, but apart from computing sizes/digests of layers where
	// this is unknown in advance, the actual copy is serialized by d.archive, so there probably isn’t
	// much benefit from concurrency, mostly just extra CPU, memory and I/O contention.
	return false
}

// PutBlob writes contents of stream and returns data representing the result (with all data filled in).
// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
// inputInfo.Size is the expected length of stream, if known.
// May update cache.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (d *Destination) PutBlob(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, cache types.BlobInfoCache, isConfig bool) (types.BlobInfo, error) {
	// Ouch, we need to stream the blob into a temporary file just to determine the size.
	// When the layer is decompressed, we also have to generate the digest on uncompressed data.
	if inputInfo.Size == -1 || inputInfo.Digest.String() == "" {
		logrus.Debugf("docker tarfile: input with unknown size, streaming to disk first ...")
		streamCopy, err := ioutil.TempFile(tmpdir.TemporaryDirectoryForBigFiles(d.sysCtx), "docker-tarfile-blob")
		if err != nil {
			return types.BlobInfo{}, err
		}
		defer os.Remove(streamCopy.Name())
		defer streamCopy.Close()

		digester := digest.Canonical.Digester()
		tee := io.TeeReader(stream, digester.Hash())
		// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
		size, err := io.Copy(streamCopy, tee)
		if err != nil {
			return types.BlobInfo{}, err
		}
		_, err = streamCopy.Seek(0, io.SeekStart)
		if err != nil {
			return types.BlobInfo{}, err
		}
		inputInfo.Size = size // inputInfo is a struct, so we are only modifying our copy.
		if inputInfo.Digest == "" {
			inputInfo.Digest = digester.Digest()
		}
		stream = streamCopy
		logrus.Debugf("... streaming done")
	}

	if err := d.archive.lock(); err != nil {
		return types.BlobInfo{}, err
	}
	defer d.archive.unlock()

	// Maybe the blob has been already sent
	ok, reusedInfo, err := d.archive.tryReusingBlobLocked(inputInfo)
	if err != nil {
		return types.BlobInfo{}, err
	}
	if ok {
		return reusedInfo, nil
	}

	if isConfig {
		buf, err := iolimits.ReadAtMost(stream, iolimits.MaxConfigBodySize)
		if err != nil {
			return types.BlobInfo{}, errors.Wrap(err, "Error reading Config file stream")
		}
		d.config = buf
		if err := d.archive.sendFileLocked(d.archive.configPath(inputInfo.Digest), inputInfo.Size, bytes.NewReader(buf)); err != nil {
			return types.BlobInfo{}, errors.Wrap(err, "Error writing Config file")
		}
	} else {
		if err := d.archive.sendFileLocked(d.archive.physicalLayerPath(inputInfo.Digest), inputInfo.Size, stream); err != nil {
			return types.BlobInfo{}, err
		}
	}
	d.archive.recordBlobLocked(types.BlobInfo{Digest: inputInfo.Digest, Size: inputInfo.Size})
	return types.BlobInfo{Digest: inputInfo.Digest, Size: inputInfo.Size}, nil
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
func (d *Destination) TryReusingBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	if err := d.archive.lock(); err != nil {
		return false, types.BlobInfo{}, err
	}
	defer d.archive.unlock()

	return d.archive.tryReusingBlobLocked(info)
}

// PutManifest writes manifest to the destination.
// The instanceDigest value is expected to always be nil, because this transport does not support manifest lists, so
// there can be no secondary manifests.
// FIXME? This should also receive a MIME type if known, to differentiate between schema versions.
// If the destination is in principle available, refuses this manifest type (e.g. it does not recognize the schema),
// but may accept a different manifest type, the returned error must be an ManifestTypeRejectedError.
func (d *Destination) PutManifest(ctx context.Context, m []byte, instanceDigest *digest.Digest) error {
	if instanceDigest != nil {
		return errors.New(`Manifest lists are not supported for docker tar files`)
	}
	// We do not bother with types.ManifestTypeRejectedError; our .SupportedManifestMIMETypes() above is already providing only one alternative,
	// so the caller trying a different manifest kind would be pointless.
	var man manifest.Schema2
	if err := json.Unmarshal(m, &man); err != nil {
		return errors.Wrap(err, "Error parsing manifest")
	}
	if man.SchemaVersion != 2 || man.MediaType != manifest.DockerV2Schema2MediaType {
		return errors.Errorf("Unsupported manifest type, need a Docker schema 2 manifest")
	}

	if err := d.archive.lock(); err != nil {
		return err
	}
	defer d.archive.unlock()

	if err := d.archive.writeLegacyMetadataLocked(man.LayersDescriptors, d.config, d.repoTags); err != nil {
		return err
	}

	return d.archive.ensureManifestItemLocked(man.LayersDescriptors, man.ConfigDescriptor.Digest, d.repoTags)
}

// PutSignatures would add the given signatures to the docker tarfile (currently not supported).
// The instanceDigest value is expected to always be nil, because this transport does not support manifest lists, so
// there can be no secondary manifests.  MUST be called after PutManifest (signatures reference manifest contents).
func (d *Destination) PutSignatures(ctx context.Context, signatures [][]byte, instanceDigest *digest.Digest) error {
	if instanceDigest != nil {
		return errors.Errorf(`Manifest lists are not supported for docker tar files`)
	}
	if len(signatures) != 0 {
		return errors.Errorf("Storing signatures for docker tar files is not supported")
	}
	return nil
}
