//go:build !containers_image_storage_stub
// +build !containers_image_storage_stub

package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/image"
	"github.com/containers/image/v5/internal/imagesource/impl"
	"github.com/containers/image/v5/internal/imagesource/stubs"
	"github.com/containers/image/v5/internal/signature"
	"github.com/containers/image/v5/internal/tmpdir"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	digest "github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

type storageImageSource struct {
	impl.Compat
	impl.PropertyMethodsInitialize
	stubs.NoGetBlobAtInitialize

	imageRef        storageReference
	image           *storage.Image
	systemContext   *types.SystemContext    // SystemContext used in GetBlob() to create temporary files
	layerPosition   map[digest.Digest]int   // Where we are in reading a blob's layers
	cachedManifest  []byte                  // A cached copy of the manifest, if already known, or nil
	getBlobMutex    sync.Mutex              // Mutex to sync state for parallel GetBlob executions
	SignatureSizes  []int                   `json:"signature-sizes,omitempty"`  // List of sizes of each signature slice
	SignaturesSizes map[digest.Digest][]int `json:"signatures-sizes,omitempty"` // List of sizes of each signature slice
}

// newImageSource sets up an image for reading.
func newImageSource(sys *types.SystemContext, imageRef storageReference) (*storageImageSource, error) {
	// First, locate the image.
	img, err := imageRef.resolveImage(sys)
	if err != nil {
		return nil, err
	}

	// Build the reader object.
	image := &storageImageSource{
		PropertyMethodsInitialize: impl.PropertyMethods(impl.Properties{
			HasThreadSafeGetBlob: true,
		}),
		NoGetBlobAtInitialize: stubs.NoGetBlobAt(imageRef),

		imageRef:        imageRef,
		systemContext:   sys,
		image:           img,
		layerPosition:   make(map[digest.Digest]int),
		SignatureSizes:  []int{},
		SignaturesSizes: make(map[digest.Digest][]int),
	}
	image.Compat = impl.AddCompat(image)
	if img.Metadata != "" {
		if err := json.Unmarshal([]byte(img.Metadata), image); err != nil {
			return nil, fmt.Errorf("decoding metadata for source image: %w", err)
		}
	}
	return image, nil
}

// Reference returns the image reference that we used to find this image.
func (s *storageImageSource) Reference() types.ImageReference {
	return s.imageRef
}

// Close cleans up any resources we tied up while reading the image.
func (s *storageImageSource) Close() error {
	return nil
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *storageImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (rc io.ReadCloser, n int64, err error) {
	// We need a valid digest value.
	digest := info.Digest
	err = digest.Validate()
	if err != nil {
		return nil, 0, err
	}

	if digest == image.GzippedEmptyLayerDigest {
		return io.NopCloser(bytes.NewReader(image.GzippedEmptyLayer)), int64(len(image.GzippedEmptyLayer)), nil
	}

	// Check if the blob corresponds to a diff that was used to initialize any layers.  Our
	// callers should try to retrieve layers using their uncompressed digests, so no need to
	// check if they're using one of the compressed digests, which we can't reproduce anyway.
	layers, _ := s.imageRef.transport.store.LayersByUncompressedDigest(digest)

	// If it's not a layer, then it must be a data item.
	if len(layers) == 0 {
		b, err := s.imageRef.transport.store.ImageBigData(s.image.ID, digest.String())
		if err != nil {
			return nil, 0, err
		}
		r := bytes.NewReader(b)
		logrus.Debugf("exporting opaque data as blob %q", digest.String())
		return io.NopCloser(r), int64(r.Len()), nil
	}

	// NOTE: the blob is first written to a temporary file and subsequently
	// closed.  The intention is to keep the time we own the storage lock
	// as short as possible to allow other processes to access the storage.
	rc, n, _, err = s.getBlobAndLayerID(digest, layers)
	if err != nil {
		return nil, 0, err
	}
	defer rc.Close()

	tmpFile, err := os.CreateTemp(tmpdir.TemporaryDirectoryForBigFiles(s.systemContext), "")
	if err != nil {
		return nil, 0, err
	}
	success := false
	defer func() {
		if !success {
			tmpFile.Close()
		}
	}()
	// On Unix and modern Windows (2022 at least) we can eagerly unlink the file to ensure it's automatically
	// cleaned up on process termination (or if the caller forgets to invoke Close())
	if err := os.Remove(tmpFile.Name()); err != nil {
		return nil, 0, err
	}

	if _, err := io.Copy(tmpFile, rc); err != nil {
		return nil, 0, err
	}
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return nil, 0, err
	}

	success = true
	return tmpFile, n, nil
}

// getBlobAndLayer reads the data blob or filesystem layer which matches the digest and size, if given.
func (s *storageImageSource) getBlobAndLayerID(digest digest.Digest, layers []storage.Layer) (rc io.ReadCloser, n int64, layerID string, err error) {
	var layer storage.Layer
	var diffOptions *storage.DiffOptions

	// Step through the list of matching layers.  Tests may want to verify that if we have multiple layers
	// which claim to have the same contents, that we actually do have multiple layers, otherwise we could
	// just go ahead and use the first one every time.
	s.getBlobMutex.Lock()
	i := s.layerPosition[digest]
	s.layerPosition[digest] = i + 1
	s.getBlobMutex.Unlock()
	if len(layers) > 0 {
		layer = layers[i%len(layers)]
	}
	// Force the storage layer to not try to match any compression that was used when the layer was first
	// handed to it.
	noCompression := archive.Uncompressed
	diffOptions = &storage.DiffOptions{
		Compression: &noCompression,
	}
	if layer.UncompressedSize < 0 {
		n = -1
	} else {
		n = layer.UncompressedSize
	}
	logrus.Debugf("exporting filesystem layer %q without compression for blob %q", layer.ID, digest)
	rc, err = s.imageRef.transport.store.Diff("", layer.ID, diffOptions)
	if err != nil {
		return nil, -1, "", err
	}
	return rc, n, layer.ID, err
}

// GetManifest() reads the image's manifest.
func (s *storageImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) (manifestBlob []byte, mimeType string, err error) {
	if instanceDigest != nil {
		key := manifestBigDataKey(*instanceDigest)
		blob, err := s.imageRef.transport.store.ImageBigData(s.image.ID, key)
		if err != nil {
			return nil, "", fmt.Errorf("reading manifest for image instance %q: %w", *instanceDigest, err)
		}
		return blob, manifest.GuessMIMEType(blob), err
	}
	if len(s.cachedManifest) == 0 {
		// The manifest is stored as a big data item.
		// Prefer the manifest corresponding to the user-specified digest, if available.
		if s.imageRef.named != nil {
			if digested, ok := s.imageRef.named.(reference.Digested); ok {
				key := manifestBigDataKey(digested.Digest())
				blob, err := s.imageRef.transport.store.ImageBigData(s.image.ID, key)
				if err != nil && !os.IsNotExist(err) { // os.IsNotExist is true if the image exists but there is no data corresponding to key
					return nil, "", err
				}
				if err == nil {
					s.cachedManifest = blob
				}
			}
		}
		// If the user did not specify a digest, or this is an old image stored before manifestBigDataKey was introduced, use the default manifest.
		// Note that the manifest may not match the expected digest, and that is likely to fail eventually, e.g. in c/image/image/UnparsedImage.Manifest().
		if len(s.cachedManifest) == 0 {
			cachedBlob, err := s.imageRef.transport.store.ImageBigData(s.image.ID, storage.ImageDigestBigDataKey)
			if err != nil {
				return nil, "", err
			}
			s.cachedManifest = cachedBlob
		}
	}
	return s.cachedManifest, manifest.GuessMIMEType(s.cachedManifest), err
}

// LayerInfosForCopy() returns the list of layer blobs that make up the root filesystem of
// the image, after they've been decompressed.
func (s *storageImageSource) LayerInfosForCopy(ctx context.Context, instanceDigest *digest.Digest) ([]types.BlobInfo, error) {
	manifestBlob, manifestType, err := s.GetManifest(ctx, instanceDigest)
	if err != nil {
		return nil, fmt.Errorf("reading image manifest for %q: %w", s.image.ID, err)
	}
	if manifest.MIMETypeIsMultiImage(manifestType) {
		return nil, errors.New("can't copy layers for a manifest list (shouldn't be attempted)")
	}
	man, err := manifest.FromBlob(manifestBlob, manifestType)
	if err != nil {
		return nil, fmt.Errorf("parsing image manifest for %q: %w", s.image.ID, err)
	}

	uncompressedLayerType := ""
	switch manifestType {
	case imgspecv1.MediaTypeImageManifest:
		uncompressedLayerType = imgspecv1.MediaTypeImageLayer
	case manifest.DockerV2Schema1MediaType, manifest.DockerV2Schema1SignedMediaType, manifest.DockerV2Schema2MediaType:
		uncompressedLayerType = manifest.DockerV2SchemaLayerMediaTypeUncompressed
	}

	physicalBlobInfos := []types.BlobInfo{}
	layerID := s.image.TopLayer
	for layerID != "" {
		layer, err := s.imageRef.transport.store.Layer(layerID)
		if err != nil {
			return nil, fmt.Errorf("reading layer %q in image %q: %w", layerID, s.image.ID, err)
		}
		if layer.UncompressedDigest == "" {
			return nil, fmt.Errorf("uncompressed digest for layer %q is unknown", layerID)
		}
		if layer.UncompressedSize < 0 {
			return nil, fmt.Errorf("uncompressed size for layer %q is unknown", layerID)
		}
		blobInfo := types.BlobInfo{
			Digest:    layer.UncompressedDigest,
			Size:      layer.UncompressedSize,
			MediaType: uncompressedLayerType,
		}
		physicalBlobInfos = append([]types.BlobInfo{blobInfo}, physicalBlobInfos...)
		layerID = layer.Parent
	}

	res, err := buildLayerInfosForCopy(man.LayerInfos(), physicalBlobInfos)
	if err != nil {
		return nil, fmt.Errorf("creating LayerInfosForCopy of image %q: %w", s.image.ID, err)
	}
	return res, nil
}

// buildLayerInfosForCopy builds a LayerInfosForCopy return value based on manifestInfos from the original manifest,
// but using layer data which we can actually produce — physicalInfos for non-empty layers,
// and image.GzippedEmptyLayer for empty ones.
// (This is split basically only to allow easily unit-testing the part that has no dependencies on the external environment.)
func buildLayerInfosForCopy(manifestInfos []manifest.LayerInfo, physicalInfos []types.BlobInfo) ([]types.BlobInfo, error) {
	nextPhysical := 0
	res := make([]types.BlobInfo, len(manifestInfos))
	for i, mi := range manifestInfos {
		if mi.EmptyLayer {
			res[i] = types.BlobInfo{
				Digest:    image.GzippedEmptyLayerDigest,
				Size:      int64(len(image.GzippedEmptyLayer)),
				MediaType: mi.MediaType,
			}
		} else {
			if nextPhysical >= len(physicalInfos) {
				return nil, fmt.Errorf("expected more than %d physical layers to exist", len(physicalInfos))
			}
			res[i] = physicalInfos[nextPhysical] // FIXME? Should we preserve more data in manifestInfos? Notably the current approach correctly removes zstd:chunked metadata annotations.
			nextPhysical++
		}
	}
	if nextPhysical != len(physicalInfos) {
		return nil, fmt.Errorf("used only %d out of %d physical layers", nextPhysical, len(physicalInfos))
	}
	return res, nil
}

// GetSignaturesWithFormat returns the image's signatures.  It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve signatures for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
func (s *storageImageSource) GetSignaturesWithFormat(ctx context.Context, instanceDigest *digest.Digest) ([]signature.Signature, error) {
	var offset int
	signatureBlobs := []byte{}
	signatureSizes := s.SignatureSizes
	key := "signatures"
	instance := "default instance"
	if instanceDigest != nil {
		signatureSizes = s.SignaturesSizes[*instanceDigest]
		key = signatureBigDataKey(*instanceDigest)
		instance = instanceDigest.Encoded()
	}
	if len(signatureSizes) > 0 {
		data, err := s.imageRef.transport.store.ImageBigData(s.image.ID, key)
		if err != nil {
			return nil, fmt.Errorf("looking up signatures data for image %q (%s): %w", s.image.ID, instance, err)
		}
		signatureBlobs = data
	}
	res := []signature.Signature{}
	for _, length := range signatureSizes {
		if offset+length > len(signatureBlobs) {
			return nil, fmt.Errorf("looking up signatures data for image %q (%s): expected at least %d bytes, only found %d", s.image.ID, instance, len(signatureBlobs), offset+length)
		}
		sig, err := signature.FromBlob(signatureBlobs[offset : offset+length])
		if err != nil {
			return nil, fmt.Errorf("parsing signature at (%d, %d): %w", offset, length, err)
		}
		res = append(res, sig)
		offset += length
	}
	if offset != len(signatureBlobs) {
		return nil, fmt.Errorf("signatures data (%s) contained %d extra bytes", instance, len(signatureBlobs)-offset)
	}
	return res, nil
}

// getSize() adds up the sizes of the image's data blobs (which includes the configuration blob), the
// signatures, and the uncompressed sizes of all of the image's layers.
func (s *storageImageSource) getSize() (int64, error) {
	var sum int64
	// Size up the data blobs.
	dataNames, err := s.imageRef.transport.store.ListImageBigData(s.image.ID)
	if err != nil {
		return -1, fmt.Errorf("reading image %q: %w", s.image.ID, err)
	}
	for _, dataName := range dataNames {
		bigSize, err := s.imageRef.transport.store.ImageBigDataSize(s.image.ID, dataName)
		if err != nil {
			return -1, fmt.Errorf("reading data blob size %q for %q: %w", dataName, s.image.ID, err)
		}
		sum += bigSize
	}
	// Add the signature sizes.
	for _, sigSize := range s.SignatureSizes {
		sum += int64(sigSize)
	}
	// Walk the layer list.
	layerID := s.image.TopLayer
	for layerID != "" {
		layer, err := s.imageRef.transport.store.Layer(layerID)
		if err != nil {
			return -1, err
		}
		if layer.UncompressedDigest == "" || layer.UncompressedSize < 0 {
			return -1, fmt.Errorf("size for layer %q is unknown, failing getSize()", layerID)
		}
		sum += layer.UncompressedSize
		if layer.Parent == "" {
			break
		}
		layerID = layer.Parent
	}
	return sum, nil
}

// Size() adds up the sizes of the image's data blobs (which includes the configuration blob), the
// signatures, and the uncompressed sizes of all of the image's layers.
func (s *storageImageSource) Size() (int64, error) {
	return s.getSize()
}
