package tarfile

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sync"

	"github.com/containers/image/v5/internal/iolimits"
	"github.com/containers/image/v5/internal/tmpdir"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// Source is a partial implementation of types.ImageSource for reading from tarPath.
type Source struct {
	tarPath              string
	removeTarPathOnClose bool // Remove temp file on close if true
	// The following data is only available after ensureCachedDataIsPresent() succeeds
	tarManifest       *ManifestItem // nil if not available yet.
	configBytes       []byte
	configDigest      digest.Digest
	orderedDiffIDList []digest.Digest
	knownLayers       map[digest.Digest]*layerInfo
	// Other state
	generatedManifest []byte    // Private cache for GetManifest(), nil if not set yet.
	cacheDataLock     sync.Once // Private state for ensureCachedDataIsPresent to make it concurrency-safe
	cacheDataResult   error     // Private state for ensureCachedDataIsPresent
}

type layerInfo struct {
	path string
	size int64
}

// TODO: We could add support for multiple images in a single archive, so
//       that people could use docker-archive:opensuse.tar:opensuse:leap as
//       the source of an image.
// 	To do for both the NewSourceFromFile and NewSourceFromStream functions

// NewSourceFromFile returns a tarfile.Source for the specified path.
// Deprecated: Please use NewSourceFromFileWithContext which will allows you to configure temp directory
// for big files through SystemContext.BigFilesTemporaryDir
func NewSourceFromFile(path string) (*Source, error) {
	return NewSourceFromFileWithContext(nil, path)
}

// NewSourceFromFileWithContext returns a tarfile.Source for the specified path.
func NewSourceFromFileWithContext(sys *types.SystemContext, path string) (*Source, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening file %q", path)
	}
	defer file.Close()

	// If the file is already not compressed we can just return the file itself
	// as a source. Otherwise we pass the stream to NewSourceFromStream.
	stream, isCompressed, err := compression.AutoDecompress(file)
	if err != nil {
		return nil, errors.Wrapf(err, "Error detecting compression for file %q", path)
	}
	defer stream.Close()
	if !isCompressed {
		return &Source{
			tarPath: path,
		}, nil
	}
	return NewSourceFromStreamWithSystemContext(sys, stream)
}

// NewSourceFromStream returns a tarfile.Source for the specified inputStream,
// which can be either compressed or uncompressed. The caller can close the
// inputStream immediately after NewSourceFromFile returns.
// Deprecated: Please use NewSourceFromStreamWithSystemContext which will allows you to configure
// temp directory for big files through SystemContext.BigFilesTemporaryDir
func NewSourceFromStream(inputStream io.Reader) (*Source, error) {
	return NewSourceFromStreamWithSystemContext(nil, inputStream)
}

// NewSourceFromStreamWithSystemContext returns a tarfile.Source for the specified inputStream,
// which can be either compressed or uncompressed. The caller can close the
// inputStream immediately after NewSourceFromFile returns.
func NewSourceFromStreamWithSystemContext(sys *types.SystemContext, inputStream io.Reader) (*Source, error) {
	// FIXME: use SystemContext here.
	// Save inputStream to a temporary file
	tarCopyFile, err := ioutil.TempFile(tmpdir.TemporaryDirectoryForBigFiles(sys), "docker-tar")
	if err != nil {
		return nil, errors.Wrap(err, "error creating temporary file")
	}
	defer tarCopyFile.Close()

	succeeded := false
	defer func() {
		if !succeeded {
			os.Remove(tarCopyFile.Name())
		}
	}()

	// In order to be compatible with docker-load, we need to support
	// auto-decompression (it's also a nice quality-of-life thing to avoid
	// giving users really confusing "invalid tar header" errors).
	uncompressedStream, _, err := compression.AutoDecompress(inputStream)
	if err != nil {
		return nil, errors.Wrap(err, "Error auto-decompressing input")
	}
	defer uncompressedStream.Close()

	// Copy the plain archive to the temporary file.
	//
	// TODO: This can take quite some time, and should ideally be cancellable
	//       using a context.Context.
	if _, err := io.Copy(tarCopyFile, uncompressedStream); err != nil {
		return nil, errors.Wrapf(err, "error copying contents to temporary file %q", tarCopyFile.Name())
	}
	succeeded = true

	return &Source{
		tarPath:              tarCopyFile.Name(),
		removeTarPathOnClose: true,
	}, nil
}

// tarReadCloser is a way to close the backing file of a tar.Reader when the user no longer needs the tar component.
type tarReadCloser struct {
	*tar.Reader
	backingFile *os.File
}

func (t *tarReadCloser) Close() error {
	return t.backingFile.Close()
}

// openTarComponent returns a ReadCloser for the specific file within the archive.
// This is linear scan; we assume that the tar file will have a fairly small amount of files (~layers),
// and that filesystem caching will make the repeated seeking over the (uncompressed) tarPath cheap enough.
// The caller should call .Close() on the returned stream.
func (s *Source) openTarComponent(componentPath string) (io.ReadCloser, error) {
	f, err := os.Open(s.tarPath)
	if err != nil {
		return nil, err
	}
	succeeded := false
	defer func() {
		if !succeeded {
			f.Close()
		}
	}()

	tarReader, header, err := findTarComponent(f, componentPath)
	if err != nil {
		return nil, err
	}
	if header == nil {
		return nil, os.ErrNotExist
	}
	if header.FileInfo().Mode()&os.ModeType == os.ModeSymlink { // FIXME: untested
		// We follow only one symlink; so no loops are possible.
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		// The new path could easily point "outside" the archive, but we only compare it to existing tar headers without extracting the archive,
		// so we don't care.
		tarReader, header, err = findTarComponent(f, path.Join(path.Dir(componentPath), header.Linkname))
		if err != nil {
			return nil, err
		}
		if header == nil {
			return nil, os.ErrNotExist
		}
	}

	if !header.FileInfo().Mode().IsRegular() {
		return nil, errors.Errorf("Error reading tar archive component %s: not a regular file", header.Name)
	}
	succeeded = true
	return &tarReadCloser{Reader: tarReader, backingFile: f}, nil
}

// findTarComponent returns a header and a reader matching path within inputFile,
// or (nil, nil, nil) if not found.
func findTarComponent(inputFile io.Reader, path string) (*tar.Reader, *tar.Header, error) {
	t := tar.NewReader(inputFile)
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if h.Name == path {
			return t, h, nil
		}
	}
	return nil, nil, nil
}

// readTarComponent returns full contents of componentPath.
func (s *Source) readTarComponent(path string, limit int) ([]byte, error) {
	file, err := s.openTarComponent(path)
	if err != nil {
		return nil, errors.Wrapf(err, "Error loading tar component %s", path)
	}
	defer file.Close()
	bytes, err := iolimits.ReadAtMost(file, limit)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// ensureCachedDataIsPresent loads data necessary for any of the public accessors.
// It is safe to call this from multi-threaded code.
func (s *Source) ensureCachedDataIsPresent() error {
	s.cacheDataLock.Do(func() {
		s.cacheDataResult = s.ensureCachedDataIsPresentPrivate()
	})
	return s.cacheDataResult
}

// ensureCachedDataIsPresentPrivate is a private implementation detail of ensureCachedDataIsPresent.
// Call ensureCachedDataIsPresent instead.
func (s *Source) ensureCachedDataIsPresentPrivate() error {
	// Read and parse manifest.json
	tarManifest, err := s.loadTarManifest()
	if err != nil {
		return err
	}

	// Check to make sure length is 1
	if len(tarManifest) != 1 {
		return errors.Errorf("Unexpected tar manifest.json: expected 1 item, got %d", len(tarManifest))
	}

	// Read and parse config.
	configBytes, err := s.readTarComponent(tarManifest[0].Config, iolimits.MaxConfigBodySize)
	if err != nil {
		return err
	}
	var parsedConfig manifest.Schema2Image // There's a lot of info there, but we only really care about layer DiffIDs.
	if err := json.Unmarshal(configBytes, &parsedConfig); err != nil {
		return errors.Wrapf(err, "Error decoding tar config %s", tarManifest[0].Config)
	}
	if parsedConfig.RootFS == nil {
		return errors.Errorf("Invalid image config (rootFS is not set): %s", tarManifest[0].Config)
	}

	knownLayers, err := s.prepareLayerData(&tarManifest[0], &parsedConfig)
	if err != nil {
		return err
	}

	// Success; commit.
	s.tarManifest = &tarManifest[0]
	s.configBytes = configBytes
	s.configDigest = digest.FromBytes(configBytes)
	s.orderedDiffIDList = parsedConfig.RootFS.DiffIDs
	s.knownLayers = knownLayers
	return nil
}

// loadTarManifest loads and decodes the manifest.json.
func (s *Source) loadTarManifest() ([]ManifestItem, error) {
	// FIXME? Do we need to deal with the legacy format?
	bytes, err := s.readTarComponent(manifestFileName, iolimits.MaxTarFileManifestSize)
	if err != nil {
		return nil, err
	}
	var items []ManifestItem
	if err := json.Unmarshal(bytes, &items); err != nil {
		return nil, errors.Wrap(err, "Error decoding tar manifest.json")
	}
	return items, nil
}

// Close removes resources associated with an initialized Source, if any.
func (s *Source) Close() error {
	if s.removeTarPathOnClose {
		return os.Remove(s.tarPath)
	}
	return nil
}

// LoadTarManifest loads and decodes the manifest.json
func (s *Source) LoadTarManifest() ([]ManifestItem, error) {
	return s.loadTarManifest()
}

func (s *Source) prepareLayerData(tarManifest *ManifestItem, parsedConfig *manifest.Schema2Image) (map[digest.Digest]*layerInfo, error) {
	// Collect layer data available in manifest and config.
	if len(tarManifest.Layers) != len(parsedConfig.RootFS.DiffIDs) {
		return nil, errors.Errorf("Inconsistent layer count: %d in manifest, %d in config", len(tarManifest.Layers), len(parsedConfig.RootFS.DiffIDs))
	}
	knownLayers := map[digest.Digest]*layerInfo{}
	unknownLayerSizes := map[string]*layerInfo{} // Points into knownLayers, a "to do list" of items with unknown sizes.
	for i, diffID := range parsedConfig.RootFS.DiffIDs {
		if _, ok := knownLayers[diffID]; ok {
			// Apparently it really can happen that a single image contains the same layer diff more than once.
			// In that case, the diffID validation ensures that both layers truly are the same, and it should not matter
			// which of the tarManifest.Layers paths is used; (docker save) actually makes the duplicates symlinks to the original.
			continue
		}
		layerPath := tarManifest.Layers[i]
		if _, ok := unknownLayerSizes[layerPath]; ok {
			return nil, errors.Errorf("Layer tarfile %s used for two different DiffID values", layerPath)
		}
		li := &layerInfo{ // A new element in each iteration
			path: layerPath,
			size: -1,
		}
		knownLayers[diffID] = li
		unknownLayerSizes[layerPath] = li
	}

	// Scan the tar file to collect layer sizes.
	file, err := os.Open(s.tarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	t := tar.NewReader(file)
	for {
		h, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if li, ok := unknownLayerSizes[h.Name]; ok {
			// Since GetBlob will decompress layers that are compressed we need
			// to do the decompression here as well, otherwise we will
			// incorrectly report the size. Pretty critical, since tools like
			// umoci always compress layer blobs. Obviously we only bother with
			// the slower method of checking if it's compressed.
			uncompressedStream, isCompressed, err := compression.AutoDecompress(t)
			if err != nil {
				return nil, errors.Wrapf(err, "Error auto-decompressing %s to determine its size", h.Name)
			}
			defer uncompressedStream.Close()

			uncompressedSize := h.Size
			if isCompressed {
				uncompressedSize, err = io.Copy(ioutil.Discard, uncompressedStream)
				if err != nil {
					return nil, errors.Wrapf(err, "Error reading %s to find its size", h.Name)
				}
			}
			li.size = uncompressedSize
			delete(unknownLayerSizes, h.Name)
		}
	}
	if len(unknownLayerSizes) != 0 {
		return nil, errors.Errorf("Some layer tarfiles are missing in the tarball") // This could do with a better error reporting, if this ever happened in practice.
	}

	return knownLayers, nil
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list);
// this never happens if the primary manifest is not a manifest list (e.g. if the source never returns manifest lists).
// This source implementation does not support manifest lists, so the passed-in instanceDigest should always be nil,
// as the primary manifest can not be a list, so there can be no secondary instances.
func (s *Source) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	if instanceDigest != nil {
		// How did we even get here? GetManifest(ctx, nil) has returned a manifest.DockerV2Schema2MediaType.
		return nil, "", errors.New(`Manifest lists are not supported by "docker-daemon:"`)
	}
	if s.generatedManifest == nil {
		if err := s.ensureCachedDataIsPresent(); err != nil {
			return nil, "", err
		}
		m := manifest.Schema2{
			SchemaVersion: 2,
			MediaType:     manifest.DockerV2Schema2MediaType,
			ConfigDescriptor: manifest.Schema2Descriptor{
				MediaType: manifest.DockerV2Schema2ConfigMediaType,
				Size:      int64(len(s.configBytes)),
				Digest:    s.configDigest,
			},
			LayersDescriptors: []manifest.Schema2Descriptor{},
		}
		for _, diffID := range s.orderedDiffIDList {
			li, ok := s.knownLayers[diffID]
			if !ok {
				return nil, "", errors.Errorf("Internal inconsistency: Information about layer %s missing", diffID)
			}
			m.LayersDescriptors = append(m.LayersDescriptors, manifest.Schema2Descriptor{
				Digest:    diffID, // diffID is a digest of the uncompressed tarball
				MediaType: manifest.DockerV2Schema2LayerMediaType,
				Size:      li.size,
			})
		}
		manifestBytes, err := json.Marshal(&m)
		if err != nil {
			return nil, "", err
		}
		s.generatedManifest = manifestBytes
	}
	return s.generatedManifest, manifest.DockerV2Schema2MediaType, nil
}

// uncompressedReadCloser is an io.ReadCloser that closes both the uncompressed stream and the underlying input.
type uncompressedReadCloser struct {
	io.Reader
	underlyingCloser   func() error
	uncompressedCloser func() error
}

func (r uncompressedReadCloser) Close() error {
	var res error
	if err := r.uncompressedCloser(); err != nil {
		res = err
	}
	if err := r.underlyingCloser(); err != nil && res == nil {
		res = err
	}
	return res
}

// HasThreadSafeGetBlob indicates whether GetBlob can be executed concurrently.
func (s *Source) HasThreadSafeGetBlob() bool {
	return true
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *Source) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	if err := s.ensureCachedDataIsPresent(); err != nil {
		return nil, 0, err
	}

	if info.Digest == s.configDigest { // FIXME? Implement a more general algorithm matching instead of assuming sha256.
		return ioutil.NopCloser(bytes.NewReader(s.configBytes)), int64(len(s.configBytes)), nil
	}

	if li, ok := s.knownLayers[info.Digest]; ok { // diffID is a digest of the uncompressed tarball,
		underlyingStream, err := s.openTarComponent(li.path)
		if err != nil {
			return nil, 0, err
		}
		closeUnderlyingStream := true
		defer func() {
			if closeUnderlyingStream {
				underlyingStream.Close()
			}
		}()

		// In order to handle the fact that digests != diffIDs (and thus that a
		// caller which is trying to verify the blob will run into problems),
		// we need to decompress blobs. This is a bit ugly, but it's a
		// consequence of making everything addressable by their DiffID rather
		// than by their digest...
		//
		// In particular, because the v2s2 manifest being generated uses
		// DiffIDs, any caller of GetBlob is going to be asking for DiffIDs of
		// layers not their _actual_ digest. The result is that copy/... will
		// be verifing a "digest" which is not the actual layer's digest (but
		// is instead the DiffID).

		uncompressedStream, _, err := compression.AutoDecompress(underlyingStream)
		if err != nil {
			return nil, 0, errors.Wrapf(err, "Error auto-decompressing blob %s", info.Digest)
		}

		newStream := uncompressedReadCloser{
			Reader:             uncompressedStream,
			underlyingCloser:   underlyingStream.Close,
			uncompressedCloser: uncompressedStream.Close,
		}
		closeUnderlyingStream = false

		return newStream, li.size, nil
	}

	return nil, 0, errors.Errorf("Unknown blob %s", info.Digest)
}

// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
// This source implementation does not support manifest lists, so the passed-in instanceDigest should always be nil,
// as there can be no secondary manifests.
func (s *Source) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	if instanceDigest != nil {
		// How did we even get here? GetManifest(ctx, nil) has returned a manifest.DockerV2Schema2MediaType.
		return nil, errors.Errorf(`Manifest lists are not supported by "docker-daemon:"`)
	}
	return [][]byte{}, nil
}

// LayerInfosForCopy returns either nil (meaning the values in the manifest are fine), or updated values for the layer
// blobsums that are listed in the image's manifest.  If values are returned, they should be used when using GetBlob()
// to read the image's layers.
// This source implementation does not support manifest lists, so the passed-in instanceDigest should always be nil,
// as the primary manifest can not be a list, so there can be no secondary manifests.
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (s *Source) LayerInfosForCopy(context.Context, *digest.Digest) ([]types.BlobInfo, error) {
	return nil, nil
}
