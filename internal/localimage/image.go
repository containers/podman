package localimage

import (
	"fmt"

	"github.com/containers/image/v5/docker/reference"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/pkg/errors"
)

// Image refers to a local image.
// (FIXME: is this an API commitment?) The reference is by image ID, not name.
type Image struct {
	storage *Storage
	ref     types.ImageReference // a storage reference by ID (must not contain a name, see storageImage() below)
}

// newImageFromStorageImage creates an Image referring to the input storageImage
// FIXME: This shouldn’t be public, callers should get to storage (if at all) from here.
func newImageFromStorageImage(storage *Storage, storageImage *storage.Image) (*Image, error) {
	ref, err := is.Transport.NewStoreReference(storage.store, nil, storageImage.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "internal error creating store reference for image %q", storageImage.ID)
	}
	return &Image{
		storage: storage,
		ref:     ref,
	}, nil
}

// storageImage resolves an Image into a storage.Image.
// This is typically quite fast (a hash lookup in memory + a copy of the data).
// Note that it _can_ fail if the image has been removed since creating the Image reference.
func (i *Image) storageImage() (*storage.Image, error) {
	// NOTE: This only works correctly because ref does not contain a name; otherwise GetStoreImage would look up by name.
	// FIXME: Conceptually, is.Transport.GetImage should be enough, but that uses the is.Transport default store instead of using the one
	// inside the reference.  Use an explicit store reference to be a bit less stateful.
	return is.Transport.GetStoreImage(i.storage.store, i.ref)
}

// AddTag adds tag to image.
// If the tag is already present, the method silently succeeds.
// Returns true if the tag was previously missing and added, false if the tags were not changed. // FIXME: can/should this be dropped?
// FIXME: This allows tagging using name@digest, and name:tag@digest.  Should that be allowed??!!
func (i *Image) AddTag(tag reference.Named) (bool, error) {
	if reference.IsNameOnly(tag) {
		return false, fmt.Errorf("refusing to tag an image with %q which has neither a tag nor a digest", tag.String())
	}

	si, err := i.storageImage()
	if err != nil {
		return false, err
	}
	tagString := tag.String()
	if util.StringInSlice(tagString, si.Names) {
		return false, nil
	}

	newNames := append(si.Names, tagString)
	if err := i.storage.store.SetNames(si.ID, newNames); err != nil {
		return false, err
	}
	return true, nil
}
