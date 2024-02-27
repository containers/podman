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

	imageRef              storageReference
	directory             string                   // Temporary directory where we store blobs until Commit() time
	nextTempFileID        atomic.Int32             // A counter that we use for computing filenames to assign to blobs
	manifest              []byte                   // Manifest contents, temporary
	manifestDigest        digest.Digest            // Valid if len(manifest) != 0
	untrustedDiffIDValues []digest.Digest          // From config’s RootFS.DiffIDs, valid if not nil
	signatures            []byte                   // Signature contents, temporary
	signatureses          map[digest.Digest][]byte // Instance signature contents, temporary
	metadata              storageImageMetadata     // Metadata contents being built

	// Mapping from layer (by index) to the associated ID in the storage.
	// It's protected *implicitly* since `commitLayer()`, at any given
	// time, can only be executed by *one* goroutine.  Please refer to
	// `queueOrCommit()` for further details on how the single-caller
	// guarantee is implemented.
	indexToStorageID map[int]string

	// A storage destination may be used concurrently, due to HasThreadSafePutBlob.
	lock          sync.Mutex // Protects lockProtected
	lockProtected storageImageDestinationLockProtected
}

// storageImageDestinationLockProtected contains storageImageDestination data which might be
// accessed concurrently, due to HasThreadSafePutBlob.
// _During the concurrent TryReusingBlob/PutBlob/* calls_ (but not necessarily during the final Commit)
// uses must hold storageImageDestination.lock.
type storageImageDestinationLockProtected struct {
	currentIndex          int                    // The index of the layer to be committed (i.e., lower indices have already been committed)
	indexToAddedLayerInfo map[int]addedLayerInfo // Mapping from layer (by index) to blob to add to the image

	// In general, a layer is identified either by (compressed) digest, or by TOC digest.
	// When creating a layer, the c/storage layer metadata and image IDs must _only_ be based on trusted values
	// we have computed ourselves. (Layer reuse can then look up against such trusted values, but it might not
	// recompute those values for incomding layers — the point of the reuse is that we don’t need to consume the incoming layer.)

	// Layer identification: For a layer, at least one of indexToTOCDigest and blobDiffIDs must be available before commitLayer is called.
	// The presence of an indexToTOCDigest is what decides how the layer is identified, i.e. which fields must be trusted.
	blobDiffIDs      map[digest.Digest]digest.Digest // Mapping from layer blobsums to their corresponding DiffIDs
	indexToTOCDigest map[int]digest.Digest           // Mapping from layer index to a TOC Digest, IFF the layer was created/found/reused by TOC digest

	// Layer data: Before commitLayer is called, either at least one of (diffOutputs, blobAdditionalLayer, filenames)
	// should be available; or indexToTOCDigest/blobDiffIDs should be enough to locate an existing c/storage layer.
	// They are looked up in the order they are mentioned above.
	diffOutputs         map[int]*graphdriver.DriverWithDifferOutput // Mapping from layer index to a partially-pulled layer intermediate data
	blobAdditionalLayer map[digest.Digest]storage.AdditionalLayer   // Mapping from layer blobsums to their corresponding additional layer
	// Mapping from layer blobsums to names of files we used to hold them. If set, fileSizes and blobDiffIDs must also be set.
	filenames map[digest.Digest]string
	// Mapping from layer blobsums to their sizes. If set, filenames and blobDiffIDs must also be set.
	fileSizes map[digest.Digest]int64
}

// addedLayerInfo records data about a layer to use in this image.
type addedLayerInfo struct {
	digest     digest.Digest // Mandatory, the digest of the layer.
	emptyLayer bool          // The layer is an “empty”/“throwaway” one, and may or may not be physically represented in various transport / storage systems.  false if the manifest type does not have the concept.
}

// newImageDestination sets us up to write a new image, caching blobs in a temporary directory until
// it's time to Commit() the image
func newImageDestination(sys *types.SystemContext, imageRef storageReference) (*storageImageDestination, error) {
	directory, err := tmpdir.MkDirBigFileTemp(sys, "storage")
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

		imageRef:     imageRef,
		directory:    directory,
		signatureses: make(map[digest.Digest][]byte),
		metadata: storageImageMetadata{
			SignatureSizes:  []int{},
			SignaturesSizes: make(map[digest.Digest][]int),
		},
		indexToStorageID: make(map[int]string),
		lockProtected: storageImageDestinationLockProtected{
			indexToAddedLayerInfo: make(map[int]addedLayerInfo),
			blobDiffIDs:           make(map[digest.Digest]digest.Digest),
			indexToTOCDigest:      make(map[int]digest.Digest),
			diffOutputs:           make(map[int]*graphdriver.DriverWithDifferOutput),
			blobAdditionalLayer:   make(map[digest.Digest]storage.AdditionalLayer),
			filenames:             make(map[digest.Digest]string),
			fileSizes:             make(map[digest.Digest]int64),
		},
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
	// This is outside of the scope of HasThreadSafePutBlob, so we don’t need to hold s.lock.
	for _, al := range s.lockProtected.blobAdditionalLayer {
		al.Release()
	}
	for _, v := range s.lockProtected.diffOutputs {
		if v.Target != "" {
			_ = s.imageRef.transport.store.CleanupStagedLayer(v)
		}
	}
	return os.RemoveAll(s.directory)
}

func (s *storageImageDestination) computeNextBlobCacheFile() string {
	return filepath.Join(s.directory, fmt.Sprintf("%d", s.nextTempFileID.Add(1)))
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
	s.lockProtected.blobDiffIDs[blobDigest] = diffID.Digest()
	s.lockProtected.fileSizes[blobDigest] = counter.Count
	s.lockProtected.filenames[blobDigest] = filename
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
func (s *storageImageDestination) PutBlobPartial(ctx context.Context, chunkAccessor private.BlobChunkAccessor, srcInfo types.BlobInfo, options private.PutBlobPartialOptions) (private.UploadedBlob, error) {
	fetcher := zstdFetcher{
		chunkAccessor: chunkAccessor,
		ctx:           ctx,
		blobInfo:      srcInfo,
	}

	differ, err := chunked.GetDiffer(ctx, s.imageRef.transport.store, srcInfo.Digest, srcInfo.Size, srcInfo.Annotations, &fetcher)
	if err != nil {
		return private.UploadedBlob{}, err
	}

	out, err := s.imageRef.transport.store.ApplyDiffWithDiffer("", nil, differ)
	if err != nil {
		return private.UploadedBlob{}, err
	}

	if out.TOCDigest == "" && out.UncompressedDigest == "" {
		return private.UploadedBlob{}, errors.New("internal error: ApplyDiffWithDiffer succeeded with neither TOCDigest nor UncompressedDigest set")
	}

	blobDigest := srcInfo.Digest

	s.lock.Lock()
	if out.UncompressedDigest != "" {
		// The computation of UncompressedDigest means the whole layer has been consumed; while doing that, chunked.GetDiffer is
		// responsible for ensuring blobDigest has been validated.
		s.lockProtected.blobDiffIDs[blobDigest] = out.UncompressedDigest
	} else {
		// Don’t identify layers by TOC if UncompressedDigest is available.
		// - Using UncompressedDigest allows image reuse with non-partially-pulled layers
		// - If UncompressedDigest has been computed, that means the layer was read completely, and the TOC has been created from scratch.
		//   That TOC is quite unlikely to match with any other TOC value.
		s.lockProtected.indexToTOCDigest[options.LayerIndex] = out.TOCDigest
	}
	s.lockProtected.diffOutputs[options.LayerIndex] = out
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
	if !impl.OriginalCandidateMatchesTryReusingBlobOptions(options) {
		return false, private.ReusedBlob{}, nil
	}
	reused, info, err := s.tryReusingBlobAsPending(blobinfo.Digest, blobinfo.Size, &options)
	if err != nil || !reused || options.LayerIndex == nil {
		return reused, info, err
	}

	return reused, info, s.queueOrCommit(*options.LayerIndex, addedLayerInfo{
		digest:     info.Digest,
		emptyLayer: options.EmptyLayer,
	})
}

// tryReusingBlobAsPending implements TryReusingBlobWithOptions for (blobDigest, size or -1), filling s.blobDiffIDs and other metadata.
// The caller must arrange the blob to be eventually committed using s.commitLayer().
func (s *storageImageDestination) tryReusingBlobAsPending(blobDigest digest.Digest, size int64, options *private.TryReusingBlobOptions) (bool, private.ReusedBlob, error) {
	// lock the entire method as it executes fairly quickly
	s.lock.Lock()
	defer s.lock.Unlock()

	if options.SrcRef != nil {
		// Check if we have the layer in the underlying additional layer store.
		aLayer, err := s.imageRef.transport.store.LookupAdditionalLayer(blobDigest, options.SrcRef.String())
		if err != nil && !errors.Is(err, storage.ErrLayerUnknown) {
			return false, private.ReusedBlob{}, fmt.Errorf(`looking for compressed layers with digest %q and labels: %w`, blobDigest, err)
		} else if err == nil {
			s.lockProtected.blobDiffIDs[blobDigest] = aLayer.UncompressedDigest()
			s.lockProtected.blobAdditionalLayer[blobDigest] = aLayer
			return true, private.ReusedBlob{
				Digest: blobDigest,
				Size:   aLayer.CompressedSize(),
			}, nil
		}
	}

	if blobDigest == "" {
		return false, private.ReusedBlob{}, errors.New(`Can not check for a blob with unknown digest`)
	}
	if err := blobDigest.Validate(); err != nil {
		return false, private.ReusedBlob{}, fmt.Errorf("Can not check for a blob with invalid digest: %w", err)
	}
	if options.TOCDigest != "" {
		if err := options.TOCDigest.Validate(); err != nil {
			return false, private.ReusedBlob{}, fmt.Errorf("Can not check for a blob with invalid digest: %w", err)
		}
	}

	// Check if we have a wasn't-compressed layer in storage that's based on that blob.

	// Check if we've already cached it in a file.
	if size, ok := s.lockProtected.fileSizes[blobDigest]; ok {
		// s.lockProtected.blobDiffIDs is set either by putBlobToPendingFile or in createNewLayer when creating the
		// filenames/fileSizes entry.
		return true, private.ReusedBlob{
			Digest: blobDigest,
			Size:   size,
		}, nil
	}

	layers, err := s.imageRef.transport.store.LayersByUncompressedDigest(blobDigest)
	if err != nil && !errors.Is(err, storage.ErrLayerUnknown) {
		return false, private.ReusedBlob{}, fmt.Errorf(`looking for layers with digest %q: %w`, blobDigest, err)
	}
	if len(layers) > 0 {
		s.lockProtected.blobDiffIDs[blobDigest] = blobDigest
		return true, private.ReusedBlob{
			Digest: blobDigest,
			Size:   layers[0].UncompressedSize,
		}, nil
	}

	// Check if we have a was-compressed layer in storage that's based on that blob.
	layers, err = s.imageRef.transport.store.LayersByCompressedDigest(blobDigest)
	if err != nil && !errors.Is(err, storage.ErrLayerUnknown) {
		return false, private.ReusedBlob{}, fmt.Errorf(`looking for compressed layers with digest %q: %w`, blobDigest, err)
	}
	if len(layers) > 0 {
		// LayersByCompressedDigest only finds layers which were created from a full layer blob, and extracting that
		// always sets UncompressedDigest.
		diffID := layers[0].UncompressedDigest
		if diffID == "" {
			return false, private.ReusedBlob{}, fmt.Errorf("internal error: compressed layer %q (for compressed digest %q) does not have an uncompressed digest", layers[0].ID, blobDigest.String())
		}
		s.lockProtected.blobDiffIDs[blobDigest] = diffID
		return true, private.ReusedBlob{
			Digest: blobDigest,
			Size:   layers[0].CompressedSize,
		}, nil
	}

	// Does the blob correspond to a known DiffID which we already have available?
	// Because we must return the size, which is unknown for unavailable compressed blobs, the returned BlobInfo refers to the
	// uncompressed layer, and that can happen only if options.CanSubstitute, or if the incoming manifest already specifies the size.
	if options.CanSubstitute || size != -1 {
		if uncompressedDigest := options.Cache.UncompressedDigest(blobDigest); uncompressedDigest != "" && uncompressedDigest != blobDigest {
			layers, err := s.imageRef.transport.store.LayersByUncompressedDigest(uncompressedDigest)
			if err != nil && !errors.Is(err, storage.ErrLayerUnknown) {
				return false, private.ReusedBlob{}, fmt.Errorf(`looking for layers with digest %q: %w`, uncompressedDigest, err)
			}
			if len(layers) > 0 {
				if size != -1 {
					s.lockProtected.blobDiffIDs[blobDigest] = uncompressedDigest
					return true, private.ReusedBlob{
						Digest: blobDigest,
						Size:   size,
					}, nil
				}
				if !options.CanSubstitute {
					return false, private.ReusedBlob{}, fmt.Errorf("Internal error: options.CanSubstitute was expected to be true for blob with digest %s", blobDigest)
				}
				s.lockProtected.blobDiffIDs[uncompressedDigest] = uncompressedDigest
				return true, private.ReusedBlob{
					Digest: uncompressedDigest,
					Size:   layers[0].UncompressedSize,
				}, nil
			}
		}
	}

	if options.TOCDigest != "" && options.LayerIndex != nil {
		// Check if we have a chunked layer in storage with the same TOC digest.
		layers, err := s.imageRef.transport.store.LayersByTOCDigest(options.TOCDigest)

		if err != nil && !errors.Is(err, storage.ErrLayerUnknown) {
			return false, private.ReusedBlob{}, fmt.Errorf(`looking for layers with TOC digest %q: %w`, options.TOCDigest, err)
		}
		if len(layers) > 0 {
			if size != -1 {
				s.lockProtected.indexToTOCDigest[*options.LayerIndex] = options.TOCDigest
				return true, private.ReusedBlob{
					Digest:             blobDigest,
					Size:               size,
					MatchedByTOCDigest: true,
				}, nil
			} else if options.CanSubstitute && layers[0].UncompressedDigest != "" {
				s.lockProtected.indexToTOCDigest[*options.LayerIndex] = options.TOCDigest
				return true, private.ReusedBlob{
					Digest:             layers[0].UncompressedDigest,
					Size:               layers[0].UncompressedSize,
					MatchedByTOCDigest: true,
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
	// This is outside of the scope of HasThreadSafePutBlob, so we don’t need to hold s.lock.

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
			diffID, ok := s.lockProtected.blobDiffIDs[blobSum]
			if !ok {
				// this can, in principle, legitimately happen when a layer is reused by TOC.
				logrus.Infof("error looking up diffID for layer %q", blobSum.String())
				return ""
			}
			diffIDs = append([]digest.Digest{diffID}, diffIDs...)
		}
	case *manifest.Schema2, *manifest.OCI1:
		// We know the ID calculation doesn't actually use the diffIDs, so we don't need to populate
		// the diffID list.
	default:
		return ""
	}

	// We want to use the same ID for “the same” images, but without risking unwanted sharing / malicious image corruption.
	//
	// Traditionally that means the same ~config digest, as computed by m.ImageID;
	// but if we pull a layer by TOC, we verify the layer against neither the (compressed) blob digest in the manifest,
	// nor against the config’s RootFS.DiffIDs. We don’t really want to do either, to allow partial layer pulls where we never see
	// most of the data.
	//
	// So, if a layer is pulled by TOC (and we do validate against the TOC), the fact that we used the TOC, and the value of the TOC,
	// must enter into the image ID computation.
	// But for images where no TOC was used, continue to use IDs computed the traditional way, to maximize image reuse on upgrades,
	// and to introduce the changed behavior only when partial pulls are used.
	//
	// Note that it’s not 100% guaranteed that an image pulled by TOC uses an OCI manifest; consider
	// (skopeo copy --format v2s2 docker://…/zstd-chunked-image containers-storage:… ). So this is not happening only in the OCI case above.
	ordinaryImageID, err := m.ImageID(diffIDs)
	if err != nil {
		return ""
	}
	tocIDInput := ""
	hasLayerPulledByTOC := false
	for i := range m.LayerInfos() {
		layerValue := ""                                     // An empty string is not a valid digest, so this is unambiguous with the TOC case.
		tocDigest, ok := s.lockProtected.indexToTOCDigest[i] // "" if not a TOC
		if ok {
			hasLayerPulledByTOC = true
			layerValue = tocDigest.String()
		}
		tocIDInput += layerValue + "|" // "|" can not be present in a TOC digest, so this is an unambiguous separator.
	}

	if !hasLayerPulledByTOC {
		return ordinaryImageID
	}
	// ordinaryImageID is a digest of a config, which is a JSON value.
	// To avoid the risk of collisions, start the input with @ so that the input is not a valid JSON.
	tocImageID := digest.FromString("@With TOC:" + tocIDInput).Hex()
	logrus.Debugf("Ordinary storage image ID %s; a layer was looked up by TOC, so using image ID %s", ordinaryImageID, tocImageID)
	return tocImageID
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
	if filename, ok := s.lockProtected.filenames[info.Digest]; ok {
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
	s.lockProtected.indexToAddedLayerInfo[index] = info

	// We're still waiting for at least one previous/parent layer to be
	// committed, so there's nothing to do.
	if index != s.lockProtected.currentIndex {
		s.lock.Unlock()
		return nil
	}

	for {
		info, ok := s.lockProtected.indexToAddedLayerInfo[index]
		if !ok {
			break
		}
		s.lock.Unlock()
		// Note: commitLayer locks on-demand.
		if stopQueue, err := s.commitLayer(index, info, -1); stopQueue || err != nil {
			return err
		}
		s.lock.Lock()
		index++
	}

	// Set the index at the very end to make sure that only one routine
	// enters stage 2).
	s.lockProtected.currentIndex = index
	s.lock.Unlock()
	return nil
}

// singleLayerIDComponent returns a single layer’s the input to computing a layer (chain) ID,
// and an indication whether the input already has the shape of a layer ID.
// It returns ("", false) if the layer is not found at all (which should never happen)
func (s *storageImageDestination) singleLayerIDComponent(layerIndex int, blobDigest digest.Digest) (string, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if d, found := s.lockProtected.indexToTOCDigest[layerIndex]; found {
		return "@TOC=" + d.Hex(), false // "@" is not a valid start of a digest.Digest, so this is unambiguous.
	}

	if d, found := s.lockProtected.blobDiffIDs[blobDigest]; found {
		return d.Hex(), true // This looks like chain IDs, and it uses the traditional value.
	}
	return "", false
}

// commitLayer commits the specified layer with the given index to the storage.
// size can usually be -1; it can be provided if the layer is not known to be already present in blobDiffIDs.
//
// If the layer cannot be committed yet, the function returns (true, nil).
//
// Note that the previous layer is expected to already be committed.
//
// Caution: this function must be called without holding `s.lock`.  Callers
// must guarantee that, at any given time, at most one goroutine may execute
// `commitLayer()`.
func (s *storageImageDestination) commitLayer(index int, info addedLayerInfo, size int64) (bool, error) {
	// Already committed?  Return early.
	if _, alreadyCommitted := s.indexToStorageID[index]; alreadyCommitted {
		return false, nil
	}

	// Start with an empty string or the previous layer ID.  Note that
	// `s.indexToStorageID` can only be accessed by *one* goroutine at any
	// given time. Hence, we don't need to lock accesses.
	var parentLayer string
	if index != 0 {
		prev, ok := s.indexToStorageID[index-1]
		if !ok {
			return false, fmt.Errorf("Internal error: commitLayer called with previous layer %d not committed yet", index-1)
		}
		parentLayer = prev
	}

	// Carry over the previous ID for empty non-base layers.
	if info.emptyLayer {
		s.indexToStorageID[index] = parentLayer
		return false, nil
	}

	// Check if there's already a layer with the ID that we'd give to the result of applying
	// this layer blob to its parent, if it has one, or the blob's hex value otherwise.
	// The layerID refers either to the DiffID or the digest of the TOC.
	layerIDComponent, layerIDComponentStandalone := s.singleLayerIDComponent(index, info.digest)
	if layerIDComponent == "" {
		// Check if it's elsewhere and the caller just forgot to pass it to us in a PutBlob() / TryReusingBlob() / …
		//
		// Use none.NoCache to avoid a repeated DiffID lookup in the BlobInfoCache: a caller
		// that relies on using a blob digest that has never been seen by the store had better call
		// TryReusingBlob; not calling PutBlob already violates the documented API, so there’s only
		// so far we are going to accommodate that (if we should be doing that at all).
		//
		// We are also ignoring lookups by TOC, and other non-trivial situations.
		// Those can only happen using the c/image/internal/private API,
		// so those internal callers should be fixed to follow the API instead of expanding this fallback.
		logrus.Debugf("looking for diffID for blob=%+v", info.digest)

		// Use tryReusingBlobAsPending, not the top-level TryReusingBlobWithOptions, to prevent recursion via queueOrCommit.
		has, _, err := s.tryReusingBlobAsPending(info.digest, size, &private.TryReusingBlobOptions{
			Cache:         none.NoCache,
			CanSubstitute: false,
		})
		if err != nil {
			return false, fmt.Errorf("checking for a layer based on blob %q: %w", info.digest.String(), err)
		}
		if !has {
			return false, fmt.Errorf("error determining uncompressed digest for blob %q", info.digest.String())
		}

		layerIDComponent, layerIDComponentStandalone = s.singleLayerIDComponent(index, info.digest)
		if layerIDComponent == "" {
			return false, fmt.Errorf("we have blob %q, but don't know its layer ID", info.digest.String())
		}
	}

	id := layerIDComponent
	if !layerIDComponentStandalone || parentLayer != "" {
		id = digest.Canonical.FromString(parentLayer + "+" + layerIDComponent).Hex()
	}
	if layer, err2 := s.imageRef.transport.store.Layer(id); layer != nil && err2 == nil {
		// There's already a layer that should have the right contents, just reuse it.
		s.indexToStorageID[index] = layer.ID
		return false, nil
	}

	layer, err := s.createNewLayer(index, info.digest, parentLayer, id)
	if err != nil {
		return false, err
	}
	if layer == nil {
		return true, nil
	}
	s.indexToStorageID[index] = layer.ID
	return false, nil
}

// createNewLayer creates a new layer newLayerID for (index, layerDigest) on top of parentLayer (which may be "").
// If the layer cannot be committed yet, the function returns (nil, nil).
func (s *storageImageDestination) createNewLayer(index int, layerDigest digest.Digest, parentLayer, newLayerID string) (*storage.Layer, error) {
	s.lock.Lock()
	diffOutput, ok := s.lockProtected.diffOutputs[index]
	s.lock.Unlock()
	if ok {
		var untrustedUncompressedDigest digest.Digest
		if diffOutput.UncompressedDigest == "" {
			d, err := s.untrustedLayerDiffID(index)
			if err != nil {
				return nil, err
			}
			if d == "" {
				logrus.Debugf("Skipping commit for layer %q, manifest not yet available", newLayerID)
				return nil, nil
			}
			untrustedUncompressedDigest = d
		}

		flags := make(map[string]interface{})
		if untrustedUncompressedDigest != "" {
			flags[expectedLayerDiffIDFlag] = untrustedUncompressedDigest
			logrus.Debugf("Setting uncompressed digest to %q for layer %q", untrustedUncompressedDigest, newLayerID)
		}

		args := storage.ApplyStagedLayerOptions{
			ID:          newLayerID,
			ParentLayer: parentLayer,

			DiffOutput: diffOutput,
			DiffOptions: &graphdriver.ApplyDiffWithDifferOpts{
				Flags: flags,
			},
		}
		layer, err := s.imageRef.transport.store.ApplyStagedLayer(args)
		if err != nil && !errors.Is(err, storage.ErrDuplicateID) {
			return nil, fmt.Errorf("failed to put layer using a partial pull: %w", err)
		}
		return layer, nil
	}

	s.lock.Lock()
	al, ok := s.lockProtected.blobAdditionalLayer[layerDigest]
	s.lock.Unlock()
	if ok {
		layer, err := al.PutAs(newLayerID, parentLayer, nil)
		if err != nil && !errors.Is(err, storage.ErrDuplicateID) {
			return nil, fmt.Errorf("failed to put layer from digest and labels: %w", err)
		}
		return layer, nil
	}

	// Check if we previously cached a file with that blob's contents.  If we didn't,
	// then we need to read the desired contents from a layer.
	var trustedUncompressedDigest, trustedOriginalDigest digest.Digest // For storage.LayerOptions
	s.lock.Lock()
	tocDigest := s.lockProtected.indexToTOCDigest[index]       // "" if not set
	optionalDiffID := s.lockProtected.blobDiffIDs[layerDigest] // "" if not set
	filename, gotFilename := s.lockProtected.filenames[layerDigest]
	s.lock.Unlock()
	if gotFilename && tocDigest == "" {
		// If tocDigest != "", if we now happen to find a layerDigest match, the newLayerID has already been computed as TOC-based,
		// and we don't know the relationship of the layerDigest and TOC digest.
		// We could recompute newLayerID to be DiffID-based and use the file, but such a within-image layer
		// reuse is expected to be pretty rare; instead, ignore the unexpected file match and proceed to the
		// originally-planned TOC match.

		// Because tocDigest == "", optionaldiffID must have been set; and even if it weren’t, PutLayer will recompute the digest from the stream.
		trustedUncompressedDigest = optionalDiffID
		trustedOriginalDigest = layerDigest // The code setting .filenames[layerDigest] is responsible for the contents matching.
	} else {
		// Try to find the layer with contents matching the data we use.
		var layer *storage.Layer // = nil
		if tocDigest != "" {
			layers, err2 := s.imageRef.transport.store.LayersByTOCDigest(tocDigest)
			if err2 == nil && len(layers) > 0 {
				layer = &layers[0]
			} else {
				return nil, fmt.Errorf("locating layer for TOC digest %q: %w", tocDigest, err2)
			}
		} else {
			// Because tocDigest == "", optionaldiffID must have been set
			layers, err2 := s.imageRef.transport.store.LayersByUncompressedDigest(optionalDiffID)
			if err2 == nil && len(layers) > 0 {
				layer = &layers[0]
			} else {
				layers, err2 = s.imageRef.transport.store.LayersByCompressedDigest(layerDigest)
				if err2 == nil && len(layers) > 0 {
					layer = &layers[0]
				}
			}
			if layer == nil {
				return nil, fmt.Errorf("locating layer for blob %q: %w", layerDigest, err2)
			}
		}
		// Read the layer's contents.
		noCompression := archive.Uncompressed
		diffOptions := &storage.DiffOptions{
			Compression: &noCompression,
		}
		diff, err2 := s.imageRef.transport.store.Diff("", layer.ID, diffOptions)
		if err2 != nil {
			return nil, fmt.Errorf("reading layer %q for blob %q: %w", layer.ID, layerDigest, err2)
		}
		// Copy the layer diff to a file.  Diff() takes a lock that it holds
		// until the ReadCloser that it returns is closed, and PutLayer() wants
		// the same lock, so the diff can't just be directly streamed from one
		// to the other.
		filename = s.computeNextBlobCacheFile()
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_EXCL, 0o600)
		if err != nil {
			diff.Close()
			return nil, fmt.Errorf("creating temporary file %q: %w", filename, err)
		}
		// Copy the data to the file.
		// TODO: This can take quite some time, and should ideally be cancellable using
		// ctx.Done().
		fileSize, err := io.Copy(file, diff)
		diff.Close()
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("storing blob to file %q: %w", filename, err)
		}

		if optionalDiffID == "" && layer.UncompressedDigest != "" {
			optionalDiffID = layer.UncompressedDigest
		}
		// The stream we have is uncompressed, this matches contents of the stream.
		// If tocDigest != "", trustedUncompressedDigest might still be ""; in that case PutLayer will compute the value from the stream.
		trustedUncompressedDigest = optionalDiffID
		// FIXME? trustedOriginalDigest could be set to layerDigest IF tocDigest == "" (otherwise layerDigest is untrusted).
		// But for c/storage to reasonably use it (as a CompressedDigest value), we should also ensure the CompressedSize of the created
		// layer is correct, and the API does not currently make it possible (.CompressedSize is set from the input stream).
		//
		// We can legitimately set storage.LayerOptions.OriginalDigest to "",
		// but that would just result in PutLayer computing the digest of the input stream == optionalDiffID.
		// So, instead, set .OriginalDigest to the value we know already, to avoid that digest computation.
		trustedOriginalDigest = optionalDiffID

		// Allow using the already-collected layer contents without extracting the layer again.
		//
		// This only matches against the uncompressed digest.
		// We don’t have the original compressed data here to trivially set filenames[layerDigest].
		// In particular we can’t achieve the correct Layer.CompressedSize value with the current c/storage API.
		// Within-image layer reuse is probably very rare, for now we prefer to avoid that complexity.
		if trustedUncompressedDigest != "" {
			s.lock.Lock()
			s.lockProtected.blobDiffIDs[trustedUncompressedDigest] = trustedUncompressedDigest
			s.lockProtected.filenames[trustedUncompressedDigest] = filename
			s.lockProtected.fileSizes[trustedUncompressedDigest] = fileSize
			s.lock.Unlock()
		}
	}
	// Read the cached blob and use it as a diff.
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening file %q: %w", filename, err)
	}
	defer file.Close()
	// Build the new layer using the diff, regardless of where it came from.
	// TODO: This can take quite some time, and should ideally be cancellable using ctx.Done().
	layer, _, err := s.imageRef.transport.store.PutLayer(newLayerID, parentLayer, nil, "", false, &storage.LayerOptions{
		OriginalDigest:     trustedOriginalDigest,
		UncompressedDigest: trustedUncompressedDigest,
	}, file)
	if err != nil && !errors.Is(err, storage.ErrDuplicateID) {
		return nil, fmt.Errorf("adding layer with blob %q: %w", layerDigest, err)
	}
	return layer, nil
}

// untrustedLayerDiffID returns a DiffID value for layerIndex from the image’s config.
// If the value is not yet available (but it can be available after s.manifets is set), it returns ("", nil).
// WARNING: We don’t validate the DiffID value against the layer contents; it must not be used for any deduplication.
func (s *storageImageDestination) untrustedLayerDiffID(layerIndex int) (digest.Digest, error) {
	// At this point, we are either inside the multi-threaded scope of HasThreadSafePutBlob, and
	// nothing is writing to s.manifest yet, or PutManifest has been called and s.manifest != nil.
	// Either way this function does not need the protection of s.lock.
	if s.manifest == nil {
		logrus.Debugf("Skipping commit for layer %d, manifest not yet available", layerIndex)
		return "", nil
	}

	if s.untrustedDiffIDValues == nil {
		mt := manifest.GuessMIMEType(s.manifest)
		if mt != imgspecv1.MediaTypeImageManifest {
			// We could, in principle, build an ImageSource, support arbitrary image formats using image.FromUnparsedImage,
			// and then use types.Image.OCIConfig so that we can parse the image.
			//
			// In practice, this should, right now, only matter for pulls of OCI images (this code path implies that a layer has annotation),
			// while converting to a non-OCI formats, using a manual (skopeo copy) or something similar, not (podman pull).
			// So it is not implemented yet.
			return "", fmt.Errorf("determining DiffID for manifest type %q is not yet supported", mt)
		}
		man, err := manifest.FromBlob(s.manifest, mt)
		if err != nil {
			return "", fmt.Errorf("parsing manifest: %w", err)
		}

		cb, err := s.getConfigBlob(man.ConfigInfo())
		if err != nil {
			return "", err
		}

		// retrieve the expected uncompressed digest from the config blob.
		configOCI := &imgspecv1.Image{}
		if err := json.Unmarshal(cb, configOCI); err != nil {
			return "", err
		}
		s.untrustedDiffIDValues = slices.Clone(configOCI.RootFS.DiffIDs)
		if s.untrustedDiffIDValues == nil { // Unlikely but possible in theory…
			s.untrustedDiffIDValues = []digest.Digest{}
		}
	}
	if layerIndex >= len(s.untrustedDiffIDValues) {
		return "", fmt.Errorf("image config has only %d DiffID values, but a layer with index %d exists", len(s.untrustedDiffIDValues), layerIndex)
	}
	return s.untrustedDiffIDValues[layerIndex], nil
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// unparsedToplevel contains data about the top-level manifest of the source (which may be a single-arch image or a manifest list
// if PutManifest was only called for the single-arch image with instanceDigest == nil), primarily to allow lookups by the
// original manifest list digest, if desired.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (s *storageImageDestination) Commit(ctx context.Context, unparsedToplevel types.UnparsedImage) error {
	// This function is outside of the scope of HasThreadSafePutBlob, so we don’t need to hold s.lock.

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
		if stopQueue, err := s.commitLayer(i, addedLayerInfo{
			digest:     blob.Digest,
			emptyLayer: blob.EmptyLayer,
		}, blob.Size); err != nil {
			return err
		} else if stopQueue {
			return fmt.Errorf("Internal error: storageImageDestination.Commit(): commitLayer() not ready to commit for layer %q", blob.Digest)
		}
	}
	var lastLayer string
	if len(layerBlobs) > 0 { // Zero-layer images rarely make sense, but it is technically possible, and may happen for non-image artifacts.
		prev, ok := s.indexToStorageID[len(layerBlobs)-1]
		if !ok {
			return fmt.Errorf("Internal error: storageImageDestination.Commit(): previous layer %d hasn't been committed (lastLayer == nil)", len(layerBlobs)-1)
		}
		lastLayer = prev
	}

	// If one of those blobs was a configuration blob, then we can try to dig out the date when the image
	// was originally created, in case we're just copying it.  If not, no harm done.
	options := &storage.ImageOptions{}
	if inspect, err := man.Inspect(s.getConfigBlob); err == nil && inspect.Created != nil {
		logrus.Debugf("setting image creation date to %s", inspect.Created)
		options.CreationDate = *inspect.Created
	}

	// Set up to save the non-layer blobs as data items.  Since we only share layers, they should all be in files, so
	// we just need to screen out the ones that are actually layers to get the list of non-layers.
	dataBlobs := set.New[digest.Digest]()
	for blob := range s.lockProtected.filenames {
		dataBlobs.Add(blob)
	}
	for _, layerBlob := range layerBlobs {
		dataBlobs.Delete(layerBlob.Digest)
	}
	for _, blob := range dataBlobs.Values() {
		v, err := os.ReadFile(s.lockProtected.filenames[blob])
		if err != nil {
			return fmt.Errorf("copying non-layer blob %q to image: %w", blob, err)
		}
		options.BigData = append(options.BigData, storage.ImageBigDataOption{
			Key:    blob.String(),
			Data:   v,
			Digest: digest.Canonical.FromBytes(v),
		})
	}
	// Set up to save the unparsedToplevel's manifest if it differs from
	// the per-platform one, which is saved below.
	if len(toplevelManifest) != 0 && !bytes.Equal(toplevelManifest, s.manifest) {
		manifestDigest, err := manifest.Digest(toplevelManifest)
		if err != nil {
			return fmt.Errorf("digesting top-level manifest: %w", err)
		}
		options.BigData = append(options.BigData, storage.ImageBigDataOption{
			Key:    manifestBigDataKey(manifestDigest),
			Data:   toplevelManifest,
			Digest: manifestDigest,
		})
	}
	// Set up to save the image's manifest.  Allow looking it up by digest by using the key convention defined by the Store.
	// Record the manifest twice: using a digest-specific key to allow references to that specific digest instance,
	// and using storage.ImageDigestBigDataKey for future users that don’t specify any digest and for compatibility with older readers.
	options.BigData = append(options.BigData, storage.ImageBigDataOption{
		Key:    manifestBigDataKey(s.manifestDigest),
		Data:   s.manifest,
		Digest: s.manifestDigest,
	})
	options.BigData = append(options.BigData, storage.ImageBigDataOption{
		Key:    storage.ImageDigestBigDataKey,
		Data:   s.manifest,
		Digest: s.manifestDigest,
	})
	// Set up to save the signatures, if we have any.
	if len(s.signatures) > 0 {
		options.BigData = append(options.BigData, storage.ImageBigDataOption{
			Key:    "signatures",
			Data:   s.signatures,
			Digest: digest.Canonical.FromBytes(s.signatures),
		})
	}
	for instanceDigest, signatures := range s.signatureses {
		options.BigData = append(options.BigData, storage.ImageBigDataOption{
			Key:    signatureBigDataKey(instanceDigest),
			Data:   signatures,
			Digest: digest.Canonical.FromBytes(signatures),
		})
	}

	// Set up to save our metadata.
	metadata, err := json.Marshal(s.metadata)
	if err != nil {
		return fmt.Errorf("encoding metadata for image: %w", err)
	}
	if len(metadata) != 0 {
		options.Metadata = string(metadata)
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
		// set the data items and metadata on the already-present image
		// FIXME: this _replaces_ any "signatures" blobs and their
		// sizes (tracked in the metadata) which might have already
		// been present with new values, when ideally we'd find a way
		// to merge them since they all apply to the same image
		for _, data := range options.BigData {
			if err := s.imageRef.transport.store.SetImageBigData(img.ID, data.Key, data.Data, manifest.Digest); err != nil {
				logrus.Debugf("error saving big data %q for image %q: %v", data.Key, img.ID, err)
				return fmt.Errorf("saving big data %q for image %q: %w", data.Key, img.ID, err)
			}
		}
		if options.Metadata != "" {
			if err := s.imageRef.transport.store.SetMetadata(img.ID, options.Metadata); err != nil {
				logrus.Debugf("error saving metadata for image %q: %v", img.ID, err)
				return fmt.Errorf("saving metadata for image %q: %w", img.ID, err)
			}
			logrus.Debugf("saved image metadata %q", options.Metadata)
		}
	} else {
		logrus.Debugf("created new image ID %q with metadata %q", img.ID, options.Metadata)
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

	// Add the reference's name on the image.  We don't need to worry about avoiding duplicate
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
		sigblob = append(sigblob, sig...)
	}
	if instanceDigest == nil {
		s.signatures = sigblob
		s.metadata.SignatureSizes = sizes
		if len(s.manifest) > 0 {
			manifestDigest := s.manifestDigest
			instanceDigest = &manifestDigest
		}
	}
	if instanceDigest != nil {
		s.signatureses[*instanceDigest] = sigblob
		s.metadata.SignaturesSizes[*instanceDigest] = sizes
	}
	return nil
}
