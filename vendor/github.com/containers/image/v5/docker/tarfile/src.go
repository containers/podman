package tarfile

import (
	"context"
	"io"

	internal "github.com/containers/image/v5/docker/internal/tarfile"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
)

// Source is a partial implementation of types.ImageSource for reading from tarPath.
// Most users should use this via implementations of ImageReference from docker/archive or docker/daemon.
type Source struct {
	internal *internal.Source
}

// NewSourceFromFile returns a tarfile.Source for the specified path.
// Deprecated: Please use NewSourceFromFileWithContext which will allows you to configure temp directory
// for big files through SystemContext.BigFilesTemporaryDir
func NewSourceFromFile(path string) (*Source, error) {
	return NewSourceFromFileWithContext(nil, path)
}

// NewSourceFromFileWithContext returns a tarfile.Source for the specified path.
func NewSourceFromFileWithContext(sys *types.SystemContext, path string) (*Source, error) {
	archive, err := internal.NewReaderFromFile(sys, path)
	if err != nil {
		return nil, err
	}
	src := internal.NewSource(archive, true, nil, -1)
	return &Source{internal: src}, nil
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
	archive, err := internal.NewReaderFromStream(sys, inputStream)
	if err != nil {
		return nil, err
	}
	src := internal.NewSource(archive, true, nil, -1)
	return &Source{internal: src}, nil
}

// Close removes resources associated with an initialized Source, if any.
func (s *Source) Close() error {
	return s.internal.Close()
}

// LoadTarManifest loads and decodes the manifest.json
func (s *Source) LoadTarManifest() ([]ManifestItem, error) {
	return s.internal.TarManifest(), nil
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list);
// this never happens if the primary manifest is not a manifest list (e.g. if the source never returns manifest lists).
// This source implementation does not support manifest lists, so the passed-in instanceDigest should always be nil,
// as the primary manifest can not be a list, so there can be no secondary instances.
func (s *Source) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	return s.internal.GetManifest(ctx, instanceDigest)
}

// HasThreadSafeGetBlob indicates whether GetBlob can be executed concurrently.
func (s *Source) HasThreadSafeGetBlob() bool {
	return s.internal.HasThreadSafeGetBlob()
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *Source) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	return s.internal.GetBlob(ctx, info, cache)
}

// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
// This source implementation does not support manifest lists, so the passed-in instanceDigest should always be nil,
// as there can be no secondary manifests.
func (s *Source) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	return s.internal.GetSignatures(ctx, instanceDigest)
}

// LayerInfosForCopy returns either nil (meaning the values in the manifest are fine), or updated values for the layer
// blobsums that are listed in the image's manifest.  If values are returned, they should be used when using GetBlob()
// to read the image's layers.
// This source implementation does not support manifest lists, so the passed-in instanceDigest should always be nil,
// as the primary manifest can not be a list, so there can be no secondary manifests.
// The Digest field is guaranteed to be provided; Size may be -1.
// WARNING: The list may contain duplicates, and they are semantically relevant.
func (s *Source) LayerInfosForCopy(ctx context.Context, instanceDigest *digest.Digest) ([]types.BlobInfo, error) {
	return s.internal.LayerInfosForCopy(ctx, instanceDigest)
}
