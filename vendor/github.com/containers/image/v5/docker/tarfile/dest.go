package tarfile

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

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
	writer   io.Writer
	tar      *tar.Writer
	repoTags []reference.NamedTagged
	// Other state.
	blobs  map[digest.Digest]types.BlobInfo // list of already-sent blobs
	config []byte
	sysCtx *types.SystemContext
}

// NewDestination returns a tarfile.Destination for the specified io.Writer.
// Deprecated: please use NewDestinationWithContext instead
func NewDestination(dest io.Writer, ref reference.NamedTagged) *Destination {
	return NewDestinationWithContext(nil, dest, ref)
}

// NewDestinationWithContext returns a tarfile.Destination for the specified io.Writer.
func NewDestinationWithContext(sys *types.SystemContext, dest io.Writer, ref reference.NamedTagged) *Destination {
	repoTags := []reference.NamedTagged{}
	if ref != nil {
		repoTags = append(repoTags, ref)
	}
	return &Destination{
		writer:   dest,
		tar:      tar.NewWriter(dest),
		repoTags: repoTags,
		blobs:    make(map[digest.Digest]types.BlobInfo),
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
	// When the layer is decompressed, we also have to generate the digest on uncompressed datas.
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

	// Maybe the blob has been already sent
	ok, reusedInfo, err := d.TryReusingBlob(ctx, inputInfo, cache, false)
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
		if err := d.sendFile(inputInfo.Digest.Hex()+".json", inputInfo.Size, bytes.NewReader(buf)); err != nil {
			return types.BlobInfo{}, errors.Wrap(err, "Error writing Config file")
		}
	} else {
		// Note that this can't be e.g. filepath.Join(l.Digest.Hex(), legacyLayerFileName); due to the way
		// writeLegacyLayerMetadata constructs layer IDs differently from inputinfo.Digest values (as described
		// inside it), most of the layers would end up in subdirectories alone without any metadata; (docker load)
		// tries to load every subdirectory as an image and fails if the config is missing.  So, keep the layers
		// in the root of the tarball.
		if err := d.sendFile(inputInfo.Digest.Hex()+".tar", inputInfo.Size, stream); err != nil {
			return types.BlobInfo{}, err
		}
	}
	d.blobs[inputInfo.Digest] = types.BlobInfo{Digest: inputInfo.Digest, Size: inputInfo.Size}
	return types.BlobInfo{Digest: inputInfo.Digest, Size: inputInfo.Size}, nil
}

// TryReusingBlob checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If canSubstitute, TryReusingBlob can use an equivalent equivalent of the desired blob; in that case the returned info may not match the input.
// If the blob has been succesfully reused, returns (true, info, nil); info must contain at least a digest and size.
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
// May use and/or update cache.
func (d *Destination) TryReusingBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	if info.Digest == "" {
		return false, types.BlobInfo{}, errors.Errorf("Can not check for a blob with unknown digest")
	}
	if blob, ok := d.blobs[info.Digest]; ok {
		return true, types.BlobInfo{Digest: info.Digest, Size: blob.Size}, nil
	}
	return false, types.BlobInfo{}, nil
}

func (d *Destination) createRepositoriesFile(rootLayerID string) error {
	repositories := map[string]map[string]string{}
	for _, repoTag := range d.repoTags {
		if val, ok := repositories[repoTag.Name()]; ok {
			val[repoTag.Tag()] = rootLayerID
		} else {
			repositories[repoTag.Name()] = map[string]string{repoTag.Tag(): rootLayerID}
		}
	}

	b, err := json.Marshal(repositories)
	if err != nil {
		return errors.Wrap(err, "Error marshaling repositories")
	}
	if err := d.sendBytes(legacyRepositoriesFileName, b); err != nil {
		return errors.Wrap(err, "Error writing config json file")
	}
	return nil
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

	layerPaths, lastLayerID, err := d.writeLegacyLayerMetadata(man.LayersDescriptors)
	if err != nil {
		return err
	}

	if len(man.LayersDescriptors) > 0 {
		if err := d.createRepositoriesFile(lastLayerID); err != nil {
			return err
		}
	}

	repoTags := []string{}
	for _, tag := range d.repoTags {
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
		refString := fmt.Sprintf("%s:%s", tag.Name(), tag.Tag())
		repoTags = append(repoTags, refString)
	}

	items := []ManifestItem{{
		Config:       man.ConfigDescriptor.Digest.Hex() + ".json",
		RepoTags:     repoTags,
		Layers:       layerPaths,
		Parent:       "",
		LayerSources: nil,
	}}
	itemsBytes, err := json.Marshal(&items)
	if err != nil {
		return err
	}

	// FIXME? Do we also need to support the legacy format?
	return d.sendBytes(manifestFileName, itemsBytes)
}

// writeLegacyLayerMetadata writes legacy VERSION and configuration files for all layers
func (d *Destination) writeLegacyLayerMetadata(layerDescriptors []manifest.Schema2Descriptor) (layerPaths []string, lastLayerID string, err error) {
	var chainID digest.Digest
	lastLayerID = ""
	for i, l := range layerDescriptors {
		// This chainID value matches the computation in docker/docker/layer.CreateChainID …
		if chainID == "" {
			chainID = l.Digest
		} else {
			chainID = digest.Canonical.FromString(chainID.String() + " " + l.Digest.String())
		}
		// … but note that this image ID does not match docker/docker/image/v1.CreateID. At least recent
		// versions allocate new IDs on load, as long as the IDs we use are unique / cannot loop.
		//
		// Overall, the goal of computing a digest dependent on the full history is to avoid reusing an image ID
		// (and possibly creating a loop in the "parent" links) if a layer with the same DiffID appears two or more
		// times in layersDescriptors.  The ChainID values are sufficient for this, the v1.CreateID computation
		// which also mixes in the full image configuration seems unnecessary, at least as long as we are storing
		// only a single image per tarball, i.e. all DiffID prefixes are unique (can’t differ only with
		// configuration).
		layerID := chainID.Hex()

		physicalLayerPath := l.Digest.Hex() + ".tar"
		// The layer itself has been stored into physicalLayerPath in PutManifest.
		// So, use that path for layerPaths used in the non-legacy manifest
		layerPaths = append(layerPaths, physicalLayerPath)
		// ... and create a symlink for the legacy format;
		if err := d.sendSymlink(filepath.Join(layerID, legacyLayerFileName), filepath.Join("..", physicalLayerPath)); err != nil {
			return nil, "", errors.Wrap(err, "Error creating layer symbolic link")
		}

		b := []byte("1.0")
		if err := d.sendBytes(filepath.Join(layerID, legacyVersionFileName), b); err != nil {
			return nil, "", errors.Wrap(err, "Error writing VERSION file")
		}

		// The legacy format requires a config file per layer
		layerConfig := make(map[string]interface{})
		layerConfig["id"] = layerID

		// The root layer doesn't have any parent
		if lastLayerID != "" {
			layerConfig["parent"] = lastLayerID
		}
		// The root layer configuration file is generated by using subpart of the image configuration
		if i == len(layerDescriptors)-1 {
			var config map[string]*json.RawMessage
			err := json.Unmarshal(d.config, &config)
			if err != nil {
				return nil, "", errors.Wrap(err, "Error unmarshaling config")
			}
			for _, attr := range [7]string{"architecture", "config", "container", "container_config", "created", "docker_version", "os"} {
				layerConfig[attr] = config[attr]
			}
		}
		b, err := json.Marshal(layerConfig)
		if err != nil {
			return nil, "", errors.Wrap(err, "Error marshaling layer config")
		}
		if err := d.sendBytes(filepath.Join(layerID, legacyConfigFileName), b); err != nil {
			return nil, "", errors.Wrap(err, "Error writing config json file")
		}

		lastLayerID = layerID
	}
	return layerPaths, lastLayerID, nil
}

type tarFI struct {
	path      string
	size      int64
	isSymlink bool
}

func (t *tarFI) Name() string {
	return t.path
}
func (t *tarFI) Size() int64 {
	return t.size
}
func (t *tarFI) Mode() os.FileMode {
	if t.isSymlink {
		return os.ModeSymlink
	}
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

// sendSymlink sends a symlink into the tar stream.
func (d *Destination) sendSymlink(path string, target string) error {
	hdr, err := tar.FileInfoHeader(&tarFI{path: path, size: 0, isSymlink: true}, target)
	if err != nil {
		return nil
	}
	logrus.Debugf("Sending as tar link %s -> %s", path, target)
	return d.tar.WriteHeader(hdr)
}

// sendBytes sends a path into the tar stream.
func (d *Destination) sendBytes(path string, b []byte) error {
	return d.sendFile(path, int64(len(b)), bytes.NewReader(b))
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
	// TODO: This can take quite some time, and should ideally be cancellable using a context.Context.
	size, err := io.Copy(d.tar, stream)
	if err != nil {
		return err
	}
	if size != expectedSize {
		return errors.Errorf("Size mismatch when copying %s, expected %d, got %d", path, expectedSize, size)
	}
	return nil
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

// Commit finishes writing data to the underlying io.Writer.
// It is the caller's responsibility to close it, if necessary.
func (d *Destination) Commit(ctx context.Context) error {
	return d.tar.Close()
}
