package stubs

import (
	"context"
	"fmt"

	"github.com/containers/image/v5/internal/blobinfocache"
	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/types"
)

// NoPutBlobPartialInitialize implements parts of private.ImageDestination
// for transports that don’t support PutBlobPartial().
// See NoPutBlobPartial() below.
type NoPutBlobPartialInitialize struct {
	transportName string
}

// NoPutBlobPartial creates a NoPutBlobPartialInitialize for ref.
func NoPutBlobPartial(ref types.ImageReference) NoPutBlobPartialInitialize {
	return NoPutBlobPartialRaw(ref.Transport().Name())
}

// NoPutBlobPartialRaw is the same thing as NoPutBlobPartial, but it can be used
// in situations where no ImageReference is available.
func NoPutBlobPartialRaw(transportName string) NoPutBlobPartialInitialize {
	return NoPutBlobPartialInitialize{
		transportName: transportName,
	}
}

// SupportsPutBlobPartial returns true if PutBlobPartial is supported.
func (stub NoPutBlobPartialInitialize) SupportsPutBlobPartial() bool {
	return false
}

// PutBlobPartial attempts to create a blob using the data that is already present
// at the destination. chunkAccessor is accessed in a non-sequential way to retrieve the missing chunks.
// It is available only if SupportsPutBlobPartial().
// Even if SupportsPutBlobPartial() returns true, the call can fail, in which case the caller
// should fall back to PutBlobWithOptions.
func (stub NoPutBlobPartialInitialize) PutBlobPartial(ctx context.Context, chunkAccessor private.BlobChunkAccessor, srcInfo types.BlobInfo, cache blobinfocache.BlobInfoCache2) (types.BlobInfo, error) {
	return types.BlobInfo{}, fmt.Errorf("internal error: PutBlobPartial is not supported by the %q transport", stub.transportName)
}

// ImplementsPutBlobPartial implements SupportsPutBlobPartial() that returns true.
type ImplementsPutBlobPartial struct{}

// SupportsPutBlobPartial returns true if PutBlobPartial is supported.
func (stub ImplementsPutBlobPartial) SupportsPutBlobPartial() bool {
	return true
}
