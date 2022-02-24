package imagesource

import (
	"context"
	"fmt"
	"io"

	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/types"
)

// FromPublic(src) returns an object that provides the private.ImageSource API
//
// Eventually, we might want to expose this function, and methods of the returned object,
// as a public API (or rather, a variant that does not include the already-superseded
// methods of types.ImageSource, and has added more future-proofing), and more strongly
// deprecate direct use of types.ImageSource.
//
// NOTE: The returned API MUST NOT be a public interface (it can be either just a struct
// with public methods, or perhaps a private interface), so that we can add methods
// without breaking any external implementors of a public interface.
func FromPublic(src types.ImageSource) private.ImageSource {
	if src2, ok := src.(private.ImageSource); ok {
		return src2
	}
	return &wrapped{ImageSource: src}
}

// wrapped provides the private.ImageSource operations
// for a source that only implements types.ImageSource
type wrapped struct {
	types.ImageSource
}

// SupportsGetBlobAt() returns true if GetBlobAt (BlobChunkAccessor) is supported.
func (w *wrapped) SupportsGetBlobAt() bool {
	return false
}

// GetBlobAt returns a sequential channel of readers that contain data for the requested
// blob chunks, and a channel that might get a single error value.
// The specified chunks must be not overlapping and sorted by their offset.
// The readers must be fully consumed, in the order they are returned, before blocking
// to read the next chunk.
func (w *wrapped) GetBlobAt(ctx context.Context, info types.BlobInfo, chunks []private.ImageSourceChunk) (chan io.ReadCloser, chan error, error) {
	return nil, nil, fmt.Errorf("internal error: GetBlobAt is not supported by the %q transport", w.Reference().Transport().Name())
}
