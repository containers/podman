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
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/internal/blobinfocache"
	"github.com/containers/image/v5/internal/imagedestination/impl"
	"github.com/containers/image/v5/internal/imagedestination/stubs"
	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/internal/putblobdigest"
	"github.com/containers/image/v5/internal/set"
	"github.com/containers/image/v5/internal/signature"
	"github.com/containers/image/v5/internal/tmpdir"
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
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

var (
	// ErrBlobDigestMismatch could potentially be returned when PutBlob() is given a blob
	// with a digest-based name that doesn't match its contents.
	// Deprecated: PutBlob() doesn't do this any more (it just accepts the caller’s value),
	// and there is no known user of this error.
	ErrBlobDigestMismatch = errors.New("blob digest mismatch")
	// ErrBlobSizeMismatch is returned when PutBlob() is given a blob
	// with an expected size that doesn't match the reader.
	ErrBlobSizeMismatch = errors.New("blob size mismatch")
)

type storageImageDestination struct {
	impl.Compat
	impl.PropertyMethodsInitialize
	stubs.ImplementsPutBlobPartial
	stubs.AlwaysSupportsSignatures

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
	blobDiffIDs           map[digest.Digest]digest.Digest                       // Mapping from layer blobsums to their corresponding DiffIDs
	fileSizes             map[digest.Digest]int64                               // Mapping from layer blobsums to their sizes
	filenames             map[digest.Digest]string                              // Mapping from layer blobsums to names of files we used to hold them
	currentIndex          int                                                   // The index of the layer to be committed (i.e., lower indices have already been committed)
	indexToAddedLayerInfo map[int]addedLayerInfo                                // Mapping from layer (by index) to blob to add to the image
	blobAdditionalLayer   map[digest.Digest]storage.AdditionalLayer             // Mapping from layer blobsums to their corresponding additional layer
	diffOutputs           map[digest.Digest]*graphdriver.DriverWithDifferOutput // Mapping from digest to differ output
}

// addedLayerInfo records data about a layer to use in this image.
type addedLayerInfo struct {
	digest     digest.Digest
	emptyLayer bool // The layer is an “empty”/“throwaway” one, and may or may not be physically represented in various transport / storage systems.  false if the manifest type does not have the concept.
}

// newImageDestination sets us up to write a new image, caching blobs in a temporary directory until
// it's time to Commit() the image
func newImageDestination(sys *types.SystemContext, imageRef storageReference) (*storageImageDestination, error) {
	directory, err := os.MkdirTemp(tmpdir.TemporaryDirectoryForBigFiles(sys), "storage")
	if err != nil {
		return nil, fmt.Errorf("creating a temporary directory: %w", err)
	}
	dest := &storageImageDestination{
		PropertyMethodsInitialize: impl.PropertyMethods(impl.Properties{
			SupportedManifestMIMETypes: []string{
				imgspecv1.MediaTypeImageManifest,
				manifest.DockerV2Schema2MediaType,
				manifest.DockerV2Schema1SignedMediaType,
				manifest.DockerV2Schema1MediaType,
			},
			// We ultimately have to decompress layers to populate trees on disk
			// and need to explicitly ask for it here, so that the layers' MIME
			// types can be set accordingly.
			DesiredLayerCompression:        types.PreserveOriginal,
			AcceptsForeignLayerURLs:        false,
			MustMatchRuntimeOS:             true,
			IgnoresEmbeddedDockerReference: true, // Yes, we want the unmodified manifest
			HasThreadSafePutBlob:           true,
		}),

		imageRef:              imageRef,
		directory:             directory,
		signatureses:          make(map[digest.Digest][]byte),
		blobDiffIDs:           make(map[digest.Digest]digest.Digest),
		blobAdditionalLayer:   make(map[digest.Digest]storage.AdditionalLayer),
		fileSizes:             make(map[digest.Digest]int64),
		filenames:             make(map[digest.Digest]string),
		SignatureSizes:        []int{},
		SignaturesSizes:       make(map[digest.Digest][]int),
		indexToStorageID:      make(map[int]*string),
		indexToAddedLayerInfo: make(map[int]addedLayerInfo),
		diffOutputs:           make(map[digest.Digest]*graphdriver.DriverWithDifferOutput),
	}
	dest.Compat = impl.AddCompat(dest)
	return dest, nil
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

func (s *storageImageDestination) computeNextBlobCacheFile() string {
	return filepath.Join(s.directory, fmt.Sprintf("%d", atomic.AddInt32(&s.nextTempFileID, 1)))
}

// PutBlobWithOptions writes contents of stream and returns data representing the result.
// inputInfo.Digest can be optionally provided if known; if provided, and stream is read to the end without error, the digest MUST match the stream contents.
// inputInfo.Size is the expected length of stream, if known.
// inputInfo.MediaType describes the blob format, if known.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (s *storageImageDestination) PutBlobWithOptions(ctx context.Context, stream io.Reader, blobinfo types.BlobInfo, options private.PutBlobOptions) (private.UploadedBlob, error) {
	info, err := s.putBlobToPendingFile(stream, blobinfo, &options)
	if err != nil {
		return info, err
	}

	if options.IsConfig || options.LayerIndex == nil {
		return info, nil
	}

	return info, s.queueOrCommit(*options.LayerIndex, addedLayerInfo{
		digest:     info.Digest,
		emptyLayer: options.EmptyLayer,
	})
}

// putBlobToPendingFile implements ImageDestination.PutBlobWithOptions, storing stream into an on-disk file.
// The caller must arrange the blob to be eventually committed using s.commitLayer().
func (s *storageImageDestination) putBlobToPendingFile(stream io.Reader, blobinfo types.BlobInfo, options *private.PutBlobOptions) (private.UploadedBlob, error) {
	// Stores a layer or data blob in our temporary directory, checking that any information
	// in the blobinfo matches the incoming data.
	if blobinfo.Digest != "" {
		if err := blobinfo.Digest.Validate(); err != nil {
			return private.UploadedBlob{}, fmt.Errorf("invalid digest %#v: %w", blobinfo.Digest.String(), err)
		}
	}

	// Set up to digest the blob if necessary, and count its size while saving it to a file.
	filename := s.computeNextBlobCacheFile()
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return private.UploadedBlob{}, fmt.Errorf("creating temporary file %q: %w", filename, err)
	}
	defer file.Close()
	counter := ioutils.NewWriteCounter(file)
	stream = io.TeeReader(stream, counter)
	digester, stream := putblobdigest.DigestIfUnknown(stream, blobinfo)
	decompressed, err := archive.DecompressStream(stream)
	if err != nil {
		return private.UploadedBlob{}, fmt.Errorf("setting up to decompress blob: %w", err)
	}

	diffID := digest.Canonical.Digester()
	// Copy the data to the file.
	// TODO: This can take quite some time, and should ideally be cancellable using context.Context.
	_, err = io.Copy(diffID.Hash(), decompressed)
	decompressed.Close()
	if err != nil {
		return private.UploadedBlob{}, fmt.Errorf("storing blob to file %q: %w", filename, err)
	}

	// Determine blob properties, and fail if information that we were given about the blob
	// is known to be incorrect.
	blobDigest := digester.Digest()
	blobSize := blobinfo.Size
	if blobSize < 0 {
		blobSize = counter.Count
	} else if blobinfo.Size != counter.Count {
		return private.UploadedBlob{}, ErrBlobSizeMismatch
	}

	// Record information about the blob.
	s.lock.Lock()
	s.blobDiffIDs[blobDigest] = diffID.Digest()
	s.fileSizes[blobDigest] = counter.Count
	s.filenames[blobDigest] = filename
	s.lock.Unlock()
	// This is safe because we have just computed diffID, and blobDigest was either computed
	// by us, or validated by the caller (usually copy.digestingReader).
	options.Cache.RecordDigestUncompressedPair(blobDigest, diffID.Digest())
	return private.UploadedBlob{
		Digest: blobDigest,
		Size:   blobSize,
	}, nil
}

type zstdFetcher struct {
	chunkAccessor private.BlobChunkAccessor
	ctx           context.Context
	blobInfo      types.BlobInfo
}

// GetBlobAt converts from chunked.GetBlobAt to BlobChunkAccessor.GetBlobAt.
func (f *zstdFetcher) GetBlobAt(chunks []chunked.ImageSourceChunk) (chan io.ReadCloser, chan error, error) {
	newChunks := make([]private.ImageSourceChunk, 0, len(chunks))
	for _, v := range chunks {
		i := private.ImageSourceChunk{
			Offset: v.Offset,
			Length: v.Length,
		}
		newChunks = append(newChunks, i)
	}
	rc, errs, err := f.chunkAccessor.GetBlobAt(f.ctx, f.blobInfo, newChunks)
	if _, ok := err.(private.BadPartialRequestError); ok {
		err = chunked.ErrBadRequest{}
	}
	return rc, errs, err

}

// PutBlobPartial attempts to create a blob using the data that is already present
// at the destination. chunkAccessor is accessed in a non-sequential way to retrieve the missing chunks.
// It is available only if SupportsPutBlobPartial().
// Even if SupportsPutBlobPartial() returns true, the call can fail, in which case the caller
// should fall back to PutBlobWithOptions.
func (s *storageImageDestination) PutBlobPartial(ctx context.Context, chunkAccessor private.BlobChunkAccessor, srcInfo types.BlobInfo, cache blobinfocache.BlobInfoCache2) (private.UploadedBlob, error) {
	fetcher := zstdFetcher{
		chunkAccessor: chunkAccessor,
		ctx:           ctx,
		blobInfo:      srcInfo,
	}

	differ, err := chunked.GetDiffer(ctx, s.imageRef.transport.store, srcInfo.Size, srcInfo.Annotations, &fetcher)
	if err != nil {
		return private.UploadedBlob{}, err
	}

	out, err := s.imageRef.transport.store.ApplyDiffWithDiffer("", nil, differ)
	if err != nil {
		return private.UploadedBlob{}, err
	}

	blobDigest := srcInfo.Digest

	s.lock.Lock()
	s.blobDiffIDs[blobDigest] = blobDigest
	s.fileSizes[blobDigest] = 0
	s.filenames[blobDigest] = ""
	s.diffOutputs[blobDigest] = out
	s.lock.Unlock()

	return private.UploadedBlob{
		Digest: blobDigest,
		Size:   srcInfo.Size,
	}, nil
}

// TryReusingBlobWithOptions checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If the blob has been successfully reused, returns (true, info, nil).
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
func (s *storageImageDestination) TryReusingBlobWithOptions(ctx context.Context, blobinfo types.BlobInfo, options private.TryReusingBlobOptions) (bool, private.ReusedBlob, error) {
	reused, info, err := s.tryReusingBlobAsPending(blobinfo.Digest, blobinfo.Size, &options)
	if err != nil || !reused || options.LayerIndex == nil {
		return reused, info, err
	}

	return reused, info, s.queueOrCommit(*options.LayerIndex, addedLayerInfo{
		digest:     info.Digest,
		emptyLayer: options.EmptyLayer,
	})
}

// tryReusingBlobAsPending implements TryReusingBlobWithOptions for (digest, size or -1), filling s.blobDiffIDs and other metadata.
// The caller must arrange the blob to be eventually committed using s.commitLayer().
func (s *storageImageDestination) tryReusingBlobAsPending(digest digest.Digest, size int64, options *private.TryReusingBlobOptions) (bool, private.ReusedBlob, error) {
	// lock the entire method as it executes fairly quickly
	s.lock.Lock()
	defer s.lock.Unlock()

	if options.SrcRef != nil {
		// Check if we have the layer in the underlying additional layer store.
		aLayer, err := s.imageRef.transport.store.LookupAdditionalLayer(digest, options.SrcRef.String())
		if err != nil && !errors.Is(err, storage.ErrLayerUnknown) {
			return false, private.ReusedBlob{}, fmt.Errorf(`looking for compressed layers with digest %q and labels: %w`, digest, err)
		} else if err == nil {
			// Record the uncompressed value so that we can use it to calculate layer IDs.
			s.blobDiffIDs[digest] = aLayer.UncompressedDigest()
			s.blobAdditionalLayer[digest] = aLayer
			return true, private.ReusedBlob{
				Digest: digest,
				Size:   aLayer.CompressedSize(),
			}, nil
		}
	}

	if digest == "" {
		return false, private.ReusedBlob{}, errors.New(`Can not check for a blob with unknown digest`)
	}
	if err := digest.Validate(); err != nil {
		return false, private.ReusedBlob{}, fmt.Errorf("Can not check for a blob with invalid digest: %w", err)
	}

	// Check if we've already cached it in a file.
	if size, ok := s.fileSizes[digest]; ok {
		return true, private.ReusedBlob{
			Digest: digest,
			Size:   size,
		}, nil
	}

	// Check if we have a wasn't-compressed layer in storage that's based on that blob.
	layers, err := s.imageRef.transport.store.LayersByUncompressedDigest(digest)
	if err != nil && !errors.Is(err, storage.ErrLayerUnknown) {
		return false, private.ReusedBlob{}, fmt.Errorf(`looking for layers with digest %q: %w`, digest, err)
	}
	if len(layers) > 0 {
		// Save this for completeness.
		s.blobDiffIDs[digest] = layers[0].UncompressedDigest
		return true, private.ReusedBlob{
			Digest: digest,
			Size:   layers[0].UncompressedSize,
		}, nil
	}

	// Check if we have a was-compressed layer in storage that's based on that blob.
	layers, err = s.imageRef.transport.store.LayersByCompressedDigest(digest)
	if err != nil && !errors.Is(err, storage.ErrLayerUnknown) {
		return false, private.ReusedBlob{}, fmt.Errorf(`looking for compressed layers with digest %q: %w`, digest, err)
	}
	if len(layers) > 0 {
		// Record the uncompressed value so that we can use it to calculate layer IDs.
		s.blobDiffIDs[digest] = layers[0].UncompressedDigest
		return true, private.ReusedBlob{
			Digest: digest,
			Size:   layers[0].CompressedSize,
		}, nil
	}

	// Does the blob correspond to a known DiffID which we already have available?
	// Because we must return the size, which is unknown for unavailable compressed blobs, the returned BlobInfo refers to the
	// uncompressed layer, and that can happen only if options.CanSubstitute, or if the incoming manifest already specifies the size.
	if options.CanSubstitute || size != -1 {
		if uncompressedDigest := options.Cache.UncompressedDigest(digest); uncompressedDigest != "" && uncompressedDigest != digest {
			layers, err := s.imageRef.transport.store.LayersByUncompressedDigest(uncompressedDigest)
			if err != nil && !errors.Is(err, storage.ErrLayerUnknown) {
				return false, private.ReusedBlob{}, fmt.Errorf(`looking for layers with digest %q: %w`, uncompressedDigest, err)
			}
			if len(layers) > 0 {
				if size != -1 {
					s.blobDiffIDs[digest] = layers[0].UncompressedDigest
					return true, private.ReusedBlob{
						Digest: digest,
						Size:   size,
					}, nil
				}
				if !options.CanSubstitute {
					return false, private.ReusedBlob{}, fmt.Errorf("Internal error: options.CanSubstitute was expected to be true for blob with digest %s", digest)
				}
				s.blobDiffIDs[uncompressedDigest] = layers[0].UncompressedDigest
				return true, private.ReusedBlob{
					Digest: uncompressedDigest,
					Size:   layers[0].UncompressedSize,
				}, nil
			}
		}
	}

	// Nope, we don't have it.
	return false, private.ReusedBlob{}, nil
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
		return nil, errors.New(`no digest supplied when reading blob`)
	}
	if err := info.Digest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid digest supplied when reading blob: %w", err)
	}
	// Assume it's a file, since we're only calling this from a place that expects to read files.
	if filename, ok := s.filenames[info.Digest]; ok {
		contents, err2 := os.ReadFile(filename)
		if err2 != nil {
			return nil, fmt.Errorf(`reading blob from file %q: %w`, filename, err2)
		}
		return contents, nil
	}
	// If it's not a file, it's a bug, because we're not expecting to be asked for a layer.
	return nil, errors.New("blob not found")
}

// queueOrCommit queues the specified layer to be committed to the storage.
// If no other goroutine is already committing layers, the layer and all
// subsequent layers (if already queued) will be committed to the storage.
func (s *storageImageDestination) queueOrCommit(index int, info addedLayerInfo) error {
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
	s.indexToAddedLayerInfo[index] = info

	// We're still waiting for at least one previous/parent layer to be
	// committed, so there's nothing to do.
	if index != s.currentIndex {
		s.lock.Unlock()
		return nil
	}

	for {
		info, ok := s.indexToAddedLayerInfo[index]
		if !ok {
			break
		}
		s.lock.Unlock()
		// Note: commitLayer locks on-demand.
		if err := s.commitLayer(index, info, -1); err != nil {
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

// commitLayer commits the specified layer with the given index to the storage.
// size can usually be -1; it can be provided if the layer is not known to be already present in blobDiffIDs.
//
// Note that the previous layer is expected to already be committed.
//
// Caution: this function must be called without holding `s.lock`.  Callers
// must guarantee that, at any given time, at most one goroutine may execute
// `commitLayer()`.
func (s *storageImageDestination) commitLayer(index int, info addedLayerInfo, size int64) error {
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
	if info.emptyLayer {
		s.indexToStorageID[index] = &lastLayer
		return nil
	}

	// Check if there's already a layer with the ID that we'd give to the result of applying
	// this layer blob to its parent, if it has one, or the blob's hex value otherwise.
	s.lock.Lock()
	diffID, haveDiffID := s.blobDiffIDs[info.digest]
	s.lock.Unlock()
	if !haveDiffID {
		// Check if it's elsewhere and the caller just forgot to pass it to us in a PutBlob(),
		// or to even check if we had it.
		// Use none.NoCache to avoid a repeated DiffID lookup in the BlobInfoCache; a caller
		// that relies on using a blob digest that has never been seen by the store had better call
		// TryReusingBlob; not calling PutBlob already violates the documented API, so there’s only
		// so far we are going to accommodate that (if we should be doing that at all).
		logrus.Debugf("looking for diffID for blob %+v", info.digest)
		// Use tryReusingBlobAsPending, not the top-level TryReusingBlobWithOptions, to prevent recursion via queueOrCommit.
		has, _, err := s.tryReusingBlobAsPending(info.digest, size, &private.TryReusingBlobOptions{
			Cache:         none.NoCache,
			CanSubstitute: false,
		})
		if err != nil {
			return fmt.Errorf("checking for a layer based on blob %q: %w", info.digest.String(), err)
		}
		if !has {
			return fmt.Errorf("error determining uncompressed digest for blob %q", info.digest.String())
		}
		diffID, haveDiffID = s.blobDiffIDs[info.digest]
		if !haveDiffID {
			return fmt.Errorf("we have blob %q, but don't know its uncompressed digest", info.digest.String())
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
	diffOutput, ok := s.diffOutputs[info.digest]
	s.lock.Unlock()
	if ok {
		layer, err := s.imageRef.transport.store.CreateLayer(id, lastLayer, nil, "", false, nil)
		if err != nil {
			return err
		}

		// FIXME: what to do with the uncompressed digest?
		diffOutput.UncompressedDigest = info.digest

		if err := s.imageRef.transport.store.ApplyDiffFromStagingDirectory(layer.ID, diffOutput.Target, diffOutput, nil); err != nil {
			_ = s.imageRef.transport.store.Delete(layer.ID)
			return err
		}

		s.indexToStorageID[index] = &layer.ID
		return nil
	}

	s.lock.Lock()
	al, ok := s.blobAdditionalLayer[info.digest]
	s.lock.Unlock()
	if ok {
		layer, err := al.PutAs(id, lastLayer, nil)
		if err != nil && !errors.Is(err, storage.ErrDuplicateID) {
			return fmt.Errorf("failed to put layer from digest and labels: %w", err)
		}
		lastLayer = layer.ID
		s.indexToStorageID[index] = &lastLayer
		return nil
	}

	// Check if we previously cached a file with that blob's contents.  If we didn't,
	// then we need to read the desired contents from a layer.
	s.lock.Lock()
	filename, ok := s.filenames[info.digest]
	s.lock.Unlock()
	if !ok {
		// Try to find the layer with contents matching that blobsum.
		layer := ""
		layers, err2 := s.imageRef.transport.store.LayersByUncompressedDigest(diffID)
		if err2 == nil && len(layers) > 0 {
			layer = layers[0].ID
		} else {
			layers, err2 = s.imageRef.transport.store.LayersByCompressedDigest(info.digest)
			if err2 == nil && len(layers) > 0 {
				layer = layers[0].ID
			}
		}
		if layer == "" {
			return fmt.Errorf("locating layer for blob %q: %w", info.digest, err2)
		}
		// Read the layer's contents.
		noCompression := archive.Uncompressed
		diffOptions := &storage.DiffOptions{
			Compression: &noCompression,
		}
		diff, err2 := s.imageRef.transport.store.Diff("", layer, diffOptions)
		if err2 != nil {
			return fmt.Errorf("reading layer %q for blob %q: %w", layer, info.digest, err2)
		}
		// Copy the layer diff to a file.  Diff() takes a lock that it holds
		// until the ReadCloser that it returns is closed, and PutLayer() wants
		// the same lock, so the diff can't just be directly streamed from one
		// to the other.
		filename = s.computeNextBlobCacheFile()
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_EXCL, 0600)
		if err != nil {
			diff.Close()
			return fmt.Errorf("creating temporary file %q: %w", filename, err)
		}
		// Copy the data to the file.
		// TODO: This can take quite some time, and should ideally be cancellable using
		// ctx.Done().
		_, err = io.Copy(file, diff)
		diff.Close()
		file.Close()
		if err != nil {
			return fmt.Errorf("storing blob to file %q: %w", filename, err)
		}
		// Make sure that we can find this file later, should we need the layer's
		// contents again.
		s.lock.Lock()
		s.filenames[info.digest] = filename
		s.lock.Unlock()
	}
	// Read the cached blob and use it as a diff.
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("opening file %q: %w", filename, err)
	}
	defer file.Close()
	// Build the new layer using the diff, regardless of where it came from.
	// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
	layer, _, err := s.imageRef.transport.store.PutLayer(id, lastLayer, nil, "", false, &storage.LayerOptions{
		OriginalDigest:     info.digest,
		UncompressedDigest: diffID,
	}, file)
	if err != nil && !errors.Is(err, storage.ErrDuplicateID) {
		return fmt.Errorf("adding layer with blob %q: %w", info.digest, err)
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
		return fmt.Errorf("retrieving top-level manifest: %w", err)
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
		return fmt.Errorf("parsing manifest: %w", err)
	}
	layerBlobs := man.LayerInfos()

	// Extract, commit, or find the layers.
	for i, blob := range layerBlobs {
		if err := s.commitLayer(i, addedLayerInfo{
			digest:     blob.Digest,
			emptyLayer: blob.EmptyLayer,
		}, blob.Size); err != nil {
			return err
		}
	}
	var lastLayer string
	if len(layerBlobs) > 0 { // Can happen when using caches
		prev := s.indexToStorageID[len(layerBlobs)-1]
		if prev == nil {
			return fmt.Errorf("Internal error: StorageImageDestination.Commit(): previous layer %d hasn't been committed (lastLayer == nil)", len(layerBlobs)-1)
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
		if !errors.Is(err, storage.ErrDuplicateID) {
			logrus.Debugf("error creating image: %q", err)
			return fmt.Errorf("creating image %q: %w", intendedID, err)
		}
		img, err = s.imageRef.transport.store.Image(intendedID)
		if err != nil {
			return fmt.Errorf("reading image %q: %w", intendedID, err)
		}
		if img.TopLayer != lastLayer {
			logrus.Debugf("error creating image: image with ID %q exists, but uses different layers", intendedID)
			return fmt.Errorf("image with ID %q already exists, but uses a different top layer: %w", intendedID, storage.ErrDuplicateID)
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
	dataBlobs := set.New[digest.Digest]()
	for blob := range s.filenames {
		dataBlobs.Add(blob)
	}
	for _, layerBlob := range layerBlobs {
		dataBlobs.Delete(layerBlob.Digest)
	}
	for _, blob := range dataBlobs.Values() {
		v, err := os.ReadFile(s.filenames[blob])
		if err != nil {
			return fmt.Errorf("copying non-layer blob %q to image: %w", blob, err)
		}
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, blob.String(), v, manifest.Digest); err != nil {
			logrus.Debugf("error saving big data %q for image %q: %v", blob.String(), img.ID, err)
			return fmt.Errorf("saving big data %q for image %q: %w", blob.String(), img.ID, err)
		}
	}
	// Save the unparsedToplevel's manifest if it differs from the per-platform one, which is saved below.
	if len(toplevelManifest) != 0 && !bytes.Equal(toplevelManifest, s.manifest) {
		manifestDigest, err := manifest.Digest(toplevelManifest)
		if err != nil {
			return fmt.Errorf("digesting top-level manifest: %w", err)
		}
		key := manifestBigDataKey(manifestDigest)
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, toplevelManifest, manifest.Digest); err != nil {
			logrus.Debugf("error saving top-level manifest for image %q: %v", img.ID, err)
			return fmt.Errorf("saving top-level manifest for image %q: %w", img.ID, err)
		}
	}
	// Save the image's manifest.  Allow looking it up by digest by using the key convention defined by the Store.
	// Record the manifest twice: using a digest-specific key to allow references to that specific digest instance,
	// and using storage.ImageDigestBigDataKey for future users that don’t specify any digest and for compatibility with older readers.
	key := manifestBigDataKey(s.manifestDigest)
	if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, s.manifest, manifest.Digest); err != nil {
		logrus.Debugf("error saving manifest for image %q: %v", img.ID, err)
		return fmt.Errorf("saving manifest for image %q: %w", img.ID, err)
	}
	key = storage.ImageDigestBigDataKey
	if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, s.manifest, manifest.Digest); err != nil {
		logrus.Debugf("error saving manifest for image %q: %v", img.ID, err)
		return fmt.Errorf("saving manifest for image %q: %w", img.ID, err)
	}
	// Save the signatures, if we have any.
	if len(s.signatures) > 0 {
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, "signatures", s.signatures, manifest.Digest); err != nil {
			logrus.Debugf("error saving signatures for image %q: %v", img.ID, err)
			return fmt.Errorf("saving signatures for image %q: %w", img.ID, err)
		}
	}
	for instanceDigest, signatures := range s.signatureses {
		key := signatureBigDataKey(instanceDigest)
		if err := s.imageRef.transport.store.SetImageBigData(img.ID, key, signatures, manifest.Digest); err != nil {
			logrus.Debugf("error saving signatures for image %q: %v", img.ID, err)
			return fmt.Errorf("saving signatures for image %q: %w", img.ID, err)
		}
	}
	// Save our metadata.
	metadata, err := json.Marshal(s)
	if err != nil {
		logrus.Debugf("error encoding metadata for image %q: %v", img.ID, err)
		return fmt.Errorf("encoding metadata for image %q: %w", img.ID, err)
	}
	if len(metadata) != 0 {
		if err = s.imageRef.transport.store.SetMetadata(img.ID, string(metadata)); err != nil {
			logrus.Debugf("error saving metadata for image %q: %v", img.ID, err)
			return fmt.Errorf("saving metadata for image %q: %w", img.ID, err)
		}
		logrus.Debugf("saved image metadata %q", string(metadata))
	}
	// Adds the reference's name on the image.  We don't need to worry about avoiding duplicate
	// values because AddNames() will deduplicate the list that we pass to it.
	if name := s.imageRef.DockerReference(); name != nil {
		if err := s.imageRef.transport.store.AddNames(img.ID, []string{name.String()}); err != nil {
			return fmt.Errorf("adding names %v to image %q: %w", name, img.ID, err)
		}
		logrus.Debugf("added name %q to image %q", name, img.ID)
	}

	commitSucceeded = true
	return nil
}

// PutManifest writes the manifest to the destination.
func (s *storageImageDestination) PutManifest(ctx context.Context, manifestBlob []byte, instanceDigest *digest.Digest) error {
	digest, err := manifest.Digest(manifestBlob)
	if err != nil {
		return err
	}
	s.manifest = slices.Clone(manifestBlob)
	s.manifestDigest = digest
	return nil
}

// PutSignaturesWithFormat writes a set of signatures to the destination.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to write or overwrite the signatures for
// (when the primary manifest is a manifest list); this should always be nil if the primary manifest is not a manifest list.
// MUST be called after PutManifest (signatures may reference manifest contents).
func (s *storageImageDestination) PutSignaturesWithFormat(ctx context.Context, signatures []signature.Signature, instanceDigest *digest.Digest) error {
	sizes := []int{}
	sigblob := []byte{}
	for _, sigWithFormat := range signatures {
		sig, err := signature.Blob(sigWithFormat)
		if err != nil {
			return err
		}
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
