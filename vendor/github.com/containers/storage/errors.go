package storage

import (
	"errors"
)

var (
	// ErrContainerUnknown indicates that there was no container with the specified name or ID.
	ErrContainerUnknown = errors.New("container not known")
	// ErrImageUnknown indicates that there was no image with the specified name or ID.
	ErrImageUnknown = errors.New("image not known")
	// ErrParentUnknown indicates that we didn't record the ID of the parent of the specified layer.
	ErrParentUnknown = errors.New("parent of layer not known")
	// ErrLayerUnknown indicates that there was no layer with the specified name or ID.
	ErrLayerUnknown = errors.New("layer not known")
	// ErrLoadError indicates that there was an initialization error.
	ErrLoadError = errors.New("error loading storage metadata")
	// ErrDuplicateID indicates that an ID which is to be assigned to a new item is already being used.
	ErrDuplicateID = errors.New("that ID is already in use")
	// ErrDuplicateName indicates that a name which is to be assigned to a new item is already being used.
	ErrDuplicateName = errors.New("that name is already in use")
	// ErrParentIsContainer is returned when a caller attempts to create a layer as a child of a container's layer.
	ErrParentIsContainer = errors.New("would-be parent layer is a container")
	// ErrNotAContainer is returned when the caller attempts to delete a container that isn't a container.
	ErrNotAContainer = errors.New("identifier is not a container")
	// ErrNotAnImage is returned when the caller attempts to delete an image that isn't an image.
	ErrNotAnImage = errors.New("identifier is not an image")
	// ErrNotALayer is returned when the caller attempts to delete a layer that isn't a layer.
	ErrNotALayer = errors.New("identifier is not a layer")
	// ErrNotAnID is returned when the caller attempts to read or write metadata from an item that doesn't exist.
	ErrNotAnID = errors.New("identifier is not a layer, image, or container")
	// ErrLayerHasChildren is returned when the caller attempts to delete a layer that has children.
	ErrLayerHasChildren = errors.New("layer has children")
	// ErrLayerUsedByImage is returned when the caller attempts to delete a layer that is an image's top layer.
	ErrLayerUsedByImage = errors.New("layer is in use by an image")
	// ErrLayerUsedByContainer is returned when the caller attempts to delete a layer that is a container's layer.
	ErrLayerUsedByContainer = errors.New("layer is in use by a container")
	// ErrImageUsedByContainer is returned when the caller attempts to delete an image that is a container's image.
	ErrImageUsedByContainer = errors.New("image is in use by a container")
	// ErrIncompleteOptions is returned when the caller attempts to initialize a Store without providing required information.
	ErrIncompleteOptions = errors.New("missing necessary StoreOptions")
	// ErrSizeUnknown is returned when the caller asks for the size of a big data item, but the Store couldn't determine the answer.
	ErrSizeUnknown = errors.New("size is not known")
	// ErrStoreIsReadOnly is returned when the caller makes a call to a read-only store that would require modifying its contents.
	ErrStoreIsReadOnly = errors.New("called a write method on a read-only store")
	// ErrDuplicateImageNames indicates that the read-only store uses the same name for multiple images.
	ErrDuplicateImageNames = errors.New("read-only image store assigns the same name to multiple images")
	// ErrDuplicateLayerNames indicates that the read-only store uses the same name for multiple layers.
	ErrDuplicateLayerNames = errors.New("read-only layer store assigns the same name to multiple layers")
	// ErrInvalidBigDataName indicates that the name for a big data item is not acceptable; it may be empty.
	ErrInvalidBigDataName = errors.New("not a valid name for a big data item")
	// ErrDigestUnknown indicates that we were unable to compute the digest of a specified item.
	ErrDigestUnknown = errors.New("could not compute digest of item")
	// ErrLayerNotMounted is returned when the requested information can only be computed for a mounted layer, and the layer is not mounted.
	ErrLayerNotMounted = errors.New("layer is not mounted")
)
