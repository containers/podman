//go:build !containers_image_storage_stub
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
	"github.com/containers/image/v5/internal/putblobdigest"
	"github.com/containers/image/v5/internal/tmpdir"
	internalTypes "github.com/containers/image/v5/internal/types"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/blobinfocache/none"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	graphdriver "github.com/containers/storage/drivers"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chunked"
	"github.com/containers/storage/pkg/ioutils"
	digest "github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	// ErrBlobDigestMismatch could potentially be returned when PutBlob() is given a blob
	// with a digest-based name that doesn't match its contents.
	// Deprecated: PutBlob() doesn't do this any more (it just accepts the caller’s value),
	// and there is no known user of this error.
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
	systemContext   *types.SystemContext    // SystemContext used in GetBlob() to create temporary files
	layerPosition   map[digest.Digest]int   // Where we are in reading a blob's layers
	cachedManifest  []byte                  // A cached copy of the manifest, if already known, or nil
	getBlobMutex    sync.Mutex              // Mutex to sync state for parallel GetBlob executions
	SignatureSizes  []int                   `json:"signature-sizes,omitempty"`  // List of sizes of each signature slice
	SignaturesSizes map[digest.Digest][]int `json:"signatures-sizes,omitempty"` // List of sizes of each signature slice
}

type storageImageDestination struct {
	imageRef        storageReference
	directory       string                   // Temporary directory where we store blobs until Commit() time
	nextTempFileID  int32                    // A counter that we use for computing filenames to assign to blobs
	manifest        []byte                   // Manifest contents, temporary
	manifestDigest  digest.Digest            // Valid if len(manifest) != 0
	signatures      []byte                   // Signature contents, temporary
	signatureses    map[digest.Digest][]byte // Instance signature contents, temporary
	SignatureSizes  []int                    `json:"signature-sizes,omitempty"`  // List of sizes of each signature slice
	SignaturesSizes map[digest.Digest][]int  `json:"signatures-sizes,omitempty"` // Sizes of each manifest's signature slice

	// A storage destination may be used concurrently.  Accesses are
	// serialized via a mutex.  Please refer to the individual comments
	// below for details.
	lock sync.Mutex
	// Mapping from layer (by index) to the associated ID in the storage.
	// It's protected *implicitly* since `commitLayer()`, at any given
	// time, can only be executed by *one* goroutine.  Please refer to
	// `queueOrCommit()` for further details on how the single-caller
	// guarantee is implemented.
	indexToStorageID map[int]*string
	// All accesses to below data are protected by `lock` which is made
	// *explicit* in the code.
	blobDiffIDs            map[digest.Digest]digest.Digest                       // Mapping from layer blobsums to their corresponding DiffIDs
	fileSizes              map[digest.Digest]int64                               // Mapping from layer blobsums to their sizes
	filenames              map[digest.Digest]string                              // Mapping from layer blobsums to names of files we used to hold them
	currentIndex           int                                                   // The index of the layer to be committed (i.e., lower indices have already been committed)
	indexToPulledLayerInfo map[int]*manifest.LayerInfo                           // Mapping from layer (by index) to pulled down blob
	blobAdditionalLayer    map[digest.Digest]storage.AdditionalLayer             // Mapping from layer blobsums to their corresponding additional layer
	diffOutputs            map[digest.Digest]*graphdriver.DriverWithDifferOutput // Mapping from digest to differ output
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
		systemContext:   sys,
		image:           img,
		layerPosition:   make(map[digest.Digest]int),
		SignatureSizes:  []int{},
		SignaturesSizes: make(map[digest.Digest][]int),
	}
	if img.Metadata != "" {
		if err := json.Unmarshal([]byte(img.Metadata), image); err != nil {
			return nil, errors.Wrap(err, "decoding metadata for source image")
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

	// NOTE: the blob is first written to a temporary file and subsequently
	// closed.  The intention is to keep the time we own the storage lock
	// as short as possible to allow other processes to access the storage.
	rc, n, _, err = s.getBlobAndLayerID(info)
	if err != nil {
		return nil, 0, err
	}
	defer rc.Close()

	tmpFile, err := ioutil.TempFile(tmpdir.TemporaryDirectoryForBigFiles(s.systemContext), "")
	if err != nil {
		return nil, 0, err
	}

	if _, err := io.Copy(tmpFile, rc); err != nil {
		return nil, 0, err
	}

	if _, err := tmpFile.Seek(0, 0); err != nil {
		return nil, 0, err
	}

	wrapper := ioutils.NewReadCloserWrapper(tmpFile, func() error {
		defer os.Remove(tmpFile.Name())
		return tmpFile.Close()
	})

	return wrapper, n, err
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
			return nil, "", errors.Wrapf(err, "reading manifest for image instance %q", *instanceDigest)
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
		return nil, errors.Wrapf(err, "reading image manifest for %q", s.image.ID)
	}
	if manifest.MIMETypeIsMultiImage(manifestType) {
		return nil, errors.Errorf("can't copy layers for a manifest list (shouldn't be attempted)")
	}
	man, err := manifest.FromBlob(manifestBlob, manifestType)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing image manifest for %q", s.image.ID)
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
			return nil, errors.Wrapf(err, "reading layer %q in image %q", layerID, s.image.ID)
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
		return nil, errors.Wrapf(err, "creating LayerInfosForCopy of image %q", s.image.ID)
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
			return nil, errors.Wrapf(err, "looking up signatures data for image %q (%s)", s.image.ID, instance)
		}
		signature = signatureBlob
	}
	for _, length := range signatureSizes {
		if offset+length > len(signature) {
			return nil, errors.Wrapf(err, "looking up signatures data for image %q (%s): expected at least %d bytes, only found %d", s.image.ID, instance, len(signature), offset+length)
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
		return nil, errors.Wrapf(err, "creating a temporary directory")
	}
	image := &storageImageDestination{
		imageRef:               imageRef,
		directory:              directory,
		signatureses:           make(map[digest.Digest][]byte),
		blobDiffIDs:            make(map[digest.Digest]digest.Digest),
		blobAdditionalLayer:    make(map[digest.Digest]storage.AdditionalLayer),
		fileSizes:              make(map[digest.Digest]int64),
		filenames:              make(map[digest.Digest]string),
		SignatureSizes:         []int{},
		SignaturesSizes:        make(map[digest.Digest][]int),
		indexToStorageID:       make(map[int]*string),
		indexToPulledLayerInfo: make(map[int]*manifest.LayerInfo),
		diffOutputs:            make(map[digest.Digest]*graphdriver.DriverWithDifferOutput),
	}
	return image, nil
}

// Reference returns the reference used to set up this destination.  Note that this should directly correspond to user's intent,
// e.g. it should use the public hostname instead of the result of resolving CNAMEs or following redirects.
func (s *storageImageDestination) Reference() types.ImageReference {
	return s.imageRef
}

// Close cleans up the temporary directory and additional layer store handlers.
func (s *storageImageDestination) Close() error {
	for _, al := range s.blobAdditionalLayer {
		al.Release()
	}
	for _, v := range s.diffOutputs {
		if v.Target != "" {
			_ = s.imageRef.transport.store.CleanupStagingDirectory(v.Target)
		}
	}
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

// PutBlobWithOptions is a wrapper around PutBlob.  If options.LayerIndex is
// set, the blob will be committed directly.  Either by the calling goroutine
// or by another goroutine already committing layers.
//
// Please not that TryReusingBlobWithOptions and PutBlobWithOptions *must* be
// used the together.  Mixing the two with non "WithOptions" functions is not
// supported.
func (s *storageImageDestination) PutBlobWithOptions(ctx context.Context, stream io.Reader, blobinfo types.BlobInfo, options internalTypes.PutBlobOptions) (types.BlobInfo, error) {
	info, err := s.PutBlob(ctx, stream, blobinfo, options.Cache, options.IsConfig)
	if err != nil {
		return info, err
	}

	if options.IsConfig || options.LayerIndex == nil {
		return info, nil
	}

	return info, s.queueOrCommit(ctx, info, *options.LayerIndex, options.EmptyLayer)
}

// HasThreadSafePutBlob indicates whether PutBlob can be executed concurrently.
func (s *storageImageDestination) HasThreadSafePutBlob() bool {
	return true
}

// PutBlob writes contents of stream and returns data representing the result.
// inputInfo.Digest can be optionally provided if known; if provided, and stream is read to the end without error, the digest MUST match the stream contents.
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
	if blobinfo.Digest != "" {
		if err := blobinfo.Digest.Validate(); err != nil {
			return errorBlobInfo, fmt.Errorf("invalid digest %#v: %w", blobinfo.Digest.String(), err)
		}
	}

	// Set up to digest the blob if necessary, and count its size while saving it to a file.
	filename := s.computeNextBlobCacheFile()
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return errorBlobInfo, errors.Wrapf(err, "creating temporary file %q", filename)
	}
	defer file.Close()
	counter := ioutils.NewWriteCounter(file)
	stream = io.TeeReader(stream, counter)
	digester, stream := putblobdigest.DigestIfUnknown(stream, blobinfo)
	decompressed, err := archive.DecompressStream(stream)
	if err != nil {
		return errorBlobInfo, errors.Wrap(err, "setting up to decompress blob")
	}

	diffID := digest.Canonical.Digester()
	// Copy the data to the file.
	// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
	_, err = io.Copy(diffID.Hash(), decompressed)
	decompressed.Close()
	if err != nil {
		return errorBlobInfo, errors.Wrapf(err, "storing blob to file %q", filename)
	}

	// Determine blob properties, and fail if information that we were given about the blob
	// is known to be incorrect.
	blobDigest := digester.Digest()
	blobSize := blobinfo.Size
	if blobSize < 0 {
		blobSize = counter.Count
	} else if blobinfo.Size != counter.Count {
		return errorBlobInfo, errors.WithStack(ErrBlobSizeMismatch)
	}

	// Record information about the blob.
	s.lock.Lock()
	s.blobDiffIDs[blobDigest] = diffID.Digest()
	s.fileSizes[blobDigest] = counter.Count
	s.filenames[blobDigest] = filename
	s.lock.Unlock()
	// This is safe because we have just computed diffID, and blobDigest was either computed
	// by us, or validated by the caller (usually copy.digestingReader).
	cache.RecordDigestUncompressedPair(blobDigest, diffID.Digest())
	return types.BlobInfo{
		Digest:    blobDigest,
		Size:      blobSize,
		MediaType: blobinfo.MediaType,
	}, nil
}

// TryReusingBlobWithOptions is a wrapper around TryReusingBlob.  If
// options.LayerIndex is set, the reused blob will be recoreded as already
// pulled.
//
// Please not that TryReusingBlobWithOptions and PutBlobWithOptions *must* be
// used the together.  Mixing the two with the non "WithOptions" functions
// is not supported.
func (s *storageImageDestination) TryReusingBlobWithOptions(ctx context.Context, blobinfo types.BlobInfo, options internalTypes.TryReusingBlobOptions) (bool, types.BlobInfo, error) {
	reused, info, err := s.tryReusingBlobWithSrcRef(ctx, blobinfo, options.Cache, options.CanSubstitute, options.SrcRef)
	if err != nil || !reused || options.LayerIndex == nil {
		return reused, info, err
	}

	return reused, info, s.queueOrCommit(ctx, info, *options.LayerIndex, options.EmptyLayer)
}

// tryReusingBlobWithSrcRef is a wrapper around TryReusingBlob.
// If ref is provided, this function first tries to get layer from Additional Layer Store.
func (s *storageImageDestination) tryReusingBlobWithSrcRef(ctx context.Context, blobinfo types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool, ref reference.Named) (bool, types.BlobInfo, error) {
	// lock the entire method as it executes fairly quickly
	s.lock.Lock()
	defer s.lock.Unlock()

	if ref != nil {
		// Check if we have the layer in the underlying additional layer store.
		aLayer, err := s.imageRef.transport.store.LookupAdditionalLayer(blobinfo.Digest, ref.String())
		if err != nil && errors.Cause(err) != storage.ErrLayerUnknown {
			return false, types.BlobInfo{}, errors.Wrapf(err, `looking for compressed layers with digest %q and labels`, blobinfo.Digest)
		} else if err == nil {
			// Record the uncompressed value so that we can use it to calculate layer IDs.
			s.blobDiffIDs[blobinfo.Digest] = aLayer.UncompressedDigest()
			s.blobAdditionalLayer[blobinfo.Digest] = aLayer
			return true, types.BlobInfo{
				Digest:    blobinfo.Digest,
				Size:      aLayer.CompressedSize(),
				MediaType: blobinfo.MediaType,
			}, nil
		}
	}

	return s.tryReusingBlobLocked(ctx, blobinfo, cache, canSubstitute)
}

type zstdFetcher struct {
	stream   internalTypes.ImageSourceSeekable
	ctx      context.Context
	blobInfo types.BlobInfo
}

// GetBlobAt converts from chunked.GetBlobAt to ImageSourceSeekable.GetBlobAt.
func (f *zstdFetcher) GetBlobAt(chunks []chunked.ImageSourceChunk) (chan io.ReadCloser, chan error, error) {
	var newChunks []internalTypes.ImageSourceChunk
	for _, v := range chunks {
		i := internalTypes.ImageSourceChunk{
			Offset: v.Offset,
			Length: v.Length,
		}
		newChunks = append(newChunks, i)
	}
	rc, errs, err := f.stream.GetBlobAt(f.ctx, f.blobInfo, newChunks)
	if _, ok := err.(internalTypes.BadPartialRequestError); ok {
		err = chunked.ErrBadRequest{}
	}
	return rc, errs, err

}

// PutBlobPartial attempts to create a blob using the data that is already present at the destination storage.  stream is accessed
// in a non-sequential way to retrieve the missing chunks.
func (s *storageImageDestination) PutBlobPartial(ctx context.Context, stream internalTypes.ImageSourceSeekable, srcInfo types.BlobInfo, cache types.BlobInfoCache) (types.BlobInfo, error) {
	fetcher := zstdFetcher{
		stream:   stream,
		ctx:      ctx,
		blobInfo: srcInfo,
	}

	differ, err := chunked.GetDiffer(ctx, s.imageRef.transport.store, srcInfo.Size, srcInfo.Annotations, &fetcher)
	if err != nil {
		return srcInfo, err
	}

	out, err := s.imageRef.transport.store.ApplyDiffWithDiffer("", nil, differ)
	if err != nil {
		return srcInfo, err
	}

	blobDigest := srcInfo.Digest

	s.lock.Lock()
	s.blobDiffIDs[blobDigest] = blobDigest
	s.fileSizes[blobDigest] = 0
	s.filenames[blobDigest] = ""
	s.diffOutputs[blobDigest] = out
	s.lock.Unlock()

	return srcInfo, nil
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
func (s *storageImageDestination) TryReusingBlob(ctx context.Context, blobinfo types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
	// lock the entire method as it executes fairly quickly
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.tryReusingBlobLocked(ctx, blobinfo, cache, canSubstitute)
}

// tryReusingBlobLocked implements a core functionality of TryReusingBlob.
// This must be called with a lock being held on storageImageDestination.
func (s *storageImageDestination) tryReusingBlobLocked(ctx context.Context, blobinfo types.BlobInfo, cache types.BlobInfoCache, canSubstitute bool) (bool, types.BlobInfo, error) {
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
		return false, types.BlobInfo{}, errors.Wrapf(err, `looking for layers with digest %q`, blobinfo.Digest)
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
		return false, types.BlobInfo{}, errors.Wrapf(err, `looking for compressed layers with digest %q`, blobinfo.Digest)
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
				return false, types.BlobInfo{}, errors.Wrapf(err, `looking for layers with digest %q`, uncompressedDigest)
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
			return nil, errors.Wrapf(err2, `reading blob from file %q`, filename)
		}
		return contents, nil
	}
	// If it's not a file, it's a bug, because we're not expecting to be asked for a layer.
	return nil, errors.New("blob not found")
}

// queueOrCommit queues in the specified blob to be committed to the storage.
// If no other goroutine is already committing layers, the layer and all
// subsequent layers (if already queued) will be committed to the storage.
func (s *storageImageDestination) queueOrCommit(ctx context.Context, blob types.BlobInfo, index int, emptyLayer bool) error {
	// NOTE: whenever the code below is touched, make sure that all code
	// paths unlock the lock and to unlock it exactly once.
	//
	// Conceptually, the code is divided in two stages:
	//
	// 1) Queue in work by marking the layer as ready to be committed.
	//    If at least one previous/parent layer with a lower index has
	//    not yet been committed, return early.
	//
	// 2) Process the queued-in work by committing the "ready" layers
	//    in sequence.  Make sure that more items can be queued-in
	//    during the comparatively I/O expensive task of committing a
	//    layer.
	//
	// The conceptual benefit of this design is that caller can continue
	// pulling layers after an early return.  At any given time, only one
	// caller is the "worker" routine committing layers.  All other routines
	// can continue pulling and queuing in layers.
	s.lock.Lock()
	s.indexToPulledLayerInfo[index] = &manifest.LayerInfo{
		BlobInfo:   blob,
		EmptyLayer: emptyLayer,
	}

	// We're still waiting for at least one previous/parent layer to be
	// committed, so there's nothing to do.
	if index != s.currentIndex {
		s.lock.Unlock()
		return nil
	}

	for info := s.indexToPulledLayerInfo[index]; info != nil; info = s.indexToPulledLayerInfo[index] {
		s.lock.Unlock()
		// Note: commitLayer locks on-demand.
		if err := s.commitLayer(ctx, *info, index); err != nil {
			return err
		}
		s.lock.Lock()
		index++
	}

	// Set the index at the very end to make sure that only one routine
	// enters stage 2).
	s.currentIndex = index
	s.lock.Unlock()
	return nil
}

// commitLayer commits the specified blob with the given index to the storage.
// Note that the previous layer is expected to already be committed.
//
// Caution: this function must be called without holding `s.lock`.  Callers
// must guarantee that, at any given time, at most one goroutine may execute
// `commitLayer()`.
func (s *storageImageDestination) commitLayer(ctx context.Context, blob manifest.LayerInfo, index int) error {
	// Already committed?  Return early.
	if _, alreadyCommitted := s.indexToStorageID[index]; alreadyCommitted {
		return nil
	}

	// Start with an empty string or the previous layer ID.  Note that
	// `s.indexToStorageID` can only be accessed by *one* goroutine at any
	// given time. Hence, we don't need to lock accesses.
	var lastLayer string
	if prev := s.indexToStorageID[index-1]; prev != nil {
		lastLayer = *prev
	}

	// Carry over the previous ID for empty non-base layers.
	if blob.EmptyLayer {
		s.indexToStorageID[index] = &lastLayer
		return nil
	}

	// Check if there's already a layer with the ID that we'd give to the result of applying
	// this layer blob to its parent, if it has one, or the blob's hex value otherwise.
	s.lock.Lock()
	diffID, haveDiffID := s.blobDiffIDs[blob.Digest]
	s.lock.Unlock()
	if !haveDiffID {
		// Check if it's elsewhere and the caller just forgot to pass it to us in a PutBlob(),
		// or to even check if we had it.
		// Use none.NoCache to avoid a repeated DiffID lookup in the BlobInfoCache; a caller
		// that relies on using a blob digest that has never been seen by the store had better call
		// TryReusingBlob; not calling PutBlob already violates the documented API, so there’s only
		// so far we are going to accommodate that (if we should be doing that at all).
		logrus.Debugf("looking for diffID for blob %+v", blob.Digest)
		// NOTE: use `TryReusingBlob` to prevent recursion.
		has, _, err := s.TryReusingBlob(ctx, blob.BlobInfo, none.NoCache, false)
		if err != nil {
			return errors.Wrapf(err, "checking for a layer based on blob %q", blob.Digest.String())
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
		s.indexToStorageID[index] = &lastLayer
		return nil
	}

	s.lock.Lock()
	diffOutput, ok := s.diffOutputs[blob.Digest]
	s.lock.Unlock()
	if ok {
		layer, err := s.imageRef.transport.store.CreateLayer(id, lastLayer, nil, "", false, nil)
		if err != nil {
			return err
		}

		// FIXME: what to do with the uncompressed digest?
		diffOutput.UncompressedDigest = blob.Digest

		if err := s.imageRef.transport.store.ApplyDiffFromStagingDirectory(layer.ID, diffOutput.Target, diffOutput, nil); err != nil {
			_ = s.imageRef.transport.store.Delete(layer.ID)
			return err
		}

		s.indexToStorageID[index] = &layer.ID
		return nil
	}

	s.lock.Lock()
	al, ok := s.blobAdditionalLayer[blob.Digest]
	s.lock.Unlock()
	if ok {
		layer, err := al.PutAs(id, lastLayer, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to put layer from digest and labels")
		}
		lastLayer = layer.ID
		s.indexToStorageID[index] = &lastLayer
		return nil
	}

	// Check if we previously cached a file with that blob's contents.  If we didn't,
	// then we need to read the desired contents from a layer.
	s.lock.Lock()
	filename, ok := s.filenames[blob.Digest]
	s.lock.Unlock()
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
			return errors.Wrapf(err2, "locating layer for blob %q", blob.Digest)
		}
		// Read the layer's contents.
		noCompression := archive.Uncompressed
		diffOptions := &storage.DiffOptions{
			Compression: &noCompression,
		}
		diff, err2 := s.imageRef.transport.store.Diff("", layer, diffOptions)
		if err2 != nil {
			return errors.Wrapf(err2, "reading layer %q for blob %q", layer, blob.Digest)
		}
		// Copy the layer diff to a file.  Diff() takes a lock that it holds
		// until the ReadCloser that it returns is closed, and PutLayer() wants
		// the same lock, so the diff can't just be directly streamed from one
		// to the other.
		filename = s.computeNextBlobCacheFile()
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_EXCL, 0600)
		if err != nil {
			diff.Close()
			return errors.Wrapf(err, "creating temporary file %q", filename)
		}
		// Copy the data to the file.
		// TODO: This can take quite some time, and should ideally be cancellable using
		// ctx.Done().
		_, err = io.Copy(file, diff)
		diff.Close()
		file.Close()
		if err != nil {
			return errors.Wrapf(err, "storing blob to file %q", filename)
		}
		// Make sure that we can find this file later, should we need the layer's
		// contents again.
		s.lock.Lock()
		s.filenames[blob.Digest] = filename
		s.lock.Unlock()
	}
	// Read the cached blob and use it as a diff.
	file, err := os.Open(filename)
	if err != nil {
		return errors.Wrapf(err, "opening file %q", filename)
	}
	defer file.Close()
	// Build the new layer using the diff, regardless of where it came from.
	// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
	layer, _, err := s.imageRef.transport.store.PutLayer(id, lastLayer, nil, "", false, &storage.LayerOptions{
		OriginalDigest:     blob.Digest,
		UncompressedDigest: diffID,
	}, file)
	if err != nil && errors.Cause(err) != storage.ErrDuplicateID {
		return errors.Wrapf(err, "adding layer with blob %q", blob.Digest)
	}

	s.indexToStorageID[index] = &layer.ID
	return nil
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// unparsedToplevel contains data about the top-level manifest of the source (which may be a single-arch image or a manifest list
// if PutManifest was only called for the single-arch image with instanceDigest == nil), primarily to allow lookups by the
// original manifest list digest, if desired.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (s *storageImageDestination) Commit(ctx context.Context, unparsedToplevel types.UnparsedImage) error {
	if len(s.manifest) == 0 {
		return errors.New("Internal error: storageImageDestination.Commit() called without PutManifest()")
	}
	toplevelManifest, _, err := unparsedToplevel.Manifest(ctx)
	if err != nil {
		return errors.Wrapf(err, "retrieving top-level manifest")
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
	man, err := manifest.FromBlob(s.manifest, manifest.GuessMIMEType(s.manifest))
	if err != nil {
		return errors.Wrapf(err, "parsing manifest")
	}
	layerBlobs := man.LayerInfos()

	// Extract, commit, or find the layers.
	for i, blob := range layerBlobs {
		if err := s.commitLayer(ctx, blob, i); err != nil {
			return err
		}
	}
	var lastLayer string
	if len(layerBlobs) > 0 { // Can happen when using caches
		prev := s.indexToStorageID[len(layerBlobs)-1]
		if prev == nil {
			return errors.Errorf("Internal error: StorageImageDestination.Commit(): previous layer %d hasn't been committed (lastLayer == nil)", len(layerBlobs)-1)
		}
		lastLayer = *prev
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
			return errors.Wrapf(err, "creating image %q", intendedID)
		}
		img, err = s.imageRef.transport.store.Image(intendedID)
		if err != nil {
			return errors.Wrapf(err, "reading image %q", intendedID)
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

	// Clean up the unfinished image on any error.
	// (Is this the right thing to do if the image has existed before?)
	commitSucceeded := false
	defer func() {
		if !commitSucceeded {
			logrus.Errorf("Updating image %q (old names %v) failed, deleting it", img.ID, oldNames)
			if _, err := s.imageRef.transport.store.DeleteImage(img.ID, true); err != nil {
				logrus.Errorf("Error deleting incomplete image %q: %v", img.ID, err)
			}
		}
	}()

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
			return errors.Wrapf(err, "copying non-layer blob %q to image", blob)
		}
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, blob.String(), v, manifest.Digest); err != nil {
			logrus.Debugf("error saving big data %q for image %q: %v", blob.String(), img.ID, err)
			return errors.Wrapf(err, "saving big data %q for image %q", blob.String(), img.ID)
		}
	}
	// Save the unparsedToplevel's manifest if it differs from the per-platform one, which is saved below.
	if len(toplevelManifest) != 0 && !bytes.Equal(toplevelManifest, s.manifest) {
		manifestDigest, err := manifest.Digest(toplevelManifest)
		if err != nil {
			return errors.Wrapf(err, "digesting top-level manifest")
		}
		key := manifestBigDataKey(manifestDigest)
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, toplevelManifest, manifest.Digest); err != nil {
			logrus.Debugf("error saving top-level manifest for image %q: %v", img.ID, err)
			return errors.Wrapf(err, "saving top-level manifest for image %q", img.ID)
		}
	}
	// Save the image's manifest.  Allow looking it up by digest by using the key convention defined by the Store.
	// Record the manifest twice: using a digest-specific key to allow references to that specific digest instance,
	// and using storage.ImageDigestBigDataKey for future users that don’t specify any digest and for compatibility with older readers.
	key := manifestBigDataKey(s.manifestDigest)
	if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, s.manifest, manifest.Digest); err != nil {
		logrus.Debugf("error saving manifest for image %q: %v", img.ID, err)
		return errors.Wrapf(err, "saving manifest for image %q", img.ID)
	}
	key = storage.ImageDigestBigDataKey
	if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, s.manifest, manifest.Digest); err != nil {
		logrus.Debugf("error saving manifest for image %q: %v", img.ID, err)
		return errors.Wrapf(err, "saving manifest for image %q", img.ID)
	}
	// Save the signatures, if we have any.
	if len(s.signatures) > 0 {
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, "signatures", s.signatures, manifest.Digest); err != nil {
			logrus.Debugf("error saving signatures for image %q: %v", img.ID, err)
			return errors.Wrapf(err, "saving signatures for image %q", img.ID)
		}
	}
	for instanceDigest, signatures := range s.signatureses {
		key := signatureBigDataKey(instanceDigest)
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, signatures, manifest.Digest); err != nil {
			logrus.Debugf("error saving signatures for image %q: %v", img.ID, err)
			return errors.Wrapf(err, "saving signatures for image %q", img.ID)
		}
	}
	// Save our metadata.
	metadata, err := json.Marshal(s)
	if err != nil {
		logrus.Debugf("error encoding metadata for image %q: %v", img.ID, err)
		return errors.Wrapf(err, "encoding metadata for image %q", img.ID)
	}
	if len(metadata) != 0 {
		if err = s.imageRef.transport.store.SetMetadata(img.ID, string(metadata)); err != nil {
			logrus.Debugf("error saving metadata for image %q: %v", img.ID, err)
			return errors.Wrapf(err, "saving metadata for image %q", img.ID)
		}
		logrus.Debugf("saved image metadata %q", string(metadata))
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
			logrus.Debugf("error setting names %v on image %q: %v", names, img.ID, err)
			return errors.Wrapf(err, "setting names %v on image %q", names, img.ID)
		}
		logrus.Debugf("set names of image %q to %v", img.ID, names)
	}

	commitSucceeded = true
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
	digest, err := manifest.Digest(manifestBlob)
	if err != nil {
		return err
	}
	newBlob := make([]byte, len(manifestBlob))
	copy(newBlob, manifestBlob)
	s.manifest = newBlob
	s.manifestDigest = digest
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
		if len(s.manifest) > 0 {
			manifestDigest := s.manifestDigest
			instanceDigest = &manifestDigest
		}
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
		return -1, errors.Wrapf(err, "reading image %q", s.image.ID)
	}
	for _, dataName := range dataNames {
		bigSize, err := s.imageRef.transport.store.ImageBigDataSize(s.image.ID, dataName)
		if err != nil {
			return -1, errors.Wrapf(err, "reading data blob size %q for %q", dataName, s.image.ID)
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
