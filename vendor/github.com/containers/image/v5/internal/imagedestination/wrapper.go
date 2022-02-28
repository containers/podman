package imagedestination

import (
	"context"
	"fmt"
	"io"

	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/types"
)

// FromPublic(dest) returns an object that provides the private.ImageDestination API
//
// Eventually, we might want to expose this function, and methods of the returned object,
// as a public API (or rather, a variant that does not include the already-superseded
// methods of types.ImageDestination, and has added more future-proofing), and more strongly
// deprecate direct use of types.ImageDestination.
//
// NOTE: The returned API MUST NOT be a public interface (it can be either just a struct
// with public methods, or perhaps a private interface), so that we can add methods
// without breaking any external implementors of a public interface.
func FromPublic(dest types.ImageDestination) private.ImageDestination {
	if dest2, ok := dest.(private.ImageDestination); ok {
		return dest2
	}
	return &wrapped{ImageDestination: dest}
}

// wrapped provides the private.ImageDestination operations
// for a destination that only implements types.ImageDestination
type wrapped struct {
	types.ImageDestination
}

// SupportsPutBlobPartial returns true if PutBlobPartial is supported.
func (w *wrapped) SupportsPutBlobPartial() bool {
	return false
}

// PutBlobWithOptions writes contents of stream and returns data representing the result.
// inputInfo.Digest can be optionally provided if known; if provided, and stream is read to the end without error, the digest MUST match the stream contents.
// inputInfo.Size is the expected length of stream, if known.
// inputInfo.MediaType describes the blob format, if known.
// WARNING: The contents of stream are being verified on the fly.  Until stream.Read() returns io.EOF, the contents of the data SHOULD NOT be available
// to any other readers for download using the supplied digest.
// If stream.Read() at any time, ESPECIALLY at end of input, returns an error, PutBlob MUST 1) fail, and 2) delete any data stored so far.
func (w *wrapped) PutBlobWithOptions(ctx context.Context, stream io.Reader, inputInfo types.BlobInfo, options private.PutBlobOptions) (types.BlobInfo, error) {
	return w.PutBlob(ctx, stream, inputInfo, options.Cache, options.IsConfig)
}

// PutBlobPartial attempts to create a blob using the data that is already present
// at the destination. chunkAccessor is accessed in a non-sequential way to retrieve the missing chunks.
// It is available only if SupportsPutBlobPartial().
// Even if SupportsPutBlobPartial() returns true, the call can fail, in which case the caller
// should fall back to PutBlobWithOptions.
func (w *wrapped) PutBlobPartial(ctx context.Context, chunkAccessor private.BlobChunkAccessor, srcInfo types.BlobInfo, cache types.BlobInfoCache) (types.BlobInfo, error) {
	return types.BlobInfo{}, fmt.Errorf("internal error: PutBlobPartial is not supported by the %q transport", w.Reference().Transport().Name())
}

// TryReusingBlobWithOptions checks whether the transport already contains, or can efficiently reuse, a blob, and if so, applies it to the current destination
// (e.g. if the blob is a filesystem layer, this signifies that the changes it describes need to be applied again when composing a filesystem tree).
// info.Digest must not be empty.
// If the blob has been successfully reused, returns (true, info, nil); info must contain at least a digest and size, and may
// include CompressionOperation and CompressionAlgorithm fields to indicate that a change to the compression type should be
// reflected in the manifest that will be written.
// If the transport can not reuse the requested blob, TryReusingBlob returns (false, {}, nil); it returns a non-nil error only on an unexpected failure.
func (w *wrapped) TryReusingBlobWithOptions(ctx context.Context, info types.BlobInfo, options private.TryReusingBlobOptions) (bool, types.BlobInfo, error) {
	return w.TryReusingBlob(ctx, info, options.Cache, options.CanSubstitute)
}
