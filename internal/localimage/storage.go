package localimage

import (
	"fmt"

	is "github.com/containers/image/v5/storage"
	"github.com/containers/storage"
)

// Storage is the local image store.
// FIXME: naming?
type Storage struct {
	store storage.Store // is.Transport.GetStore() should always return this store.
}

// NewStorage creates a Storage from a configured c/storage store.
// WARNING: The "store" value must be the same in all calls.
// FIXME: naming?
func NewStorage(store storage.Store) (*Storage, error) { // FIXME: Return an interface?
	// Only one instance of is.Transport can exist, to resolve "containers-storage:*" reference formats consistently throughout the process.
	// FIXME: Should this be reconsidered in favor of multiple simultaneous stores?
	// It would, I guess, make tests easier, but it would also cause confusion as long as strings are used, and most practical uses
	// in Podman/CRI-O are very likely to be bugs.

	current := is.Transport.GetStoreIfSet()
	if current != nil && store != current {
		return nil, fmt.Errorf("NewStorage called with store %#v while a different store %#v was previously set up", store, current)
	}
	is.Transport.SetStore(store)
	return &Storage{
		store: store,
	}, nil
}

// NewImageFromStorageImage creates an Image referring to the input storageImage
// FIXME: This shouldn’t be public, callers should get to storage (if at all) from here.
func (s *Storage) NewImageFromStorageImage(storageImage *storage.Image) (*Image, error) { // FIXME: Return an interface?
	return newImageFromStorageImage(s, storageImage)
}
