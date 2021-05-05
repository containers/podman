package types

import (
	"context"
	"io"

	"github.com/containers/image/v5/docker/reference"
	publicTypes "github.com/containers/image/v5/types"
)

// ImageDestinationWithOptions is an internal extension to the ImageDestination
// interface.
type ImageDestinationWithOptions interface {
	publicTypes.ImageDestination

	// PutBlobWithOptions is a wrapper around PutBlob.  If
	// options.LayerIndex is set, the blob will be committed directly.
	// Either by the calling goroutine or by another goroutine already
	// committing layers.
	//
	// Please note that TryReusingBlobWithOptions and PutBlobWithOptions
	// *must* be used the together.  Mixing the two with non "WithOptions"
	// functions is not supported.
	PutBlobWithOptions(ctx context.Context, stream io.Reader, blobinfo publicTypes.BlobInfo, options PutBlobOptions) (publicTypes.BlobInfo, error)

	// TryReusingBlobWithOptions is a wrapper around TryReusingBlob.  If
	// options.LayerIndex is set, the reused blob will be recoreded as
	// already pulled.
	//
	// Please note that TryReusingBlobWithOptions and PutBlobWithOptions
	// *must* be used the together.  Mixing the two with non "WithOptions"
	// functions is not supported.
	TryReusingBlobWithOptions(ctx context.Context, blobinfo publicTypes.BlobInfo, options TryReusingBlobOptions) (bool, publicTypes.BlobInfo, error)
}

// PutBlobOptions are used in PutBlobWithOptions.
type PutBlobOptions struct {
	// Cache to look up blob infos.
	Cache publicTypes.BlobInfoCache
	// Denotes whether the blob is a config or not.
	IsConfig bool
	// The corresponding index in the layer slice.
	LayerIndex *int
}

// TryReusingBlobOptions are used in TryReusingBlobWithOptions.
type TryReusingBlobOptions struct {
	// Cache to look up blob infos.
	Cache publicTypes.BlobInfoCache
	// Use an equivalent of the desired blob.
	CanSubstitute bool
	// The corresponding index in the layer slice.
	LayerIndex *int
	// The reference of the image that contains the target blob.
	SrcRef reference.Named
}
