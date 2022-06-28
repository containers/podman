// +build !containers_image_storage_stub

package storage

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/internal/tmpdir"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/blobinfocache/none"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/ioutils"
	digest "github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// ErrBlobDigestMismatch is returned when PutBlob() is given a blob
	// with a digest-based name that doesn't match its contents.
	ErrBlobDigestMismatch = stderrors.New("blob digest mismatch")
	// ErrBlobSizeMismatch is returned when PutBlob() is given a blob
	// with an expected size that doesn't match the reader.
	ErrBlobSizeMismatch = stderrors.New("blob size mismatch")
	// ErrNoSuchImage is returned when we attempt to access an image which
	// doesn't exist in the storage area.
	ErrNoSuchImage = storage.ErrNotAnImage
)

type storageImageSource struct {
	imageRef        storageReference
	image           *storage.Image
	layerPosition   map[digest.Digest]int   // Where we are in reading a blob's layers
	cachedManifest  []byte                  // A cached copy of the manifest, if already known, or nil
	getBlobMutex    sync.Mutex              // Mutex to sync state for parallel GetBlob executions
	SignatureSizes  []int                   `json:"signature-sizes,omitempty"`  // List of sizes of each signature slice
	SignaturesSizes map[digest.Digest][]int `json:"signatures-sizes,omitempty"` // List of sizes of each signature slice
}

type storageImageDestination struct {
	imageRef        storageReference
	directory       string                          // Temporary directory where we store blobs until Commit() time
	nextTempFileID  int32                           // A counter that we use for computing filenames to assign to blobs
	manifest        []byte                          // Manifest contents, temporary
	signatures      []byte                          // Signature contents, temporary
	signatureses    map[digest.Digest][]byte        // Instance signature contents, temporary
	putBlobMutex    sync.Mutex                      // Mutex to sync state for parallel PutBlob executions
	blobDiffIDs     map[digest.Digest]digest.Digest // Mapping from layer blobsums to their corresponding DiffIDs
	fileSizes       map[digest.Digest]int64         // Mapping from layer blobsums to their sizes
	filenames       map[digest.Digest]string        // Mapping from layer blobsums to names of files we used to hold them
	SignatureSizes  []int                           `json:"signature-sizes,omitempty"`  // List of sizes of each signature slice
	SignaturesSizes map[digest.Digest][]int         `json:"signatures-sizes,omitempty"` // Sizes of each manifest's signature slice
}

type storageImageCloser struct {
	types.ImageCloser
	size int64
}

// manifestBigDataKey returns a key suitable for recording a manifest with the specified digest using storage.Store.ImageBigData and related functions.
// If a specific manifest digest is explicitly requested by the user, the key returned by this function should be used preferably;
// for compatibility, if a manifest is not available under this key, check also storage.ImageDigestBigDataKey
func manifestBigDataKey(digest digest.Digest) string {
	return storage.ImageDigestManifestBigDataNamePrefix + "-" + digest.String()
}

// signatureBigDataKey returns a key suitable for recording the signatures associated with the manifest with the specified digest using storage.Store.ImageBigData and related functions.
// If a specific manifest digest is explicitly requested by the user, the key returned by this function should be used preferably;
func signatureBigDataKey(digest digest.Digest) string {
	return "signature-" + digest.Encoded()
}

// newImageSource sets up an image for reading.
func newImageSource(ctx context.Context, sys *types.SystemContext, imageRef storageReference) (*storageImageSource, error) {
	// First, locate the image.
	img, err := imageRef.resolveImage(sys)
	if err != nil {
		return nil, err
	}

	// Build the reader object.
	image := &storageImageSource{
		imageRef:        imageRef,
		image:           img,
		layerPosition:   make(map[digest.Digest]int),
		SignatureSizes:  []int{},
		SignaturesSizes: make(map[digest.Digest][]int),
	}
	if img.Metadata != "" {
		if err := json.Unmarshal([]byte(img.Metadata), image); err != nil {
			return nil, errors.Wrap(err, "error decoding metadata for source image")
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

// HasThreadSafeGetBlob indicates whether GetBlob can be executed concurrently.
func (s *storageImageSource) HasThreadSafeGetBlob() bool {
	return true
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *storageImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (rc io.ReadCloser, n int64, err error) {
	if info.Digest == image.GzippedEmptyLayerDigest {
		return ioutil.NopCloser(bytes.NewReader(image.GzippedEmptyLayer)), int64(len(image.GzippedEmptyLayer)), nil
	}
	rc, n, _, err = s.getBlobAndLayerID(info)
	return rc, n, err
}

// getBlobAndLayer reads the data blob or filesystem layer which matches the digest and size, if given.
func (s *storageImageSource) getBlobAndLayerID(info types.BlobInfo) (rc io.ReadCloser, n int64, layerID string, err error) {
	var layer storage.Layer
	var diffOptions *storage.DiffOptions
	// We need a valid digest value.
	err = info.Digest.Validate()
	if err != nil {
		return nil, -1, "", err
	}
	// Check if the blob corresponds to a diff that was used to initialize any layers.  Our
	// callers should try to retrieve layers using their uncompressed digests, so no need to
	// check if they're using one of the compressed digests, which we can't reproduce anyway.
	layers, _ := s.imageRef.transport.store.LayersByUncompressedDigest(info.Digest)

	// If it's not a layer, then it must be a data item.
	if len(layers) == 0 {
		b, err := s.imageRef.transport.store.ImageBigData(s.image.ID, info.Digest.String())
		if err != nil {
			return nil, -1, "", err
		}
		r := bytes.NewReader(b)
		logrus.Debugf("exporting opaque data as blob %q", info.Digest.String())
		return ioutil.NopCloser(r), int64(r.Len()), "", nil
	}
	// Step through the list of matching layers.  Tests may want to verify that if we have multiple layers
	// which claim to have the same contents, that we actually do have multiple layers, otherwise we could
	// just go ahead and use the first one every time.
	s.getBlobMutex.Lock()
	i := s.layerPosition[info.Digest]
	s.layerPosition[info.Digest] = i + 1
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
	logrus.Debugf("exporting filesystem layer %q without compression for blob %q", layer.ID, info.Digest)
	rc, err = s.imageRef.transport.store.Diff("", layer.ID, diffOptions)
	if err != nil {
		return nil, -1, "", err
	}
	return rc, n, layer.ID, err
}

// GetManifest() reads the image's manifest.
func (s *storageImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) (manifestBlob []byte, MIMEType string, err error) {
	if instanceDigest != nil {
		key := manifestBigDataKey(*instanceDigest)
		blob, err := s.imageRef.transport.store.ImageBigData(s.image.ID, key)
		if err != nil {
			return nil, "", errors.Wrapf(err, "error reading manifest for image instance %q", *instanceDigest)
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
		return nil, errors.Wrapf(err, "error reading image manifest for %q", s.image.ID)
	}
	if manifest.MIMETypeIsMultiImage(manifestType) {
		return nil, errors.Errorf("can't copy layers for a manifest list (shouldn't be attempted)")
	}
	man, err := manifest.FromBlob(manifestBlob, manifestType)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing image manifest for %q", s.image.ID)
	}

	uncompressedLayerType := ""
	switch manifestType {
	case imgspecv1.MediaTypeImageManifest:
		uncompressedLayerType = imgspecv1.MediaTypeImageLayer
	case manifest.DockerV2Schema1MediaType, manifest.DockerV2Schema1SignedMediaType, manifest.DockerV2Schema2MediaType:
		// This is actually a compressed type, but there's no uncompressed type defined
		uncompressedLayerType = manifest.DockerV2Schema2LayerMediaType
	}

	physicalBlobInfos := []types.BlobInfo{}
	layerID := s.image.TopLayer
	for layerID != "" {
		layer, err := s.imageRef.transport.store.Layer(layerID)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading layer %q in image %q", layerID, s.image.ID)
		}
		if layer.UncompressedDigest == "" {
			return nil, errors.Errorf("uncompressed digest for layer %q is unknown", layerID)
		}
		if layer.UncompressedSize < 0 {
			return nil, errors.Errorf("uncompressed size for layer %q is unknown", layerID)
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
		return nil, errors.Wrapf(err, "error creating LayerInfosForCopy of image %q", s.image.ID)
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
			res[i] = physicalInfos[nextPhysical]
			nextPhysical++
		}
	}
	if nextPhysical != len(physicalInfos) {
		return nil, fmt.Errorf("used only %d out of %d physical layers", nextPhysical, len(physicalInfos))
	}
	return res, nil
}

// GetSignatures() parses the image's signatures blob into a slice of byte slices.
func (s *storageImageSource) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) (signatures [][]byte, err error) {
	var offset int
	sigslice := [][]byte{}
	signature := []byte{}
	signatureSizes := s.SignatureSizes
	key := "signatures"
	instance := "default instance"
	if instanceDigest != nil {
		signatureSizes = s.SignaturesSizes[*instanceDigest]
		key = signatureBigDataKey(*instanceDigest)
		instance = instanceDigest.Encoded()
	}
	if len(signatureSizes) > 0 {
		signatureBlob, err := s.imageRef.transport.store.ImageBigData(s.image.ID, key)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking up signatures data for image %q (%s)", s.image.ID, instance)
		}
		signature = signatureBlob
	}
	for _, length := range signatureSizes {
		if offset+length > len(signature) {
			return nil, errors.Wrapf(err, "error looking up signatures data for image %q (%s): expected at least %d bytes, only found %d", s.image.ID, instance, len(signature), offset+length)
		}
		sigslice = append(sigslice, signature[offset:offset+length])
		offset += length
	}
	if offset != len(signature) {
		return nil, errors.Errorf("signatures data (%s) contained %d extra bytes", instance, len(signatures)-offset)
	}
	return sigslice, nil
}

// newImageDestination sets us up to write a new image, caching blobs in a temporary directory until
// it's time to Commit() the image
func newImageDestination(sys *types.SystemContext, imageRef storageReference) (*storageImageDestination, error) {
	directory, err := ioutil.TempDir(tmpdir.TemporaryDirectoryForBigFiles(sys), "storage")
	if err != nil {
		return nil, errors.Wrapf(err, "error creating a temporary directory")
	}
	image := &storageImageDestination{
		imageRef:        imageRef,
		directory:       directory,
		signatureses:    make(map[digest.Digest][]byte),
		blobDiffIDs:     make(map[digest.Digest]digest.Digest),
		fileSizes:       make(map[digest.Digest]int64),
		filenames:       make(map[digest.Digest]string),
		SignatureSizes:  []int{},
		SignaturesSizes: make(map[digest.Digest][]int),
	}
	return image, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (s *storageImageDestination) Reference() types.ImageReference {
	return s.imageRef
}

// Close cleans up the temporary directory.
func (s *storageImageDestination) Close() error {
	return os.RemoveAll(s.directory)
}

func (s *storageImageDestination) DesiredLayerCompression() types.LayerCompression {
	// We ultimately have to decompress layers to populate trees on disk
	// and need to explicitly ask for it here, so that the layers' MIME
	// types can be set accordingly.
	return types.PreserveOriginal
}

func (s *storageImageDestination) computeNextBlobCacheFile() string {
	return filepath.Join(s.directory, fmt.Sprintf("%d", atomic.AddInt32(&s.nextTempFileID, 1)))
}

// HasThreadSafePutBlob indicates whether PutBlob can be executed concurrently.
func (s *storageImageDestination) HasThreadSafePutBlob() bool {
	return true
}

// PutBlob writes contents of stream and returns data representing the result.
// inputInfo.Digest can be optionally provided if known; it is not mandatory for the implementation to verify it.
// inputInfo.Size is the expected length of stream, if known.
// inputInfo.MediaType describes the blob format, if known.
// May update cache.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (s *storageImageDestination) PutBlob(ctx context.Context, stream io.Reader, blobinfo types.BlobInfo, cache types.BlobInfoCache, isConfig bool) (types.BlobInfo, error) {
	// Stores a layer or data blob in our temporary directory, checking that any information
	// in the blobinfo matches the incoming data.
	errorBlobInfo := types.BlobInfo{
		Digest: "",
		Size:   -1,
	}
	// Set up to digest the blob and count its size while saving it to a file.
	hasher := digest.Canonical.Digester()
	if blobinfo.Digest.Validate() == nil {
		if a := blobinfo.Digest.Algorithm(); a.Available() {
			hasher = a.Digester()
		}
	}
	diffID := digest.Canonical.Digester()
	filename := s.computeNextBlobCacheFile()
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return errorBlobInfo, errors.Wrapf(err, "error creating temporary file %q", filename)
	}
	defer file.Close()
	counter := ioutils.NewWriteCounter(hasher.Hash())
	reader := io.TeeReader(io.TeeReader(stream, counter), file)
	decompressed, err := archive.DecompressStream(reader)
	if err != nil {
		return errorBlobInfo, errors.Wrap(err, "error setting up to decompress blob")
	}
	// Copy the data to the file.
	// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
	_, err = io.Copy(diffID.Hash(), decompressed)
	decompressed.Close()
	if err != nil {
		return errorBlobInfo, errors.Wrapf(err, "error storing blob to file %q", filename)
	}
	// Ensure that any information that we were given about the blob is correct.
	if blobinfo.Digest.Validate() == nil && blobinfo.Digest != hasher.Digest() {
		return errorBlobInfo, errors.WithStack(ErrBlobDigestMismatch)
	}
	if blobinfo.Size >= 0 && blobinfo.Size != counter.Count {
		return errorBlobInfo, errors.WithStack(ErrBlobSizeMismatch)
	}
	// Record information about the blob.
	s.putBlobMutex.Lock()
	s.blobDiffIDs[hasher.Digest()] = diffID.Digest()
	s.fileSizes[hasher.Digest()] = counter.Count
	s.filenames[hasher.Digest()] = filename
	s.putBlobMutex.Unlock()
	blobDigest := blobinfo.Digest
	if blobDigest.Validate() != nil {
		blobDigest = hasher.Digest()
	}
	blobSize := blobinfo.Size
	if blobSize < 0 {
		blobSize = counter.Count
	}
	// This is safe because we have just computed both values ourselves.
	cache.RecordDigestUncompressedPair(blobDigest, diffID.Digest())
	return types.BlobInfo{
		Digest:    blobDigest,
		Size:      blobSize,
		MediaType: blobinfo.MediaType,
	}, nil
}

// TryReusingBlob checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If canSubstitute, TryReusingBlob can use an equivalent equivalent of the desired blob; in that case the returned info may not match the input.
// If the blob has been succesfully reused, returns (true, info, nil); info must contain at least a digest and size.
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
// May use and/or update cache.
func (s *storageImageDestination) TryReusingBlob(ctx context.Context, blobinfo types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	// lock the entire method as it executes fairly quickly
	s.putBlobMutex.Lock()
	defer s.putBlobMutex.Unlock()
	if blobinfo.Digest == "" {
		return false, types.BlobInfo{}, errors.Errorf(`Can not check for a blob with unknown digest`)
	}
	if err := blobinfo.Digest.Validate(); err != nil {
		return false, types.BlobInfo{}, errors.Wrapf(err, `Can not check for a blob with invalid digest`)
	}

	// Check if we've already cached it in a file.
	if size, ok := s.fileSizes[blobinfo.Digest]; ok {
		return true, types.BlobInfo{
			Digest:    blobinfo.Digest,
			Size:      size,
			MediaType: blobinfo.MediaType,
		}, nil
	}

	// Check if we have a wasn't-compressed layer in storage that's based on that blob.
	layers, err := s.imageRef.transport.store.LayersByUncompressedDigest(blobinfo.Digest)
	if err != nil && errors.Cause(err) != storage.ErrLayerUnknown {
		return false, types.BlobInfo{}, errors.Wrapf(err, `Error looking for layers with digest %q`, blobinfo.Digest)
	}
	if len(layers) > 0 {
		// Save this for completeness.
		s.blobDiffIDs[blobinfo.Digest] = layers[0].UncompressedDigest
		return true, types.BlobInfo{
			Digest:    blobinfo.Digest,
			Size:      layers[0].UncompressedSize,
			MediaType: blobinfo.MediaType,
		}, nil
	}

	// Check if we have a was-compressed layer in storage that's based on that blob.
	layers, err = s.imageRef.transport.store.LayersByCompressedDigest(blobinfo.Digest)
	if err != nil && errors.Cause(err) != storage.ErrLayerUnknown {
		return false, types.BlobInfo{}, errors.Wrapf(err, `Error looking for compressed layers with digest %q`, blobinfo.Digest)
	}
	if len(layers) > 0 {
		// Record the uncompressed value so that we can use it to calculate layer IDs.
		s.blobDiffIDs[blobinfo.Digest] = layers[0].UncompressedDigest
		return true, types.BlobInfo{
			Digest:    blobinfo.Digest,
			Size:      layers[0].CompressedSize,
			MediaType: blobinfo.MediaType,
		}, nil
	}

	// Does the blob correspond to a known DiffID which we already have available?
	// Because we must return the size, which is unknown for unavailable compressed blobs, the returned BlobInfo refers to the
	// uncompressed layer, and that can happen only if canSubstitute, or if the incoming manifest already specifies the size.
	if canSubstitute || blobinfo.Size != -1 {
		if uncompressedDigest := cache.UncompressedDigest(blobinfo.Digest); uncompressedDigest != "" && uncompressedDigest != blobinfo.Digest {
			layers, err := s.imageRef.transport.store.LayersByUncompressedDigest(uncompressedDigest)
			if err != nil && errors.Cause(err) != storage.ErrLayerUnknown {
				return false, types.BlobInfo{}, errors.Wrapf(err, `Error looking for layers with digest %q`, uncompressedDigest)
			}
			if len(layers) > 0 {
				if blobinfo.Size != -1 {
					s.blobDiffIDs[blobinfo.Digest] = layers[0].UncompressedDigest
					return true, blobinfo, nil
				}
				if !canSubstitute {
					return false, types.BlobInfo{}, fmt.Errorf("Internal error: canSubstitute was expected to be true for blobInfo %v", blobinfo)
				}
				s.blobDiffIDs[uncompressedDigest] = layers[0].UncompressedDigest
				return true, types.BlobInfo{
					Digest:    uncompressedDigest,
					Size:      layers[0].UncompressedSize,
					MediaType: blobinfo.MediaType,
				}, nil
			}
		}
	}

	// Nope, we don't have it.
	return false, types.BlobInfo{}, nil
}

// computeID computes a recommended image ID based on information we have so far.  If
// the manifest is not of a type that we recognize, we return an empty value, indicating
// that since we don't have a recommendation, a random ID should be used if one needs
// to be allocated.
func (s *storageImageDestination) computeID(m manifest.Manifest) string {
	// Build the diffID list.  We need the decompressed sums that we've been calculating to
	// fill in the DiffIDs.  It's expected (but not enforced by us) that the number of
	// diffIDs corresponds to the number of non-EmptyLayer entries in the history.
	var diffIDs []digest.Digest
	switch m := m.(type) {
	case *manifest.Schema1:
		// Build a list of the diffIDs we've generated for the non-throwaway FS layers,
		// in reverse of the order in which they were originally listed.
		for i, compat := range m.ExtractedV1Compatibility {
			if compat.ThrowAway {
				continue
			}
			blobSum := m.FSLayers[i].BlobSum
			diffID, ok := s.blobDiffIDs[blobSum]
			if !ok {
				logrus.Infof("error looking up diffID for layer %q", blobSum.String())
				return ""
			}
			diffIDs = append([]digest.Digest{diffID}, diffIDs...)
		}
	case *manifest.Schema2, *manifest.OCI1:
		// We know the ID calculation for these formats doesn't actually use the diffIDs,
		// so we don't need to populate the diffID list.
	default:
		return ""
	}
	id, err := m.ImageID(diffIDs)
	if err != nil {
		return ""
	}
	return id
}

// getConfigBlob exists only to let us retrieve the configuration blob so that the manifest package can dig
// information out of it for Inspect().
func (s *storageImageDestination) getConfigBlob(info types.BlobInfo) ([]byte, error) {
	if info.Digest == "" {
		return nil, errors.Errorf(`no digest supplied when reading blob`)
	}
	if err := info.Digest.Validate(); err != nil {
		return nil, errors.Wrapf(err, `invalid digest supplied when reading blob`)
	}
	// Assume it's a file, since we're only calling this from a place that expects to read files.
	if filename, ok := s.filenames[info.Digest]; ok {
		contents, err2 := ioutil.ReadFile(filename)
		if err2 != nil {
			return nil, errors.Wrapf(err2, `error reading blob from file %q`, filename)
		}
		return contents, nil
	}
	// If it's not a file, it's a bug, because we're not expecting to be asked for a layer.
	return nil, errors.New("blob not found")
}

func (s *storageImageDestination) Commit(ctx context.Context, unparsedToplevel types.UnparsedImage) error {
	if len(s.manifest) == 0 {
		return errors.New("Internal error: storageImageDestination.Commit() called without PutManifest()")
	}
	toplevelManifest, _, err := unparsedToplevel.Manifest(ctx)
	if err != nil {
		return errors.Wrapf(err, "error retrieving top-level manifest")
	}
	// If the name we're saving to includes a digest, then check that the
	// manifests that we're about to save all either match the one from the
	// unparsedToplevel, or match the digest in the name that we're using.
	if s.imageRef.named != nil {
		if digested, ok := s.imageRef.named.(reference.Digested); ok {
			matches, err := manifest.MatchesDigest(s.manifest, digested.Digest())
			if err != nil {
				return err
			}
			if !matches {
				matches, err = manifest.MatchesDigest(toplevelManifest, digested.Digest())
				if err != nil {
					return err
				}
			}
			if !matches {
				return fmt.Errorf("Manifest to be saved does not match expected digest %s", digested.Digest())
			}
		}
	}
	// Find the list of layer blobs.
	if len(s.manifest) == 0 {
		return errors.New("Internal error: storageImageDestination.Commit() called without PutManifest()")
	}
	man, err := manifest.FromBlob(s.manifest, manifest.GuessMIMEType(s.manifest))
	if err != nil {
		return errors.Wrapf(err, "error parsing manifest")
	}
	layerBlobs := man.LayerInfos()
	// Extract or find the layers.
	lastLayer := ""
	for _, blob := range layerBlobs {
		if blob.EmptyLayer {
			continue
		}

		// Check if there's already a layer with the ID that we'd give to the result of applying
		// this layer blob to its parent, if it has one, or the blob's hex value otherwise.
		diffID, haveDiffID := s.blobDiffIDs[blob.Digest]
		if !haveDiffID {
			// Check if it's elsewhere and the caller just forgot to pass it to us in a PutBlob(),
			// or to even check if we had it.
			// Use none.NoCache to avoid a repeated DiffID lookup in the BlobInfoCache; a caller
			// that relies on using a blob digest that has never been seeen by the store had better call
			// TryReusingBlob; not calling PutBlob already violates the documented API, so there’s only
			// so far we are going to accommodate that (if we should be doing that at all).
			logrus.Debugf("looking for diffID for blob %+v", blob.Digest)
			has, _, err := s.TryReusingBlob(ctx, blob.BlobInfo, none.NoCache, false)
			if err != nil {
				return errors.Wrapf(err, "error checking for a layer based on blob %q", blob.Digest.String())
			}
			if !has {
				return errors.Errorf("error determining uncompressed digest for blob %q", blob.Digest.String())
			}
			diffID, haveDiffID = s.blobDiffIDs[blob.Digest]
			if !haveDiffID {
				return errors.Errorf("we have blob %q, but don't know its uncompressed digest", blob.Digest.String())
			}
		}
		id := diffID.Hex()
		if lastLayer != "" {
			id = digest.Canonical.FromBytes([]byte(lastLayer + "+" + diffID.Hex())).Hex()
		}
		if layer, err2 := s.imageRef.transport.store.Layer(id); layer != nil && err2 == nil {
			// There's already a layer that should have the right contents, just reuse it.
			lastLayer = layer.ID
			continue
		}
		// Check if we previously cached a file with that blob's contents.  If we didn't,
		// then we need to read the desired contents from a layer.
		filename, ok := s.filenames[blob.Digest]
		if !ok {
			// Try to find the layer with contents matching that blobsum.
			layer := ""
			layers, err2 := s.imageRef.transport.store.LayersByUncompressedDigest(diffID)
			if err2 == nil && len(layers) > 0 {
				layer = layers[0].ID
			} else {
				layers, err2 = s.imageRef.transport.store.LayersByCompressedDigest(blob.Digest)
				if err2 == nil && len(layers) > 0 {
					layer = layers[0].ID
				}
			}
			if layer == "" {
				return errors.Wrapf(err2, "error locating layer for blob %q", blob.Digest)
			}
			// Read the layer's contents.
			noCompression := archive.Uncompressed
			diffOptions := &storage.DiffOptions{
				Compression: &noCompression,
			}
			diff, err2 := s.imageRef.transport.store.Diff("", layer, diffOptions)
			if err2 != nil {
				return errors.Wrapf(err2, "error reading layer %q for blob %q", layer, blob.Digest)
			}
			// Copy the layer diff to a file.  Diff() takes a lock that it holds
			// until the ReadCloser that it returns is closed, and PutLayer() wants
			// the same lock, so the diff can't just be directly streamed from one
			// to the other.
			filename = s.computeNextBlobCacheFile()
			file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_EXCL, 0600)
			if err != nil {
				diff.Close()
				return errors.Wrapf(err, "error creating temporary file %q", filename)
			}
			// Copy the data to the file.
			// TODO: This can take quite some time, and should ideally be cancellable using
			// ctx.Done().
			_, err = io.Copy(file, diff)
			diff.Close()
			file.Close()
			if err != nil {
				return errors.Wrapf(err, "error storing blob to file %q", filename)
			}
			// Make sure that we can find this file later, should we need the layer's
			// contents again.
			s.filenames[blob.Digest] = filename
		}
		// Read the cached blob and use it as a diff.
		file, err := os.Open(filename)
		if err != nil {
			return errors.Wrapf(err, "error opening file %q", filename)
		}
		defer file.Close()
		// Build the new layer using the diff, regardless of where it came from.
		// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
		layer, _, err := s.imageRef.transport.store.PutLayer(id, lastLayer, nil, "", false, nil, file)
		if err != nil && errors.Cause(err) != storage.ErrDuplicateID {
			return errors.Wrapf(err, "error adding layer with blob %q", blob.Digest)
		}
		lastLayer = layer.ID
	}

	// If one of those blobs was a configuration blob, then we can try to dig out the date when the image
	// was originally created, in case we're just copying it.  If not, no harm done.
	options := &storage.ImageOptions{}
	if inspect, err := man.Inspect(s.getConfigBlob); err == nil && inspect.Created != nil {
		logrus.Debugf("setting image creation date to %s", inspect.Created)
		options.CreationDate = *inspect.Created
	}
	// Create the image record, pointing to the most-recently added layer.
	intendedID := s.imageRef.id
	if intendedID == "" {
		intendedID = s.computeID(man)
	}
	oldNames := []string{}
	img, err := s.imageRef.transport.store.CreateImage(intendedID, nil, lastLayer, "", options)
	if err != nil {
		if errors.Cause(err) != storage.ErrDuplicateID {
			logrus.Debugf("error creating image: %q", err)
			return errors.Wrapf(err, "error creating image %q", intendedID)
		}
		img, err = s.imageRef.transport.store.Image(intendedID)
		if err != nil {
			return errors.Wrapf(err, "error reading image %q", intendedID)
		}
		if img.TopLayer != lastLayer {
			logrus.Debugf("error creating image: image with ID %q exists, but uses different layers", intendedID)
			return errors.Wrapf(storage.ErrDuplicateID, "image with ID %q already exists, but uses a different top layer", intendedID)
		}
		logrus.Debugf("reusing image ID %q", img.ID)
		oldNames = append(oldNames, img.Names...)
	} else {
		logrus.Debugf("created new image ID %q", img.ID)
	}
	// Add the non-layer blobs as data items.  Since we only share layers, they should all be in files, so
	// we just need to screen out the ones that are actually layers to get the list of non-layers.
	dataBlobs := make(map[digest.Digest]struct{})
	for blob := range s.filenames {
		dataBlobs[blob] = struct{}{}
	}
	for _, layerBlob := range layerBlobs {
		delete(dataBlobs, layerBlob.Digest)
	}
	for blob := range dataBlobs {
		v, err := ioutil.ReadFile(s.filenames[blob])
		if err != nil {
			return errors.Wrapf(err, "error copying non-layer blob %q to image", blob)
		}
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, blob.String(), v, manifest.Digest); err != nil {
			if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
				logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
			}
			logrus.Debugf("error saving big data %q for image %q: %v", blob.String(), img.ID, err)
			return errors.Wrapf(err, "error saving big data %q for image %q", blob.String(), img.ID)
		}
	}
	// Set the reference's name on the image.  We don't need to worry about avoiding duplicate
	// values because SetNames() will deduplicate the list that we pass to it.
	if name := s.imageRef.DockerReference(); len(oldNames) > 0 || name != nil {
		names := []string{}
		if name != nil {
			names = append(names, name.String())
		}
		if len(oldNames) > 0 {
			names = append(names, oldNames...)
		}
		if err := s.imageRef.transport.store.SetNames(img.ID, names); err != nil {
			if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
				logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
			}
			logrus.Debugf("error setting names %v on image %q: %v", names, img.ID, err)
			return errors.Wrapf(err, "error setting names %v on image %q", names, img.ID)
		}
		logrus.Debugf("set names of image %q to %v", img.ID, names)
	}
	// Save the unparsedToplevel's manifest.
	if len(toplevelManifest) != 0 {
		manifestDigest, err := manifest.Digest(toplevelManifest)
		if err != nil {
			return errors.Wrapf(err, "error digesting top-level manifest")
		}
		key := manifestBigDataKey(manifestDigest)
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, toplevelManifest, manifest.Digest); err != nil {
			if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
				logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
			}
			logrus.Debugf("error saving top-level manifest for image %q: %v", img.ID, err)
			return errors.Wrapf(err, "error saving top-level manifest for image %q", img.ID)
		}
	}
	// Save the image's manifest.  Allow looking it up by digest by using the key convention defined by the Store.
	// Record the manifest twice: using a digest-specific key to allow references to that specific digest instance,
	// and using storage.ImageDigestBigDataKey for future users that don’t specify any digest and for compatibility with older readers.
	manifestDigest, err := manifest.Digest(s.manifest)
	if err != nil {
		return errors.Wrapf(err, "error computing manifest digest")
	}
	key := manifestBigDataKey(manifestDigest)
	if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, s.manifest, manifest.Digest); err != nil {
		if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
			logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
		}
		logrus.Debugf("error saving manifest for image %q: %v", img.ID, err)
		return errors.Wrapf(err, "error saving manifest for image %q", img.ID)
	}
	key = storage.ImageDigestBigDataKey
	if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, s.manifest, manifest.Digest); err != nil {
		if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
			logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
		}
		logrus.Debugf("error saving manifest for image %q: %v", img.ID, err)
		return errors.Wrapf(err, "error saving manifest for image %q", img.ID)
	}
	// Save the signatures, if we have any.
	if len(s.signatures) > 0 {
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, "signatures", s.signatures, manifest.Digest); err != nil {
			if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
				logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
			}
			logrus.Debugf("error saving signatures for image %q: %v", img.ID, err)
			return errors.Wrapf(err, "error saving signatures for image %q", img.ID)
		}
	}
	for instanceDigest, signatures := range s.signatureses {
		key := signatureBigDataKey(instanceDigest)
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, signatures, manifest.Digest); err != nil {
			if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
				logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
			}
			logrus.Debugf("error saving signatures for image %q: %v", img.ID, err)
			return errors.Wrapf(err, "error saving signatures for image %q", img.ID)
		}
	}
	// Save our metadata.
	metadata, err := json.Marshal(s)
	if err != nil {
		if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
			logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
		}
		logrus.Debugf("error encoding metadata for image %q: %v", img.ID, err)
		return errors.Wrapf(err, "error encoding metadata for image %q", img.ID)
	}
	if len(metadata) != 0 {
		if err = s.imageRef.transport.store.SetMetadata(img.ID, string(metadata)); err != nil {
			if _, err2 := s.imageRef.transport.store.DeleteImage(img.ID, true); err2 != nil {
				logrus.Debugf("error deleting incomplete image %q: %v", img.ID, err2)
			}
			logrus.Debugf("error saving metadata for image %q: %v", img.ID, err)
			return errors.Wrapf(err, "error saving metadata for image %q", img.ID)
		}
		logrus.Debugf("saved image metadata %q", string(metadata))
	}
	return nil
}

var manifestMIMETypes = []string{
	imgspecv1.MediaTypeImageManifest,
	manifest.DockerV2Schema2MediaType,
	manifest.DockerV2Schema1SignedMediaType,
	manifest.DockerV2Schema1MediaType,
}

func (s *storageImageDestination) SupportedManifestMIMETypes() []string {
	return manifestMIMETypes
}

// PutManifest writes the manifest to the destination.
func (s *storageImageDestination) PutManifest(ctx context.Context, manifestBlob []byte, instanceDigest *digest.Digest) error {
	newBlob := make([]byte, len(manifestBlob))
	copy(newBlob, manifestBlob)
	s.manifest = newBlob
	return nil
}

// SupportsSignatures returns an error if we can't expect GetSignatures() to return data that was
// previously supplied to PutSignatures().
func (s *storageImageDestination) SupportsSignatures(ctx context.Context) error {
	return nil
}

// AcceptsForeignLayerURLs returns false iff foreign layers in the manifest should actually be
// uploaded to the image destination, true otherwise.
func (s *storageImageDestination) AcceptsForeignLayerURLs() bool {
	return false
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime architecture and OS. False otherwise.
func (s *storageImageDestination) MustMatchRuntimeOS() bool {
	return true
}

// IgnoresEmbeddedDockerReference returns true iff the destination does not care about Image.EmbeddedDockerReferenceConflicts(),
// and would prefer to receive an unmodified manifest instead of one modified for the destination.
// Does not make a difference if Reference().DockerReference() is nil.
func (s *storageImageDestination) IgnoresEmbeddedDockerReference() bool {
	return true // Yes, we want the unmodified manifest
}

// PutSignatures records the image's signatures for committing as a single data blob.
func (s *storageImageDestination) PutSignatures(ctx context.Context, signatures [][]byte, instanceDigest *digest.Digest) error {
	sizes := []int{}
	sigblob := []byte{}
	for _, sig := range signatures {
		sizes = append(sizes, len(sig))
		newblob := make([]byte, len(sigblob)+len(sig))
		copy(newblob, sigblob)
		copy(newblob[len(sigblob):], sig)
		sigblob = newblob
	}
	if instanceDigest == nil {
		s.signatures = sigblob
		s.SignatureSizes = sizes
	}
	if instanceDigest == nil && len(s.manifest) > 0 {
		manifestDigest, err := manifest.Digest(s.manifest)
		if err != nil {
			return err
		}
		instanceDigest = &manifestDigest
	}
	if instanceDigest != nil {
		s.signatureses[*instanceDigest] = sigblob
		s.SignaturesSizes[*instanceDigest] = sizes
	}
	return nil
}

// getSize() adds up the sizes of the image's data blobs (which includes the configuration blob), the
// signatures, and the uncompressed sizes of all of the image's layers.
func (s *storageImageSource) getSize() (int64, error) {
	var sum int64
	// Size up the data blobs.
	dataNames, err := s.imageRef.transport.store.ListImageBigData(s.image.ID)
	if err != nil {
		return -1, errors.Wrapf(err, "error reading image %q", s.image.ID)
	}
	for _, dataName := range dataNames {
		bigSize, err := s.imageRef.transport.store.ImageBigDataSize(s.image.ID, dataName)
		if err != nil {
			return -1, errors.Wrapf(err, "error reading data blob size %q for %q", dataName, s.image.ID)
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
			return -1, errors.Errorf("size for layer %q is unknown, failing getSize()", layerID)
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

// Size() returns the previously-computed size of the image, with no error.
func (s *storageImageCloser) Size() (int64, error) {
	return s.size, nil
}

// newImage creates an image that also knows its size
func newImage(ctx context.Context, sys *types.SystemContext, s storageReference) (types.ImageCloser, error) {
	src, err := newImageSource(ctx, sys, s)
	if err != nil {
		return nil, err
	}
	img, err := image.FromSource(ctx, sys, src)
	if err != nil {
		return nil, err
	}
	size, err := src.getSize()
	if err != nil {
		return nil, err
	}
	return &storageImageCloser{ImageCloser: img, size: size}, nil
}
