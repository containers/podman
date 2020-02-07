package localimage

import (
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/pkg/errors"
)

// Image refers to a local image.
// (FIXME: is this an API commitment?) The reference is by image ID, not name.
type Image struct {
	ref types.ImageReference // a storage reference by ID.
}

// newImageFromStorageImage creates an Image referring to the input storageImage
// FIXME: This shouldn’t be public, callers should get to storage (if at all) from here.
func newImageFromStorageImage(storage *Storage, storageImage *storage.Image) (*Image, error) {
	ref, err := is.Transport.NewStoreReference(storage.store, nil, storageImage.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "internal error creating store reference for image %q", storageImage.ID)
	}
	return &Image{
		ref: ref,
	}, nil
}
