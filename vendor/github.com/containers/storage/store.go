package storage

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	// register all of the built-in drivers
	_ "github.com/containers/storage/drivers/register"

	drivers "github.com/containers/storage/drivers"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/directory"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/containers/storage/pkg/parsers"
	"github.com/containers/storage/pkg/stringutils"
	"github.com/containers/storage/pkg/system"
	"github.com/containers/storage/types"
	"github.com/hashicorp/go-multierror"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
)

type updateNameOperation int

const (
	setNames updateNameOperation = iota
	addNames
	removeNames
)

const (
	volatileFlag     = "Volatile"
	mountLabelFlag   = "MountLabel"
	processLabelFlag = "ProcessLabel"
	mountOptsFlag    = "MountOpts"
)

var (
	stores     []*store
	storesLock sync.Mutex
)

// roMetadataStore wraps a method for reading metadata associated with an ID.
type roMetadataStore interface {
	// Metadata reads metadata associated with an item with the specified ID.
	Metadata(id string) (string, error)
}

// rwMetadataStore wraps a method for setting metadata associated with an ID.
type rwMetadataStore interface {
	// SetMetadata updates the metadata associated with the item with the specified ID.
	SetMetadata(id, metadata string) error
}

// metadataStore wraps up methods for getting and setting metadata associated with IDs.
type metadataStore interface {
	roMetadataStore
	rwMetadataStore
}

// An roBigDataStore wraps up the read-only big-data related methods of the
// various types of file-based lookaside stores that we implement.
type roBigDataStore interface {
	// BigData retrieves a (potentially large) piece of data associated with
	// this ID, if it has previously been set.
	BigData(id, key string) ([]byte, error)

	// BigDataSize retrieves the size of a (potentially large) piece of
	// data associated with this ID, if it has previously been set.
	BigDataSize(id, key string) (int64, error)

	// BigDataDigest retrieves the digest of a (potentially large) piece of
	// data associated with this ID, if it has previously been set.
	BigDataDigest(id, key string) (digest.Digest, error)

	// BigDataNames() returns a list of the names of previously-stored pieces of
	// data.
	BigDataNames(id string) ([]string, error)
}

// A rwImageBigDataStore wraps up how we store big-data associated with images.
type rwImageBigDataStore interface {
	// SetBigData stores a (potentially large) piece of data associated
	// with this ID.
	// Pass github.com/containers/image/manifest.Digest as digestManifest
	// to allow ByDigest to find images by their correct digests.
	SetBigData(id, key string, data []byte, digestManifest func([]byte) (digest.Digest, error)) error
}

// A containerBigDataStore wraps up how we store big-data associated with containers.
type containerBigDataStore interface {
	roBigDataStore
	// SetBigData stores a (potentially large) piece of data associated
	// with this ID.
	SetBigData(id, key string, data []byte) error
}

// A roLayerBigDataStore wraps up how we store RO big-data associated with layers.
type roLayerBigDataStore interface {
	// SetBigData stores a (potentially large) piece of data associated
	// with this ID.
	BigData(id, key string) (io.ReadCloser, error)

	// BigDataNames() returns a list of the names of previously-stored pieces of
	// data.
	BigDataNames(id string) ([]string, error)
}

// A rwLayerBigDataStore wraps up how we store big-data associated with layers.
type rwLayerBigDataStore interface {
	// SetBigData stores a (potentially large) piece of data associated
	// with this ID.
	SetBigData(id, key string, data io.Reader) error
}

// A flaggableStore can have flags set and cleared on items which it manages.
type flaggableStore interface {
	// ClearFlag removes a named flag from an item in the store.
	ClearFlag(id string, flag string) error

	// SetFlag sets a named flag and its value on an item in the store.
	SetFlag(id string, flag string, value interface{}) error
}

type StoreOptions = types.StoreOptions

// Store wraps up the various types of file-based stores that we use into a
// singleton object that initializes and manages them all together.
type Store interface {
	// RunRoot, GraphRoot, GraphDriverName, and GraphOptions retrieve
	// settings that were passed to GetStore() when the object was created.
	RunRoot() string
	GraphRoot() string
	GraphDriverName() string
	GraphOptions() []string
	PullOptions() map[string]string
	UIDMap() []idtools.IDMap
	GIDMap() []idtools.IDMap

	// GraphDriver obtains and returns a handle to the graph Driver object used
	// by the Store.
	GraphDriver() (drivers.Driver, error)

	// CreateLayer creates a new layer in the underlying storage driver,
	// optionally having the specified ID (one will be assigned if none is
	// specified), with the specified layer (or no layer) as its parent,
	// and with optional names.  (The writeable flag is ignored.)
	CreateLayer(id, parent string, names []string, mountLabel string, writeable bool, options *LayerOptions) (*Layer, error)

	// PutLayer combines the functions of CreateLayer and ApplyDiff,
	// marking the layer for automatic removal if applying the diff fails
	// for any reason.
	//
	// Note that we do some of this work in a child process.  The calling
	// process's main() function needs to import our pkg/reexec package and
	// should begin with something like this in order to allow us to
	// properly start that child process:
	//   if reexec.Init() {
	//       return
	//   }
	PutLayer(id, parent string, names []string, mountLabel string, writeable bool, options *LayerOptions, diff io.Reader) (*Layer, int64, error)

	// CreateImage creates a new image, optionally with the specified ID
	// (one will be assigned if none is specified), with optional names,
	// referring to a specified image, and with optional metadata.  An
	// image is a record which associates the ID of a layer with a
	// additional bookkeeping information which the library stores for the
	// convenience of its caller.
	CreateImage(id string, names []string, layer, metadata string, options *ImageOptions) (*Image, error)

	// CreateContainer creates a new container, optionally with the
	// specified ID (one will be assigned if none is specified), with
	// optional names, using the specified image's top layer as the basis
	// for the container's layer, and assigning the specified ID to that
	// layer (one will be created if none is specified).  A container is a
	// layer which is associated with additional bookkeeping information
	// which the library stores for the convenience of its caller.
	CreateContainer(id string, names []string, image, layer, metadata string, options *ContainerOptions) (*Container, error)

	// Metadata retrieves the metadata which is associated with a layer,
	// image, or container (whichever the passed-in ID refers to).
	Metadata(id string) (string, error)

	// SetMetadata updates the metadata which is associated with a layer,
	// image, or container (whichever the passed-in ID refers to) to match
	// the specified value.  The metadata value can be retrieved at any
	// time using Metadata, or using Layer, Image, or Container and reading
	// the object directly.
	SetMetadata(id, metadata string) error

	// Exists checks if there is a layer, image, or container which has the
	// passed-in ID or name.
	Exists(id string) bool

	// Status asks for a status report, in the form of key-value pairs,
	// from the underlying storage driver.  The contents vary from driver
	// to driver.
	Status() ([][2]string, error)

	// Delete removes the layer, image, or container which has the
	// passed-in ID or name.  Note that no safety checks are performed, so
	// this can leave images with references to layers which do not exist,
	// and layers with references to parents which no longer exist.
	Delete(id string) error

	// DeleteLayer attempts to remove the specified layer.  If the layer is the
	// parent of any other layer, or is referred to by any images, it will return
	// an error.
	DeleteLayer(id string) error

	// DeleteImage removes the specified image if it is not referred to by
	// any containers.  If its top layer is then no longer referred to by
	// any other images and is not the parent of any other layers, its top
	// layer will be removed.  If that layer's parent is no longer referred
	// to by any other images and is not the parent of any other layers,
	// then it, too, will be removed.  This procedure will be repeated
	// until a layer which should not be removed, or the base layer, is
	// reached, at which point the list of removed layers is returned.  If
	// the commit argument is false, the image and layers are not removed,
	// but the list of layers which would be removed is still returned.
	DeleteImage(id string, commit bool) (layers []string, err error)

	// DeleteContainer removes the specified container and its layer.  If
	// there is no matching container, or if the container exists but its
	// layer does not, an error will be returned.
	DeleteContainer(id string) error

	// Wipe removes all known layers, images, and containers.
	Wipe() error

	// MountImage mounts an image to temp directory and returns the mount point.
	// MountImage allows caller to mount an image. Images will always
	// be mounted read/only
	MountImage(id string, mountOptions []string, mountLabel string) (string, error)

	// Unmount attempts to unmount an image, given an ID.
	// Returns whether or not the layer is still mounted.
	UnmountImage(id string, force bool) (bool, error)

	// Mount attempts to mount a layer, image, or container for access, and
	// returns the pathname if it succeeds.
	// Note if the mountLabel == "", the default label for the container
	// will be used.
	//
	// Note that we do some of this work in a child process.  The calling
	// process's main() function needs to import our pkg/reexec package and
	// should begin with something like this in order to allow us to
	// properly start that child process:
	//   if reexec.Init() {
	//       return
	//   }
	Mount(id, mountLabel string) (string, error)

	// Unmount attempts to unmount a layer, image, or container, given an ID, a
	// name, or a mount path. Returns whether or not the layer is still mounted.
	Unmount(id string, force bool) (bool, error)

	// Mounted returns number of times the layer has been mounted.
	Mounted(id string) (int, error)

	// Changes returns a summary of the changes which would need to be made
	// to one layer to make its contents the same as a second layer.  If
	// the first layer is not specified, the second layer's parent is
	// assumed.  Each Change structure contains a Path relative to the
	// layer's root directory, and a Kind which is either ChangeAdd,
	// ChangeModify, or ChangeDelete.
	Changes(from, to string) ([]archive.Change, error)

	// DiffSize returns a count of the size of the tarstream which would
	// specify the changes returned by Changes.
	DiffSize(from, to string) (int64, error)

	// Diff returns the tarstream which would specify the changes returned
	// by Changes.  If options are passed in, they can override default
	// behaviors.
	Diff(from, to string, options *DiffOptions) (io.ReadCloser, error)

	// ApplyDiff applies a tarstream to a layer.  Information about the
	// tarstream is cached with the layer.  Typically, a layer which is
	// populated using a tarstream will be expected to not be modified in
	// any other way, either before or after the diff is applied.
	//
	// Note that we do some of this work in a child process.  The calling
	// process's main() function needs to import our pkg/reexec package and
	// should begin with something like this in order to allow us to
	// properly start that child process:
	//   if reexec.Init() {
	//       return
	//   }
	ApplyDiff(to string, diff io.Reader) (int64, error)

	// ApplyDiffer applies a diff to a layer.
	// It is the caller responsibility to clean the staging directory if it is not
	// successfully applied with ApplyDiffFromStagingDirectory.
	ApplyDiffWithDiffer(to string, options *drivers.ApplyDiffOpts, differ drivers.Differ) (*drivers.DriverWithDifferOutput, error)

	// ApplyDiffFromStagingDirectory uses stagingDirectory to create the diff.
	ApplyDiffFromStagingDirectory(to, stagingDirectory string, diffOutput *drivers.DriverWithDifferOutput, options *drivers.ApplyDiffOpts) error

	// CleanupStagingDirectory cleanups the staging directory.  It can be used to cleanup the staging directory on errors
	CleanupStagingDirectory(stagingDirectory string) error

	// DifferTarget gets the path to the differ target.
	DifferTarget(id string) (string, error)

	// LayersByCompressedDigest returns a slice of the layers with the
	// specified compressed digest value recorded for them.
	LayersByCompressedDigest(d digest.Digest) ([]Layer, error)

	// LayersByUncompressedDigest returns a slice of the layers with the
	// specified uncompressed digest value recorded for them.
	LayersByUncompressedDigest(d digest.Digest) ([]Layer, error)

	// LayerSize returns a cached approximation of the layer's size, or -1
	// if we don't have a value on hand.
	LayerSize(id string) (int64, error)

	// LayerParentOwners returns the UIDs and GIDs of owners of parents of
	// the layer's mountpoint for which the layer's UID and GID maps (if
	// any are defined) don't contain corresponding IDs.
	LayerParentOwners(id string) ([]int, []int, error)

	// Layers returns a list of the currently known layers.
	Layers() ([]Layer, error)

	// Images returns a list of the currently known images.
	Images() ([]Image, error)

	// Containers returns a list of the currently known containers.
	Containers() ([]Container, error)

	// Names returns the list of names for a layer, image, or container.
	Names(id string) ([]string, error)

	// Free removes the store from the list of stores
	Free()

	// SetNames changes the list of names for a layer, image, or container.
	// Duplicate names are removed from the list automatically.
	// Deprecated: Prone to race conditions, suggested alternatives are `AddNames` and `RemoveNames`.
	SetNames(id string, names []string) error

	// AddNames adds the list of names for a layer, image, or container.
	// Duplicate names are removed from the list automatically.
	AddNames(id string, names []string) error

	// RemoveNames removes the list of names for a layer, image, or container.
	// Duplicate names are removed from the list automatically.
	RemoveNames(id string, names []string) error

	// ListImageBigData retrieves a list of the (possibly large) chunks of
	// named data associated with an image.
	ListImageBigData(id string) ([]string, error)

	// ImageBigData retrieves a (possibly large) chunk of named data
	// associated with an image.
	ImageBigData(id, key string) ([]byte, error)

	// ImageBigDataSize retrieves the size of a (possibly large) chunk
	// of named data associated with an image.
	ImageBigDataSize(id, key string) (int64, error)

	// ImageBigDataDigest retrieves the digest of a (possibly large) chunk
	// of named data associated with an image.
	ImageBigDataDigest(id, key string) (digest.Digest, error)

	// SetImageBigData stores a (possibly large) chunk of named data
	// associated with an image.  Pass
	// github.com/containers/image/manifest.Digest as digestManifest to
	// allow ImagesByDigest to find images by their correct digests.
	SetImageBigData(id, key string, data []byte, digestManifest func([]byte) (digest.Digest, error)) error

	// ListLayerBigData retrieves a list of the (possibly large) chunks of
	// named data associated with a layer.
	ListLayerBigData(id string) ([]string, error)

	// LayerBigData retrieves a (possibly large) chunk of named data
	// associated with a layer.
	LayerBigData(id, key string) (io.ReadCloser, error)

	// SetLayerBigData stores a (possibly large) chunk of named data
	// associated with a layer.
	SetLayerBigData(id, key string, data io.Reader) error

	// ImageSize computes the size of the image's layers and ancillary data.
	ImageSize(id string) (int64, error)

	// ListContainerBigData retrieves a list of the (possibly large) chunks of
	// named data associated with a container.
	ListContainerBigData(id string) ([]string, error)

	// ContainerBigData retrieves a (possibly large) chunk of named data
	// associated with a container.
	ContainerBigData(id, key string) ([]byte, error)

	// ContainerBigDataSize retrieves the size of a (possibly large)
	// chunk of named data associated with a container.
	ContainerBigDataSize(id, key string) (int64, error)

	// ContainerBigDataDigest retrieves the digest of a (possibly large)
	// chunk of named data associated with a container.
	ContainerBigDataDigest(id, key string) (digest.Digest, error)

	// SetContainerBigData stores a (possibly large) chunk of named data
	// associated with a container.
	SetContainerBigData(id, key string, data []byte) error

	// ContainerSize computes the size of the container's layer and ancillary
	// data.  Warning:  this is a potentially expensive operation.
	ContainerSize(id string) (int64, error)

	// Layer returns a specific layer.
	Layer(id string) (*Layer, error)

	// Image returns a specific image.
	Image(id string) (*Image, error)

	// ImagesByTopLayer returns a list of images which reference the specified
	// layer as their top layer.  They will have different IDs and names
	// and may have different metadata, big data items, and flags.
	ImagesByTopLayer(id string) ([]*Image, error)

	// ImagesByDigest returns a list of images which contain a big data item
	// named ImageDigestBigDataKey whose contents have the specified digest.
	ImagesByDigest(d digest.Digest) ([]*Image, error)

	// Container returns a specific container.
	Container(id string) (*Container, error)

	// ContainerByLayer returns a specific container based on its layer ID or
	// name.
	ContainerByLayer(id string) (*Container, error)

	// ContainerDirectory returns a path of a directory which the caller
	// can use to store data, specific to the container, which the library
	// does not directly manage.  The directory will be deleted when the
	// container is deleted.
	ContainerDirectory(id string) (string, error)

	// SetContainerDirectoryFile is a convenience function which stores
	// a piece of data in the specified file relative to the container's
	// directory.
	SetContainerDirectoryFile(id, file string, data []byte) error

	// FromContainerDirectory is a convenience function which reads
	// the contents of the specified file relative to the container's
	// directory.
	FromContainerDirectory(id, file string) ([]byte, error)

	// ContainerRunDirectory returns a path of a directory which the
	// caller can use to store data, specific to the container, which the
	// library does not directly manage.  The directory will be deleted
	// when the host system is restarted.
	ContainerRunDirectory(id string) (string, error)

	// SetContainerRunDirectoryFile is a convenience function which stores
	// a piece of data in the specified file relative to the container's
	// run directory.
	SetContainerRunDirectoryFile(id, file string, data []byte) error

	// FromContainerRunDirectory is a convenience function which reads
	// the contents of the specified file relative to the container's run
	// directory.
	FromContainerRunDirectory(id, file string) ([]byte, error)

	// ContainerParentOwners returns the UIDs and GIDs of owners of parents
	// of the container's layer's mountpoint for which the layer's UID and
	// GID maps (if any are defined) don't contain corresponding IDs.
	ContainerParentOwners(id string) ([]int, []int, error)

	// Lookup returns the ID of a layer, image, or container with the specified
	// name or ID.
	Lookup(name string) (string, error)

	// Shutdown attempts to free any kernel resources which are being used
	// by the underlying driver.  If "force" is true, any mounted (i.e., in
	// use) layers are unmounted beforehand.  If "force" is not true, then
	// layers being in use is considered to be an error condition.  A list
	// of still-mounted layers is returned along with possible errors.
	Shutdown(force bool) (layers []string, err error)

	// Version returns version information, in the form of key-value pairs, from
	// the storage package.
	Version() ([][2]string, error)

	// GetDigestLock returns digest-specific Locker.
	GetDigestLock(digest.Digest) (Locker, error)

	// LayerFromAdditionalLayerStore searches layers from the additional layer store and
	// returns the object for handling this. Note that this hasn't been stored to this store
	// yet so this needs to be done through PutAs method.
	// Releasing AdditionalLayer handler is caller's responsibility.
	// This API is experimental and can be changed without bumping the major version number.
	LookupAdditionalLayer(d digest.Digest, imageref string) (AdditionalLayer, error)
}

// AdditionalLayer reprents a layer that is contained in the additional layer store
// This API is experimental and can be changed without bumping the major version number.
type AdditionalLayer interface {
	// PutAs creates layer based on this handler, using diff contents from the additional
	// layer store.
	PutAs(id, parent string, names []string) (*Layer, error)

	// UncompressedDigest returns the uncompressed digest of this layer
	UncompressedDigest() digest.Digest

	// CompressedSize returns the compressed size of this layer
	CompressedSize() int64

	// Release tells the additional layer store that we don't use this handler.
	Release()
}

type AutoUserNsOptions = types.AutoUserNsOptions

type IDMappingOptions = types.IDMappingOptions

// LayerOptions is used for passing options to a Store's CreateLayer() and PutLayer() methods.
type LayerOptions struct {
	// IDMappingOptions specifies the type of ID mapping which should be
	// used for this layer.  If nothing is specified, the layer will
	// inherit settings from its parent layer or, if it has no parent
	// layer, the Store object.
	types.IDMappingOptions
	// TemplateLayer is the ID of a layer whose contents will be used to
	// initialize this layer.  If set, it should be a child of the layer
	// which we want to use as the parent of the new layer.
	TemplateLayer string
	// OriginalDigest specifies a digest of the tarstream (diff), if one is
	// provided along with these LayerOptions, and reliably known by the caller.
	// Use the default "" if this fields is not applicable or the value is not known.
	OriginalDigest digest.Digest
	// UncompressedDigest specifies a digest of the uncompressed version (“DiffID”)
	// of the tarstream (diff), if one is provided along with these LayerOptions,
	// and reliably known by the caller.
	// Use the default "" if this fields is not applicable or the value is not known.
	UncompressedDigest digest.Digest
}

// ImageOptions is used for passing options to a Store's CreateImage() method.
type ImageOptions struct {
	// CreationDate, if not zero, will override the default behavior of marking the image as having been
	// created when CreateImage() was called, recording CreationDate instead.
	CreationDate time.Time
	// Digest is a hard-coded digest value that we can use to look up the image.  It is optional.
	Digest digest.Digest
}

// ContainerOptions is used for passing options to a Store's CreateContainer() method.
type ContainerOptions struct {
	// IDMappingOptions specifies the type of ID mapping which should be
	// used for this container's layer.  If nothing is specified, the
	// container's layer will inherit settings from the image's top layer
	// or, if it is not being created based on an image, the Store object.
	types.IDMappingOptions
	LabelOpts  []string
	Flags      map[string]interface{}
	MountOpts  []string
	Volatile   bool
	StorageOpt map[string]string
}

type store struct {
	lastLoaded      time.Time
	runRoot         string
	graphLock       Locker
	usernsLock      Locker
	graphRoot       string
	graphDriverName string
	graphOptions    []string
	pullOptions     map[string]string
	uidMap          []idtools.IDMap
	gidMap          []idtools.IDMap
	autoUsernsUser  string
	additionalUIDs  *idSet // Set by getAvailableIDs()
	additionalGIDs  *idSet // Set by getAvailableIDs()
	autoNsMinSize   uint32
	autoNsMaxSize   uint32
	graphDriver     drivers.Driver
	layerStore      rwLayerStore
	roLayerStores   []roLayerStore
	imageStore      rwImageStore
	roImageStores   []roImageStore
	containerStore  rwContainerStore
	digestLockRoot  string
	disableVolatile bool
}

// GetStore attempts to find an already-created Store object matching the
// specified location and graph driver, and if it can't, it creates and
// initializes a new Store object, and the underlying storage that it controls.
//
// If StoreOptions `options` haven't been fully populated, then DefaultStoreOptions are used.
//
// These defaults observe environment variables:
//   - `STORAGE_DRIVER` for the name of the storage driver to attempt to use
//   - `STORAGE_OPTS` for the string of options to pass to the driver
//
// Note that we do some of this work in a child process.  The calling process's
// main() function needs to import our pkg/reexec package and should begin with
// something like this in order to allow us to properly start that child
// process:
//
//	if reexec.Init() {
//	    return
//	}
func GetStore(options types.StoreOptions) (Store, error) {
	defaultOpts, err := types.Options()
	if err != nil {
		return nil, err
	}
	if options.RunRoot == "" && options.GraphRoot == "" && options.GraphDriverName == "" && len(options.GraphDriverOptions) == 0 {
		options = defaultOpts
	}

	if options.GraphRoot != "" {
		dir, err := filepath.Abs(options.GraphRoot)
		if err != nil {
			return nil, err
		}
		options.GraphRoot = dir
	}
	if options.RunRoot != "" {
		dir, err := filepath.Abs(options.RunRoot)
		if err != nil {
			return nil, err
		}
		options.RunRoot = dir
	}

	storesLock.Lock()
	defer storesLock.Unlock()

	// return if BOTH run and graph root are matched, otherwise our run-root can be overridden if the graph is found first
	for _, s := range stores {
		if (s.graphRoot == options.GraphRoot) && (s.runRoot == options.RunRoot) && (options.GraphDriverName == "" || s.graphDriverName == options.GraphDriverName) {
			return s, nil
		}
	}

	// if passed a run-root or graph-root alone, the other should be defaulted only error if we have neither.
	switch {
	case options.RunRoot == "" && options.GraphRoot == "":
		return nil, fmt.Errorf("no storage runroot or graphroot specified: %w", ErrIncompleteOptions)
	case options.GraphRoot == "":
		options.GraphRoot = defaultOpts.GraphRoot
	case options.RunRoot == "":
		options.RunRoot = defaultOpts.RunRoot
	}

	if err := os.MkdirAll(options.RunRoot, 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(options.GraphRoot, 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(options.GraphRoot, options.GraphDriverName), 0700); err != nil {
		return nil, err
	}

	graphLock, err := GetLockfile(filepath.Join(options.GraphRoot, "storage.lock"))
	if err != nil {
		return nil, err
	}

	usernsLock, err := GetLockfile(filepath.Join(options.GraphRoot, "userns.lock"))
	if err != nil {
		return nil, err
	}

	autoNsMinSize := options.AutoNsMinSize
	autoNsMaxSize := options.AutoNsMaxSize
	if autoNsMinSize == 0 {
		autoNsMinSize = AutoUserNsMinSize
	}
	if autoNsMaxSize == 0 {
		autoNsMaxSize = AutoUserNsMaxSize
	}
	s := &store{
		runRoot:         options.RunRoot,
		graphLock:       graphLock,
		graphRoot:       options.GraphRoot,
		graphDriverName: options.GraphDriverName,
		graphOptions:    options.GraphDriverOptions,
		uidMap:          copyIDMap(options.UIDMap),
		gidMap:          copyIDMap(options.GIDMap),
		autoUsernsUser:  options.RootAutoNsUser,
		autoNsMinSize:   autoNsMinSize,
		autoNsMaxSize:   autoNsMaxSize,
		additionalUIDs:  nil,
		additionalGIDs:  nil,
		usernsLock:      usernsLock,
		disableVolatile: options.DisableVolatile,
		pullOptions:     options.PullOptions,
	}
	if err := s.load(); err != nil {
		return nil, err
	}

	stores = append(stores, s)

	return s, nil
}

func copyUint32Slice(slice []uint32) []uint32 {
	m := []uint32{}
	if slice != nil {
		m = make([]uint32, len(slice))
		copy(m, slice)
	}
	if len(m) > 0 {
		return m[:]
	}
	return nil
}

func copyIDMap(idmap []idtools.IDMap) []idtools.IDMap {
	m := []idtools.IDMap{}
	if idmap != nil {
		m = make([]idtools.IDMap, len(idmap))
		copy(m, idmap)
	}
	if len(m) > 0 {
		return m[:]
	}
	return nil
}

func (s *store) RunRoot() string {
	return s.runRoot
}

func (s *store) GraphDriverName() string {
	return s.graphDriverName
}

func (s *store) GraphRoot() string {
	return s.graphRoot
}

func (s *store) GraphOptions() []string {
	return s.graphOptions
}

func (s *store) PullOptions() map[string]string {
	cp := make(map[string]string, len(s.pullOptions))
	for k, v := range s.pullOptions {
		cp[k] = v
	}
	return cp
}

func (s *store) UIDMap() []idtools.IDMap {
	return copyIDMap(s.uidMap)
}

func (s *store) GIDMap() []idtools.IDMap {
	return copyIDMap(s.gidMap)
}

func (s *store) load() error {
	driver, err := s.GraphDriver()
	if err != nil {
		return err
	}
	s.graphDriver = driver
	s.graphDriverName = driver.String()
	driverPrefix := s.graphDriverName + "-"

	gipath := filepath.Join(s.graphRoot, driverPrefix+"images")
	if err := os.MkdirAll(gipath, 0700); err != nil {
		return err
	}
	ris, err := newImageStore(gipath)
	if err != nil {
		return err
	}
	s.imageStore = ris
	if _, err := s.getROImageStores(); err != nil {
		return err
	}

	gcpath := filepath.Join(s.graphRoot, driverPrefix+"containers")
	if err := os.MkdirAll(gcpath, 0700); err != nil {
		return err
	}
	rcs, err := newContainerStore(gcpath)
	if err != nil {
		return err
	}
	rcpath := filepath.Join(s.runRoot, driverPrefix+"containers")
	if err := os.MkdirAll(rcpath, 0700); err != nil {
		return err
	}
	s.containerStore = rcs

	for _, store := range driver.AdditionalImageStores() {
		gipath := filepath.Join(store, driverPrefix+"images")
		ris, err := newROImageStore(gipath)
		if err != nil {
			return err
		}
		s.roImageStores = append(s.roImageStores, ris)
	}

	s.digestLockRoot = filepath.Join(s.runRoot, driverPrefix+"locks")
	if err := os.MkdirAll(s.digestLockRoot, 0700); err != nil {
		return err
	}

	return nil
}

// GetDigestLock returns a digest-specific Locker.
func (s *store) GetDigestLock(d digest.Digest) (Locker, error) {
	return GetLockfile(filepath.Join(s.digestLockRoot, d.String()))
}

func (s *store) getGraphDriver() (drivers.Driver, error) {
	if s.graphDriver != nil {
		return s.graphDriver, nil
	}
	config := drivers.Options{
		Root:          s.graphRoot,
		RunRoot:       s.runRoot,
		DriverOptions: s.graphOptions,
		UIDMaps:       s.uidMap,
		GIDMaps:       s.gidMap,
	}
	driver, err := drivers.New(s.graphDriverName, config)
	if err != nil {
		return nil, err
	}
	s.graphDriver = driver
	s.graphDriverName = driver.String()
	return driver, nil
}

func (s *store) GraphDriver() (drivers.Driver, error) {
	s.graphLock.Lock()
	defer s.graphLock.Unlock()
	if s.graphLock.TouchedSince(s.lastLoaded) {
		s.graphDriver = nil
		s.layerStore = nil
		s.lastLoaded = time.Now()
	}
	return s.getGraphDriver()
}

// getLayerStore obtains and returns a handle to the writeable layer store object
// used by the Store.
func (s *store) getLayerStore() (rwLayerStore, error) {
	s.graphLock.Lock()
	defer s.graphLock.Unlock()
	if s.graphLock.TouchedSince(s.lastLoaded) {
		s.graphDriver = nil
		s.layerStore = nil
		s.lastLoaded = time.Now()
	}
	if s.layerStore != nil {
		return s.layerStore, nil
	}
	driver, err := s.getGraphDriver()
	if err != nil {
		return nil, err
	}
	driverPrefix := s.graphDriverName + "-"
	rlpath := filepath.Join(s.runRoot, driverPrefix+"layers")
	if err := os.MkdirAll(rlpath, 0700); err != nil {
		return nil, err
	}
	glpath := filepath.Join(s.graphRoot, driverPrefix+"layers")
	if err := os.MkdirAll(glpath, 0700); err != nil {
		return nil, err
	}
	rls, err := s.newLayerStore(rlpath, glpath, driver)
	if err != nil {
		return nil, err
	}
	s.layerStore = rls
	return s.layerStore, nil
}

// getROLayerStores obtains additional read/only layer store objects used by the
// Store.
func (s *store) getROLayerStores() ([]roLayerStore, error) {
	s.graphLock.Lock()
	defer s.graphLock.Unlock()
	if s.roLayerStores != nil {
		return s.roLayerStores, nil
	}
	driver, err := s.getGraphDriver()
	if err != nil {
		return nil, err
	}
	driverPrefix := s.graphDriverName + "-"
	rlpath := filepath.Join(s.runRoot, driverPrefix+"layers")
	if err := os.MkdirAll(rlpath, 0700); err != nil {
		return nil, err
	}
	for _, store := range driver.AdditionalImageStores() {
		glpath := filepath.Join(store, driverPrefix+"layers")
		rls, err := newROLayerStore(rlpath, glpath, driver)
		if err != nil {
			return nil, err
		}
		s.roLayerStores = append(s.roLayerStores, rls)
	}
	return s.roLayerStores, nil
}

// allLayerStores returns a list of all layer store objects used by the Store.
// This is a convenience method for read-only users of the Store.
func (s *store) allLayerStores() ([]roLayerStore, error) {
	primary, err := s.getLayerStore()
	if err != nil {
		return nil, fmt.Errorf("loading primary layer store data: %w", err)
	}
	additional, err := s.getROLayerStores()
	if err != nil {
		return nil, fmt.Errorf("loading additional layer stores: %w", err)
	}
	return append([]roLayerStore{primary}, additional...), nil
}

// readAllLayerStores processes allLayerStores() in order:
// It locks the store for reading, checks for updates, and calls
//
//	(done, err) := fn(store)
//
// until the callback returns done == true, and returns the data from the callback.
//
// If reading any layer store fails, it immediately returns (true, err).
//
// If all layer stores are processed without setting done == true, it returns (false, nil).
//
// Typical usage:
//
//	var res T = failureValue
//	if done, err := s.readAllLayerStores(store, func(…) {
//		…
//	}; done {
//		return res, err
//	}
func (s *store) readAllLayerStores(fn func(store roLayerStore) (bool, error)) (bool, error) {
	layerStores, err := s.allLayerStores()
	if err != nil {
		return true, err
	}
	for _, s := range layerStores {
		store := s
		if err := store.startReading(); err != nil {
			return true, err
		}
		defer store.stopReading()
		if done, err := fn(store); done {
			return true, err
		}
	}
	return false, nil
}

// writeToLayerStore is a helper for working with store.getLayerStore():
// It locks the store for writing, checks for updates, and calls fn()
// It returns the return value of fn, or its own error initializing the store.
func (s *store) writeToLayerStore(fn func(store rwLayerStore) error) error {
	store, err := s.getLayerStore()
	if err != nil {
		return err
	}

	if err := store.startWriting(); err != nil {
		return err
	}
	defer store.stopWriting()
	return fn(store)
}

// getImageStore obtains and returns a handle to the writable image store object
// used by the Store.
func (s *store) getImageStore() (rwImageStore, error) {
	if s.imageStore != nil {
		return s.imageStore, nil
	}
	return nil, ErrLoadError
}

// getROImageStores obtains additional read/only image store objects used by the
// Store.
func (s *store) getROImageStores() ([]roImageStore, error) {
	if s.imageStore == nil {
		return nil, ErrLoadError
	}

	return s.roImageStores, nil
}

// allImageStores returns a list of all image store objects used by the Store.
// This is a convenience method for read-only users of the Store.
func (s *store) allImageStores() ([]roImageStore, error) {
	primary, err := s.getImageStore()
	if err != nil {
		return nil, fmt.Errorf("loading primary image store data: %w", err)
	}
	additional, err := s.getROImageStores()
	if err != nil {
		return nil, fmt.Errorf("loading additional image stores: %w", err)
	}
	return append([]roImageStore{primary}, additional...), nil
}

// readAllImageStores processes allImageStores() in order:
// It locks the store for reading, checks for updates, and calls
//
//	(done, err) := fn(store)
//
// until the callback returns done == true, and returns the data from the callback.
//
// If reading any Image store fails, it immediately returns (true, err).
//
// If all Image stores are processed without setting done == true, it returns (false, nil).
//
// Typical usage:
//
//	var res T = failureValue
//	if done, err := s.readAllImageStores(store, func(…) {
//		…
//	}; done {
//		return res, err
//	}
func (s *store) readAllImageStores(fn func(store roImageStore) (bool, error)) (bool, error) {
	ImageStores, err := s.allImageStores()
	if err != nil {
		return true, err
	}
	for _, s := range ImageStores {
		store := s
		if err := store.startReading(); err != nil {
			return true, err
		}
		defer store.stopReading()
		if done, err := fn(store); done {
			return true, err
		}
	}
	return false, nil
}

// writeToImageStore is a convenience helper for working with store.getImageStore():
// It locks the store for writing, checks for updates, and calls fn()
// It returns the return value of fn, or its own error initializing the store.
func (s *store) writeToImageStore(fn func(store rwImageStore) error) error {
	store, err := s.getImageStore()
	if err != nil {
		return err
	}

	if err := store.startWriting(); err != nil {
		return err
	}
	defer store.stopWriting()
	return fn(store)
}

// getContainerStore obtains and returns a handle to the container store object
// used by the Store.
func (s *store) getContainerStore() (rwContainerStore, error) {
	if s.containerStore != nil {
		return s.containerStore, nil
	}
	return nil, ErrLoadError
}

// writeToContainerStore is a convenience helper for working with store.getContainerStore():
// It locks the store for writing, checks for updates, and calls fn()
// It returns the return value of fn, or its own error initializing the store.
func (s *store) writeToContainerStore(fn func(store rwContainerStore) error) error {
	store, err := s.getContainerStore()
	if err != nil {
		return err
	}

	if err := store.startWriting(); err != nil {
		return err
	}
	defer store.stopWriting()
	return fn(store)
}

// writeToAllStores is a convenience helper for writing to all three stores:
// It locks the stores for writing, checks for updates, and calls fn().
// It returns the return value of fn, or its own error initializing the stores.
func (s *store) writeToAllStores(fn func(rlstore rwLayerStore, ristore rwImageStore, rcstore rwContainerStore) error) error {
	rlstore, err := s.getLayerStore()
	if err != nil {
		return err
	}
	ristore, err := s.getImageStore()
	if err != nil {
		return err
	}
	rcstore, err := s.getContainerStore()
	if err != nil {
		return err
	}

	if err := rlstore.startWriting(); err != nil {
		return err
	}
	defer rlstore.stopWriting()
	if err := ristore.startWriting(); err != nil {
		return err
	}
	defer ristore.stopWriting()
	if err := rcstore.startWriting(); err != nil {
		return err
	}
	defer rcstore.stopWriting()

	return fn(rlstore, ristore, rcstore)
}

func (s *store) canUseShifting(uidmap, gidmap []idtools.IDMap) bool {
	if s.graphDriver == nil || !s.graphDriver.SupportsShifting() {
		return false
	}
	if uidmap != nil && !idtools.IsContiguous(uidmap) {
		return false
	}
	if gidmap != nil && !idtools.IsContiguous(gidmap) {
		return false
	}
	return true
}

func (s *store) PutLayer(id, parent string, names []string, mountLabel string, writeable bool, options *LayerOptions, diff io.Reader) (*Layer, int64, error) {
	var parentLayer *Layer
	rlstore, err := s.getLayerStore()
	if err != nil {
		return nil, -1, err
	}
	rlstores, err := s.getROLayerStores()
	if err != nil {
		return nil, -1, err
	}
	rcstore, err := s.getContainerStore()
	if err != nil {
		return nil, -1, err
	}
	if err := rlstore.startWriting(); err != nil {
		return nil, -1, err
	}
	defer rlstore.stopWriting()
	if err := rcstore.startWriting(); err != nil {
		return nil, -1, err
	}
	defer rcstore.stopWriting()
	if options == nil {
		options = &LayerOptions{}
	}
	if options.HostUIDMapping {
		options.UIDMap = nil
	}
	if options.HostGIDMapping {
		options.GIDMap = nil
	}
	uidMap := options.UIDMap
	gidMap := options.GIDMap
	if parent != "" {
		var ilayer *Layer
		for _, l := range append([]roLayerStore{rlstore}, rlstores...) {
			lstore := l
			if lstore != rlstore {
				if err := lstore.startReading(); err != nil {
					return nil, -1, err
				}
				defer lstore.stopReading()
			}
			if l, err := lstore.Get(parent); err == nil && l != nil {
				ilayer = l
				parent = ilayer.ID
				break
			}
		}
		if ilayer == nil {
			return nil, -1, ErrLayerUnknown
		}
		parentLayer = ilayer
		containers, err := rcstore.Containers()
		if err != nil {
			return nil, -1, err
		}
		for _, container := range containers {
			if container.LayerID == parent {
				return nil, -1, ErrParentIsContainer
			}
		}
		if !options.HostUIDMapping && len(options.UIDMap) == 0 {
			uidMap = ilayer.UIDMap
		}
		if !options.HostGIDMapping && len(options.GIDMap) == 0 {
			gidMap = ilayer.GIDMap
		}
	} else {
		if !options.HostUIDMapping && len(options.UIDMap) == 0 {
			uidMap = s.uidMap
		}
		if !options.HostGIDMapping && len(options.GIDMap) == 0 {
			gidMap = s.gidMap
		}
	}
	layerOptions := LayerOptions{
		OriginalDigest:     options.OriginalDigest,
		UncompressedDigest: options.UncompressedDigest,
	}
	if s.canUseShifting(uidMap, gidMap) {
		layerOptions.IDMappingOptions = types.IDMappingOptions{HostUIDMapping: true, HostGIDMapping: true, UIDMap: nil, GIDMap: nil}
	} else {
		layerOptions.IDMappingOptions = types.IDMappingOptions{
			HostUIDMapping: options.HostUIDMapping,
			HostGIDMapping: options.HostGIDMapping,
			UIDMap:         copyIDMap(uidMap),
			GIDMap:         copyIDMap(gidMap),
		}
	}
	return rlstore.Put(id, parentLayer, names, mountLabel, nil, &layerOptions, writeable, nil, diff)
}

func (s *store) CreateLayer(id, parent string, names []string, mountLabel string, writeable bool, options *LayerOptions) (*Layer, error) {
	layer, _, err := s.PutLayer(id, parent, names, mountLabel, writeable, options, nil)
	return layer, err
}

func (s *store) CreateImage(id string, names []string, layer, metadata string, options *ImageOptions) (*Image, error) {
	if layer != "" {
		layerStores, err := s.allLayerStores()
		if err != nil {
			return nil, err
		}
		var ilayer *Layer
		for _, s := range layerStores {
			store := s
			if err := store.startReading(); err != nil {
				return nil, err
			}
			defer store.stopReading()
			ilayer, err = store.Get(layer)
			if err == nil {
				break
			}
		}
		if ilayer == nil {
			return nil, ErrLayerUnknown
		}
		layer = ilayer.ID
	}

	var res *Image
	err := s.writeToImageStore(func(ristore rwImageStore) error {
		creationDate := time.Now().UTC()
		if options != nil && !options.CreationDate.IsZero() {
			creationDate = options.CreationDate
		}

		var err error
		res, err = ristore.Create(id, names, layer, metadata, creationDate, options.Digest)
		return err
	})
	return res, err
}

// imageTopLayerForMapping does ???
// On entry:
// - ristore must be locked EITHER for reading or writing
// - rlstore must be locked for writing
// - lstores must all be locked for reading
func (s *store) imageTopLayerForMapping(image *Image, ristore roImageStore, createMappedLayer bool, rlstore rwLayerStore, lstores []roLayerStore, options types.IDMappingOptions) (*Layer, error) {
	layerMatchesMappingOptions := func(layer *Layer, options types.IDMappingOptions) bool {
		// If the driver supports shifting and the layer has no mappings, we can use it.
		if s.canUseShifting(options.UIDMap, options.GIDMap) && len(layer.UIDMap) == 0 && len(layer.GIDMap) == 0 {
			return true
		}
		// If we want host mapping, and the layer uses mappings, it's not the best match.
		if options.HostUIDMapping && len(layer.UIDMap) != 0 {
			return false
		}
		if options.HostGIDMapping && len(layer.GIDMap) != 0 {
			return false
		}
		// Compare the maps.
		return reflect.DeepEqual(layer.UIDMap, options.UIDMap) && reflect.DeepEqual(layer.GIDMap, options.GIDMap)
	}
	var layer, parentLayer *Layer
	allStores := append([]roLayerStore{rlstore}, lstores...)
	// Locate the image's top layer and its parent, if it has one.
	for _, s := range allStores {
		store := s
		// Walk the top layer list.
		for _, candidate := range append([]string{image.TopLayer}, image.MappedTopLayers...) {
			if cLayer, err := store.Get(candidate); err == nil {
				// We want the layer's parent, too, if it has one.
				var cParentLayer *Layer
				if cLayer.Parent != "" {
					// Its parent should be in one of the stores, somewhere.
					for _, ps := range allStores {
						if cParentLayer, err = ps.Get(cLayer.Parent); err == nil {
							break
						}
					}
					if cParentLayer == nil {
						continue
					}
				}
				// If the layer matches the desired mappings, it's a perfect match,
				// so we're actually done here.
				if layerMatchesMappingOptions(cLayer, options) {
					return cLayer, nil
				}
				// Record the first one that we found, even if it's not ideal, so that
				// we have a starting point.
				if layer == nil {
					layer = cLayer
					parentLayer = cParentLayer
					if store != rlstore {
						// The layer is in another store, so we cannot
						// create a mapped version of it to the image.
						createMappedLayer = false
					}
				}
			}
		}
	}
	if layer == nil {
		return nil, ErrLayerUnknown
	}
	// The top layer's mappings don't match the ones we want, but it's in a read-only
	// image store, so we can't create and add a mapped copy of the layer to the image.
	// We'll have to do the mapping for the container itself, elsewhere.
	if !createMappedLayer {
		return layer, nil
	}
	// The top layer's mappings don't match the ones we want, and it's in an image store
	// that lets us edit image metadata...
	if istore, ok := ristore.(*imageStore); ok {
		// ... so create a duplicate of the layer with the desired mappings, and
		// register it as an alternate top layer in the image.
		var layerOptions LayerOptions
		if s.canUseShifting(options.UIDMap, options.GIDMap) {
			layerOptions = LayerOptions{
				IDMappingOptions: types.IDMappingOptions{
					HostUIDMapping: true,
					HostGIDMapping: true,
					UIDMap:         nil,
					GIDMap:         nil,
				},
			}
		} else {
			layerOptions = LayerOptions{
				IDMappingOptions: types.IDMappingOptions{
					HostUIDMapping: options.HostUIDMapping,
					HostGIDMapping: options.HostGIDMapping,
					UIDMap:         copyIDMap(options.UIDMap),
					GIDMap:         copyIDMap(options.GIDMap),
				},
			}
		}
		layerOptions.TemplateLayer = layer.ID
		mappedLayer, _, err := rlstore.Put("", parentLayer, nil, layer.MountLabel, nil, &layerOptions, false, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("creating an ID-mapped copy of layer %q: %w", layer.ID, err)
		}
		if err = istore.addMappedTopLayer(image.ID, mappedLayer.ID); err != nil {
			if err2 := rlstore.Delete(mappedLayer.ID); err2 != nil {
				err = fmt.Errorf("deleting layer %q: %v: %w", mappedLayer.ID, err2, err)
			}
			return nil, fmt.Errorf("registering ID-mapped layer with image %q: %w", image.ID, err)
		}
		layer = mappedLayer
	}
	return layer, nil
}

func (s *store) CreateContainer(id string, names []string, image, layer, metadata string, options *ContainerOptions) (*Container, error) {
	if options == nil {
		options = &ContainerOptions{}
	}
	if options.HostUIDMapping {
		options.UIDMap = nil
	}
	if options.HostGIDMapping {
		options.GIDMap = nil
	}
	rlstore, err := s.getLayerStore()
	if err != nil {
		return nil, err
	}

	var imageTopLayer *Layer
	imageID := ""

	if options.AutoUserNs || options.UIDMap != nil || options.GIDMap != nil {
		// Prevent multiple instances to retrieve the same range when AutoUserNs
		// are used.
		// It doesn't prevent containers that specify an explicit mapping to overlap
		// with AutoUserNs.
		s.usernsLock.Lock()
		defer s.usernsLock.Unlock()
	}

	var imageHomeStore roImageStore // Set if image != ""
	var istore rwImageStore         // Set, and locked read-write, if image != ""
	var istores []roImageStore      // Set, and NOT NECESSARILY ALL locked read-only, if image != ""
	var lstores []roLayerStore      // Set, and locked read-only, if image != ""
	var cimage *Image               // Set if image != ""
	if image != "" {
		var err error
		lstores, err = s.getROLayerStores()
		if err != nil {
			return nil, err
		}
		istore, err = s.getImageStore()
		if err != nil {
			return nil, err
		}
		istores, err = s.getROImageStores()
		if err != nil {
			return nil, err
		}
		if err := rlstore.startWriting(); err != nil {
			return nil, err
		}
		defer rlstore.stopWriting()
		for _, s := range lstores {
			store := s
			if err := store.startReading(); err != nil {
				return nil, err
			}
			defer store.stopReading()
		}
		if err := istore.startWriting(); err != nil {
			return nil, err
		}
		defer istore.stopWriting()
		cimage, err = istore.Get(image)
		if err == nil {
			imageHomeStore = istore
		} else {
			for _, s := range istores {
				store := s
				if err := store.startReading(); err != nil {
					return nil, err
				}
				defer store.stopReading()
				cimage, err = store.Get(image)
				if err == nil {
					imageHomeStore = store
					break
				}
			}
		}
		if cimage == nil {
			return nil, fmt.Errorf("locating image with ID %q: %w", image, ErrImageUnknown)
		}
		imageID = cimage.ID
	}

	if options.AutoUserNs {
		var err error
		options.UIDMap, options.GIDMap, err = s.getAutoUserNS(&options.AutoUserNsOpts, cimage, rlstore, lstores)
		if err != nil {
			return nil, err
		}
	}

	uidMap := options.UIDMap
	gidMap := options.GIDMap

	idMappingsOptions := options.IDMappingOptions
	if image != "" {
		if cimage.TopLayer != "" {
			createMappedLayer := imageHomeStore == istore
			ilayer, err := s.imageTopLayerForMapping(cimage, imageHomeStore, createMappedLayer, rlstore, lstores, idMappingsOptions)
			if err != nil {
				return nil, err
			}
			imageTopLayer = ilayer

			if !options.HostUIDMapping && len(options.UIDMap) == 0 {
				uidMap = ilayer.UIDMap
			}
			if !options.HostGIDMapping && len(options.GIDMap) == 0 {
				gidMap = ilayer.GIDMap
			}
		}
	} else {
		if err := rlstore.startWriting(); err != nil {
			return nil, err
		}
		defer rlstore.stopWriting()
		if !options.HostUIDMapping && len(options.UIDMap) == 0 {
			uidMap = s.uidMap
		}
		if !options.HostGIDMapping && len(options.GIDMap) == 0 {
			gidMap = s.gidMap
		}
	}
	var layerOptions *LayerOptions
	if s.canUseShifting(uidMap, gidMap) {
		layerOptions = &LayerOptions{
			IDMappingOptions: types.IDMappingOptions{
				HostUIDMapping: true,
				HostGIDMapping: true,
				UIDMap:         nil,
				GIDMap:         nil,
			},
		}
	} else {
		layerOptions = &LayerOptions{
			IDMappingOptions: types.IDMappingOptions{
				HostUIDMapping: idMappingsOptions.HostUIDMapping,
				HostGIDMapping: idMappingsOptions.HostGIDMapping,
				UIDMap:         copyIDMap(uidMap),
				GIDMap:         copyIDMap(gidMap),
			},
		}
	}
	if options.Flags == nil {
		options.Flags = make(map[string]interface{})
	}
	plabel, _ := options.Flags[processLabelFlag].(string)
	mlabel, _ := options.Flags[mountLabelFlag].(string)
	if (plabel == "" && mlabel != "") || (plabel != "" && mlabel == "") {
		return nil, errors.New("ProcessLabel and Mountlabel must either not be specified or both specified")
	}

	if plabel == "" {
		processLabel, mountLabel, err := label.InitLabels(options.LabelOpts)
		if err != nil {
			return nil, err
		}
		mlabel = mountLabel
		options.Flags[processLabelFlag] = processLabel
		options.Flags[mountLabelFlag] = mountLabel
	}

	clayer, err := rlstore.Create(layer, imageTopLayer, nil, mlabel, options.StorageOpt, layerOptions, true)
	if err != nil {
		return nil, err
	}
	layer = clayer.ID

	var container *Container
	err = s.writeToContainerStore(func(rcstore rwContainerStore) error {
		options.IDMappingOptions = types.IDMappingOptions{
			HostUIDMapping: len(options.UIDMap) == 0,
			HostGIDMapping: len(options.GIDMap) == 0,
			UIDMap:         copyIDMap(options.UIDMap),
			GIDMap:         copyIDMap(options.GIDMap),
		}
		var err error
		container, err = rcstore.Create(id, names, imageID, layer, metadata, options)
		if err != nil || container == nil {
			if err2 := rlstore.Delete(layer); err2 != nil {
				if err == nil {
					err = fmt.Errorf("deleting layer %#v: %w", layer, err2)
				} else {
					logrus.Errorf("While recovering from a failure to create a container, error deleting layer %#v: %v", layer, err2)
				}
			}
		}
		return err
	})
	return container, err
}

func (s *store) SetMetadata(id, metadata string) error {
	return s.writeToAllStores(func(rlstore rwLayerStore, ristore rwImageStore, rcstore rwContainerStore) error {
		if rlstore.Exists(id) {
			return rlstore.SetMetadata(id, metadata)
		}
		if ristore.Exists(id) {
			return ristore.SetMetadata(id, metadata)
		}
		if rcstore.Exists(id) {
			return rcstore.SetMetadata(id, metadata)
		}
		return ErrNotAnID
	})
}

func (s *store) Metadata(id string) (string, error) {
	var res string

	if done, err := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		if store.Exists(id) {
			var err error
			res, err = store.Metadata(id)
			return true, err
		}
		return false, nil
	}); done {
		return res, err
	}

	if done, err := s.readAllImageStores(func(store roImageStore) (bool, error) {
		if store.Exists(id) {
			var err error
			res, err = store.Metadata(id)
			return true, err
		}
		return false, nil
	}); done {
		return res, err
	}

	cstore, err := s.getContainerStore()
	if err != nil {
		return "", err
	}
	if err := cstore.startReading(); err != nil {
		return "", err
	}
	defer cstore.stopReading()
	if cstore.Exists(id) {
		return cstore.Metadata(id)
	}
	return "", ErrNotAnID
}

func (s *store) ListImageBigData(id string) ([]string, error) {
	var res []string
	if done, err := s.readAllImageStores(func(store roImageStore) (bool, error) {
		bigDataNames, err := store.BigDataNames(id)
		if err == nil {
			res = bigDataNames
			return true, nil
		}
		return false, nil
	}); done {
		return res, err
	}
	return nil, fmt.Errorf("locating image with ID %q: %w", id, ErrImageUnknown)
}

func (s *store) ImageBigDataSize(id, key string) (int64, error) {
	var res int64 = -1
	if done, err := s.readAllImageStores(func(store roImageStore) (bool, error) {
		size, err := store.BigDataSize(id, key)
		if err == nil {
			res = size
			return true, nil
		}
		return false, nil
	}); done {
		return res, err
	}
	return -1, ErrSizeUnknown
}

func (s *store) ImageBigDataDigest(id, key string) (digest.Digest, error) {
	var res digest.Digest
	if done, err := s.readAllImageStores(func(ristore roImageStore) (bool, error) {
		d, err := ristore.BigDataDigest(id, key)
		if err == nil && d.Validate() == nil {
			res = d
			return true, nil
		}
		return false, nil
	}); done {
		return res, err
	}
	return "", ErrDigestUnknown
}

func (s *store) ImageBigData(id, key string) ([]byte, error) {
	foundImage := false
	var res []byte
	if done, err := s.readAllImageStores(func(store roImageStore) (bool, error) {
		data, err := store.BigData(id, key)
		if err == nil {
			res = data
			return true, nil
		}
		if store.Exists(id) {
			foundImage = true
		}
		return false, nil
	}); done {
		return res, err
	}
	if foundImage {
		return nil, fmt.Errorf("locating item named %q for image with ID %q (consider removing the image to resolve the issue): %w", key, id, os.ErrNotExist)
	}
	return nil, fmt.Errorf("locating image with ID %q: %w", id, ErrImageUnknown)
}

// ListLayerBigData retrieves a list of the (possibly large) chunks of
// named data associated with an layer.
func (s *store) ListLayerBigData(id string) ([]string, error) {
	foundLayer := false
	var res []string
	if done, err := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		data, err := store.BigDataNames(id)
		if err == nil {
			res = data
			return true, nil
		}
		if store.Exists(id) {
			foundLayer = true
		}
		return false, nil
	}); done {
		return res, err
	}
	if foundLayer {
		return nil, fmt.Errorf("locating big data for layer with ID %q: %w", id, os.ErrNotExist)
	}
	return nil, fmt.Errorf("locating layer with ID %q: %w", id, ErrLayerUnknown)
}

// LayerBigData retrieves a (possibly large) chunk of named data
// associated with a layer.
func (s *store) LayerBigData(id, key string) (io.ReadCloser, error) {
	foundLayer := false
	var res io.ReadCloser
	if done, err := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		data, err := store.BigData(id, key)
		if err == nil {
			res = data
			return true, nil
		}
		if store.Exists(id) {
			foundLayer = true
		}
		return false, nil
	}); done {
		return res, err
	}
	if foundLayer {
		return nil, fmt.Errorf("locating item named %q for layer with ID %q: %w", key, id, os.ErrNotExist)
	}
	return nil, fmt.Errorf("locating layer with ID %q: %w", id, ErrLayerUnknown)
}

// SetLayerBigData stores a (possibly large) chunk of named data
// associated with a layer.
func (s *store) SetLayerBigData(id, key string, data io.Reader) error {
	return s.writeToLayerStore(func(store rwLayerStore) error {
		return store.SetBigData(id, key, data)
	})
}

func (s *store) SetImageBigData(id, key string, data []byte, digestManifest func([]byte) (digest.Digest, error)) error {
	return s.writeToImageStore(func(ristore rwImageStore) error {
		return ristore.SetBigData(id, key, data, digestManifest)
	})
}

func (s *store) ImageSize(id string) (int64, error) {
	layerStores, err := s.allLayerStores()
	if err != nil {
		return -1, err
	}
	for _, s := range layerStores {
		store := s
		if err := store.startReading(); err != nil {
			return -1, err
		}
		defer store.stopReading()
	}

	imageStores, err := s.allImageStores()
	if err != nil {
		return -1, err
	}
	// Look for the image's record.
	var imageStore roBigDataStore
	var image *Image
	for _, s := range imageStores {
		store := s
		if err := store.startReading(); err != nil {
			return -1, err
		}
		defer store.stopReading()
		if image, err = store.Get(id); err == nil {
			imageStore = store
			break
		}
	}
	if image == nil {
		return -1, fmt.Errorf("locating image with ID %q: %w", id, ErrImageUnknown)
	}

	// Start with a list of the image's top layers, if it has any.
	queue := make(map[string]struct{})
	for _, layerID := range append([]string{image.TopLayer}, image.MappedTopLayers...) {
		if layerID != "" {
			queue[layerID] = struct{}{}
		}
	}
	visited := make(map[string]struct{})
	// Walk all of the layers.
	var size int64
	for len(visited) < len(queue) {
		for layerID := range queue {
			// Visit each layer only once.
			if _, ok := visited[layerID]; ok {
				continue
			}
			visited[layerID] = struct{}{}
			// Look for the layer and the store that knows about it.
			var layerStore roLayerStore
			var layer *Layer
			for _, store := range layerStores {
				if layer, err = store.Get(layerID); err == nil {
					layerStore = store
					break
				}
			}
			if layer == nil {
				return -1, fmt.Errorf("locating layer with ID %q: %w", layerID, ErrLayerUnknown)
			}
			// The UncompressedSize is only valid if there's a digest to go with it.
			n := layer.UncompressedSize
			if layer.UncompressedDigest == "" {
				// Compute the size.
				n, err = layerStore.DiffSize("", layer.ID)
				if err != nil {
					return -1, fmt.Errorf("size/digest of layer with ID %q could not be calculated: %w", layerID, err)
				}
			}
			// Count this layer.
			size += n
			// Make a note to visit the layer's parent if we haven't already.
			if layer.Parent != "" {
				queue[layer.Parent] = struct{}{}
			}
		}
	}

	// Count big data items.
	names, err := imageStore.BigDataNames(id)
	if err != nil {
		return -1, fmt.Errorf("reading list of big data items for image %q: %w", id, err)
	}
	for _, name := range names {
		n, err := imageStore.BigDataSize(id, name)
		if err != nil {
			return -1, fmt.Errorf("reading size of big data item %q for image %q: %w", name, id, err)
		}
		size += n
	}

	return size, nil
}

func (s *store) ContainerSize(id string) (int64, error) {
	layerStores, err := s.allLayerStores()
	if err != nil {
		return -1, err
	}
	for _, s := range layerStores {
		store := s
		if err := store.startReading(); err != nil {
			return -1, err
		}
		defer store.stopReading()
	}

	// Get the location of the container directory and container run directory.
	// Do it before we lock the container store because they do, too.
	cdir, err := s.ContainerDirectory(id)
	if err != nil {
		return -1, err
	}
	rdir, err := s.ContainerRunDirectory(id)
	if err != nil {
		return -1, err
	}

	rcstore, err := s.getContainerStore()
	if err != nil {
		return -1, err
	}
	if err := rcstore.startReading(); err != nil {
		return -1, err
	}
	defer rcstore.stopReading()

	// Read the container record.
	container, err := rcstore.Get(id)
	if err != nil {
		return -1, err
	}

	// Read the container's layer's size.
	var layer *Layer
	var size int64
	for _, store := range layerStores {
		if layer, err = store.Get(container.LayerID); err == nil {
			size, err = store.DiffSize("", layer.ID)
			if err != nil {
				return -1, fmt.Errorf("determining size of layer with ID %q: %w", layer.ID, err)
			}
			break
		}
	}
	if layer == nil {
		return -1, fmt.Errorf("locating layer with ID %q: %w", container.LayerID, ErrLayerUnknown)
	}

	// Count big data items.
	names, err := rcstore.BigDataNames(id)
	if err != nil {
		return -1, fmt.Errorf("reading list of big data items for container %q: %w", container.ID, err)
	}
	for _, name := range names {
		n, err := rcstore.BigDataSize(id, name)
		if err != nil {
			return -1, fmt.Errorf("reading size of big data item %q for container %q: %w", name, id, err)
		}
		size += n
	}

	// Count the size of our container directory and container run directory.
	n, err := directory.Size(cdir)
	if err != nil {
		return -1, err
	}
	size += n
	n, err = directory.Size(rdir)
	if err != nil {
		return -1, err
	}
	size += n

	return size, nil
}

func (s *store) ListContainerBigData(id string) ([]string, error) {
	rcstore, err := s.getContainerStore()
	if err != nil {
		return nil, err
	}

	if err := rcstore.startReading(); err != nil {
		return nil, err
	}
	defer rcstore.stopReading()

	return rcstore.BigDataNames(id)
}

func (s *store) ContainerBigDataSize(id, key string) (int64, error) {
	rcstore, err := s.getContainerStore()
	if err != nil {
		return -1, err
	}
	if err := rcstore.startReading(); err != nil {
		return -1, err
	}
	defer rcstore.stopReading()
	return rcstore.BigDataSize(id, key)
}

func (s *store) ContainerBigDataDigest(id, key string) (digest.Digest, error) {
	rcstore, err := s.getContainerStore()
	if err != nil {
		return "", err
	}
	if err := rcstore.startReading(); err != nil {
		return "", err
	}
	defer rcstore.stopReading()
	return rcstore.BigDataDigest(id, key)
}

func (s *store) ContainerBigData(id, key string) ([]byte, error) {
	rcstore, err := s.getContainerStore()
	if err != nil {
		return nil, err
	}
	if err := rcstore.startReading(); err != nil {
		return nil, err
	}
	defer rcstore.stopReading()
	return rcstore.BigData(id, key)
}

func (s *store) SetContainerBigData(id, key string, data []byte) error {
	return s.writeToContainerStore(func(rcstore rwContainerStore) error {
		return rcstore.SetBigData(id, key, data)
	})
}

func (s *store) Exists(id string) bool {
	var res = false

	if done, _ := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		if store.Exists(id) {
			res = true
			return true, nil
		}
		return false, nil
	}); done {
		return res
	}

	if done, _ := s.readAllImageStores(func(store roImageStore) (bool, error) {
		if store.Exists(id) {
			res = true
			return true, nil
		}
		return false, nil
	}); done {
		return res
	}

	rcstore, err := s.getContainerStore()
	if err != nil {
		return false
	}
	if err := rcstore.startReading(); err != nil {
		return false
	}
	defer rcstore.stopReading()
	return rcstore.Exists(id)
}

func dedupeNames(names []string) []string {
	seen := make(map[string]bool)
	deduped := make([]string, 0, len(names))
	for _, name := range names {
		if _, wasSeen := seen[name]; !wasSeen {
			seen[name] = true
			deduped = append(deduped, name)
		}
	}
	return deduped
}

// Deprecated: Prone to race conditions, suggested alternatives are `AddNames` and `RemoveNames`.
func (s *store) SetNames(id string, names []string) error {
	return s.updateNames(id, names, setNames)
}

func (s *store) AddNames(id string, names []string) error {
	return s.updateNames(id, names, addNames)
}

func (s *store) RemoveNames(id string, names []string) error {
	return s.updateNames(id, names, removeNames)
}

func (s *store) updateNames(id string, names []string, op updateNameOperation) error {
	deduped := dedupeNames(names)

	layerFound := false
	if err := s.writeToLayerStore(func(rlstore rwLayerStore) error {
		if !rlstore.Exists(id) {
			return nil
		}
		layerFound = true
		return rlstore.updateNames(id, deduped, op)
	}); err != nil || layerFound {
		return err
	}

	ristore, err := s.getImageStore()
	if err != nil {
		return err
	}
	if err := ristore.startWriting(); err != nil {
		return err
	}
	defer ristore.stopWriting()
	if ristore.Exists(id) {
		return ristore.updateNames(id, deduped, op)
	}

	// Check is id refers to a RO Store
	ristores, err := s.getROImageStores()
	if err != nil {
		return err
	}
	for _, s := range ristores {
		store := s
		if err := store.startReading(); err != nil {
			return err
		}
		defer store.stopReading()
		if i, err := store.Get(id); err == nil {
			if len(deduped) > 1 {
				// Do not want to create image name in R/W storage
				deduped = deduped[1:]
			}
			_, err := ristore.Create(id, deduped, i.TopLayer, i.Metadata, i.Created, i.Digest)
			return err
		}
	}

	containerFound := false
	if err := s.writeToContainerStore(func(rcstore rwContainerStore) error {
		if !rcstore.Exists(id) {
			return nil
		}
		containerFound = true
		return rcstore.updateNames(id, deduped, op)
	}); err != nil || containerFound {
		return err
	}

	return ErrLayerUnknown
}

func (s *store) Names(id string) ([]string, error) {
	var res []string

	if done, err := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		if l, err := store.Get(id); l != nil && err == nil {
			res = l.Names
			return true, nil
		}
		return false, nil
	}); done {
		return res, err
	}

	if done, err := s.readAllImageStores(func(store roImageStore) (bool, error) {
		if i, err := store.Get(id); i != nil && err == nil {
			res = i.Names
			return true, nil
		}
		return false, nil
	}); done {
		return res, err
	}

	rcstore, err := s.getContainerStore()
	if err != nil {
		return nil, err
	}
	if err := rcstore.startReading(); err != nil {
		return nil, err
	}
	defer rcstore.stopReading()
	if c, err := rcstore.Get(id); c != nil && err == nil {
		return c.Names, nil
	}
	return nil, ErrLayerUnknown
}

func (s *store) Lookup(name string) (string, error) {
	var res string

	if done, err := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		if l, err := store.Get(name); l != nil && err == nil {
			res = l.ID
			return true, nil
		}
		return false, nil
	}); done {
		return res, err
	}

	if done, err := s.readAllImageStores(func(store roImageStore) (bool, error) {
		if i, err := store.Get(name); i != nil && err == nil {
			res = i.ID
			return true, nil
		}
		return false, nil
	}); done {
		return res, err
	}

	cstore, err := s.getContainerStore()
	if err != nil {
		return "", err
	}
	if err := cstore.startReading(); err != nil {
		return "", err
	}
	defer cstore.stopReading()
	if c, err := cstore.Get(name); c != nil && err == nil {
		return c.ID, nil
	}

	return "", ErrLayerUnknown
}

func (s *store) DeleteLayer(id string) error {
	return s.writeToAllStores(func(rlstore rwLayerStore, ristore rwImageStore, rcstore rwContainerStore) error {
		if rlstore.Exists(id) {
			if l, err := rlstore.Get(id); err != nil {
				id = l.ID
			}
			layers, err := rlstore.Layers()
			if err != nil {
				return err
			}
			for _, layer := range layers {
				if layer.Parent == id {
					return fmt.Errorf("used by layer %v: %w", layer.ID, ErrLayerHasChildren)
				}
			}
			images, err := ristore.Images()
			if err != nil {
				return err
			}

			for _, image := range images {
				if image.TopLayer == id {
					return fmt.Errorf("layer %v used by image %v: %w", id, image.ID, ErrLayerUsedByImage)
				}
				if stringutils.InSlice(image.MappedTopLayers, id) {
					// No write access to the image store, fail before the layer is deleted
					if _, ok := ristore.(*imageStore); !ok {
						return fmt.Errorf("layer %v used by image %v: %w", id, image.ID, ErrLayerUsedByImage)
					}
				}
			}
			containers, err := rcstore.Containers()
			if err != nil {
				return err
			}
			for _, container := range containers {
				if container.LayerID == id {
					return fmt.Errorf("layer %v used by container %v: %w", id, container.ID, ErrLayerUsedByContainer)
				}
			}
			if err := rlstore.Delete(id); err != nil {
				return fmt.Errorf("delete layer %v: %w", id, err)
			}

			// The check here is used to avoid iterating the images if we don't need to.
			// There is already a check above for the imageStore to be writeable when the layer is part of MappedTopLayers.
			if istore, ok := ristore.(*imageStore); ok {
				for _, image := range images {
					if stringutils.InSlice(image.MappedTopLayers, id) {
						if err = istore.removeMappedTopLayer(image.ID, id); err != nil {
							return fmt.Errorf("remove mapped top layer %v from image %v: %w", id, image.ID, err)
						}
					}
				}
			}
			return nil
		}
		return ErrNotALayer
	})
}

func (s *store) DeleteImage(id string, commit bool) (layers []string, err error) {
	layersToRemove := []string{}
	if err := s.writeToAllStores(func(rlstore rwLayerStore, ristore rwImageStore, rcstore rwContainerStore) error {
		if ristore.Exists(id) {
			image, err := ristore.Get(id)
			if err != nil {
				return err
			}
			id = image.ID
			containers, err := rcstore.Containers()
			if err != nil {
				return err
			}
			aContainerByImage := make(map[string]string)
			for _, container := range containers {
				aContainerByImage[container.ImageID] = container.ID
			}
			if container, ok := aContainerByImage[id]; ok {
				return fmt.Errorf("image used by %v: %w", container, ErrImageUsedByContainer)
			}
			images, err := ristore.Images()
			if err != nil {
				return err
			}
			layers, err := rlstore.Layers()
			if err != nil {
				return err
			}
			childrenByParent := make(map[string][]string)
			for _, layer := range layers {
				childrenByParent[layer.Parent] = append(childrenByParent[layer.Parent], layer.ID)
			}
			otherImagesTopLayers := make(map[string]struct{})
			for _, img := range images {
				if img.ID != id {
					otherImagesTopLayers[img.TopLayer] = struct{}{}
					for _, layerID := range img.MappedTopLayers {
						otherImagesTopLayers[layerID] = struct{}{}
					}
				}
			}
			if commit {
				if err = ristore.Delete(id); err != nil {
					return err
				}
			}
			layer := image.TopLayer
			layersToRemoveMap := make(map[string]struct{})
			layersToRemove = append(layersToRemove, image.MappedTopLayers...)
			for _, mappedTopLayer := range image.MappedTopLayers {
				layersToRemoveMap[mappedTopLayer] = struct{}{}
			}
			for layer != "" {
				if rcstore.Exists(layer) {
					break
				}
				if _, used := otherImagesTopLayers[layer]; used {
					break
				}
				parent := ""
				if l, err := rlstore.Get(layer); err == nil {
					parent = l.Parent
				}
				hasChildrenNotBeingRemoved := func() bool {
					layersToCheck := []string{layer}
					if layer == image.TopLayer {
						layersToCheck = append(layersToCheck, image.MappedTopLayers...)
					}
					for _, layer := range layersToCheck {
						if childList := childrenByParent[layer]; len(childList) > 0 {
							for _, child := range childList {
								if _, childIsSlatedForRemoval := layersToRemoveMap[child]; childIsSlatedForRemoval {
									continue
								}
								return true
							}
						}
					}
					return false
				}
				if hasChildrenNotBeingRemoved() {
					break
				}
				layersToRemove = append(layersToRemove, layer)
				layersToRemoveMap[layer] = struct{}{}
				layer = parent
			}
		} else {
			return ErrNotAnImage
		}
		if commit {
			for _, layer := range layersToRemove {
				if err = rlstore.Delete(layer); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return layersToRemove, nil
}

func (s *store) DeleteContainer(id string) error {
	return s.writeToAllStores(func(rlstore rwLayerStore, ristore rwImageStore, rcstore rwContainerStore) error {
		if !rcstore.Exists(id) {
			return ErrNotAContainer
		}

		container, err := rcstore.Get(id)
		if err != nil {
			return ErrNotAContainer
		}

		errChan := make(chan error)
		var wg sync.WaitGroup

		if rlstore.Exists(container.LayerID) {
			wg.Add(1)
			go func() {
				errChan <- rlstore.Delete(container.LayerID)
				wg.Done()
			}()
		}
		wg.Add(1)
		go func() {
			errChan <- rcstore.Delete(id)
			wg.Done()
		}()

		middleDir := s.graphDriverName + "-containers"
		gcpath := filepath.Join(s.GraphRoot(), middleDir, container.ID)
		wg.Add(1)
		go func() {
			defer wg.Done()
			// attempt a simple rm -rf first
			err := os.RemoveAll(gcpath)
			if err == nil {
				errChan <- nil
				return
			}
			// and if it fails get to the more complicated cleanup
			errChan <- system.EnsureRemoveAll(gcpath)
		}()

		rcpath := filepath.Join(s.RunRoot(), middleDir, container.ID)
		wg.Add(1)
		go func() {
			defer wg.Done()
			// attempt a simple rm -rf first
			err := os.RemoveAll(rcpath)
			if err == nil {
				errChan <- nil
				return
			}
			// and if it fails get to the more complicated cleanup
			errChan <- system.EnsureRemoveAll(rcpath)
		}()

		go func() {
			wg.Wait()
			close(errChan)
		}()

		var errors []error
		for err := range errChan {
			if err != nil {
				errors = append(errors, err)
			}
		}
		return multierror.Append(nil, errors...).ErrorOrNil()
	})
}

func (s *store) Delete(id string) error {
	return s.writeToAllStores(func(rlstore rwLayerStore, ristore rwImageStore, rcstore rwContainerStore) error {
		if rcstore.Exists(id) {
			if container, err := rcstore.Get(id); err == nil {
				if rlstore.Exists(container.LayerID) {
					if err = rlstore.Delete(container.LayerID); err != nil {
						return err
					}
					if err = rcstore.Delete(id); err != nil {
						return err
					}
					middleDir := s.graphDriverName + "-containers"
					gcpath := filepath.Join(s.GraphRoot(), middleDir, container.ID, "userdata")
					if err = os.RemoveAll(gcpath); err != nil {
						return err
					}
					rcpath := filepath.Join(s.RunRoot(), middleDir, container.ID, "userdata")
					if err = os.RemoveAll(rcpath); err != nil {
						return err
					}
					return nil
				}
				return ErrNotALayer
			}
		}
		if ristore.Exists(id) {
			return ristore.Delete(id)
		}
		if rlstore.Exists(id) {
			return rlstore.Delete(id)
		}
		return ErrLayerUnknown
	})
}

func (s *store) Wipe() error {
	return s.writeToAllStores(func(rlstore rwLayerStore, ristore rwImageStore, rcstore rwContainerStore) error {
		if err := rcstore.Wipe(); err != nil {
			return err
		}
		if err := ristore.Wipe(); err != nil {
			return err
		}
		return rlstore.Wipe()
	})
}

func (s *store) Status() ([][2]string, error) {
	rlstore, err := s.getLayerStore()
	if err != nil {
		return nil, err
	}
	return rlstore.Status()
}

func (s *store) Version() ([][2]string, error) {
	return [][2]string{}, nil
}

func (s *store) mount(id string, options drivers.MountOpts) (string, error) {
	rlstore, err := s.getLayerStore()
	if err != nil {
		return "", err
	}

	s.graphLock.Lock()
	defer s.graphLock.Unlock()
	if err := rlstore.startWriting(); err != nil {
		return "", err
	}
	defer rlstore.stopWriting()

	modified, err := s.graphLock.Modified()
	if err != nil {
		return "", err
	}

	/* We need to make sure the home mount is present when the Mount is done.  */
	if modified {
		s.graphDriver = nil
		s.layerStore = nil
		s.graphDriver, err = s.getGraphDriver()
		if err != nil {
			return "", err
		}
		s.lastLoaded = time.Now()
	}

	if options.UidMaps != nil || options.GidMaps != nil {
		options.DisableShifting = !s.canUseShifting(options.UidMaps, options.GidMaps)
	}

	if rlstore.Exists(id) {
		return rlstore.Mount(id, options)
	}
	return "", ErrLayerUnknown
}

func (s *store) MountImage(id string, mountOpts []string, mountLabel string) (string, error) {
	// Append ReadOnly option to mountOptions
	img, err := s.Image(id)
	if err != nil {
		return "", err
	}

	if err := validateMountOptions(mountOpts); err != nil {
		return "", err
	}
	options := drivers.MountOpts{
		MountLabel: mountLabel,
		Options:    append(mountOpts, "ro"),
	}

	return s.mount(img.TopLayer, options)
}

func (s *store) Mount(id, mountLabel string) (string, error) {
	options := drivers.MountOpts{
		MountLabel: mountLabel,
	}
	// check if `id` is a container, then grab the LayerID, uidmap and gidmap, along with
	// otherwise we assume the id is a LayerID and attempt to mount it.
	if container, err := s.Container(id); err == nil {
		id = container.LayerID
		options.UidMaps = container.UIDMap
		options.GidMaps = container.GIDMap
		options.Options = container.MountOpts()
		if !s.disableVolatile {
			if v, found := container.Flags[volatileFlag]; found {
				if b, ok := v.(bool); ok {
					options.Volatile = b
				}
			}
		}
	}
	return s.mount(id, options)
}

func (s *store) Mounted(id string) (int, error) {
	if layerID, err := s.ContainerLayerID(id); err == nil {
		id = layerID
	}
	rlstore, err := s.getLayerStore()
	if err != nil {
		return 0, err
	}
	if err := rlstore.startReading(); err != nil {
		return 0, err
	}
	defer rlstore.stopReading()

	return rlstore.Mounted(id)
}

func (s *store) UnmountImage(id string, force bool) (bool, error) {
	img, err := s.Image(id)
	if err != nil {
		return false, err
	}
	return s.Unmount(img.TopLayer, force)
}

func (s *store) Unmount(id string, force bool) (bool, error) {
	if layerID, err := s.ContainerLayerID(id); err == nil {
		id = layerID
	}
	var res bool
	err := s.writeToLayerStore(func(rlstore rwLayerStore) error {
		if rlstore.Exists(id) {
			var err error
			res, err = rlstore.Unmount(id, force)
			return err
		}
		return ErrLayerUnknown
	})
	return res, err
}

func (s *store) Changes(from, to string) ([]archive.Change, error) {
	var res []archive.Change
	if done, err := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		if store.Exists(to) {
			var err error
			res, err = store.Changes(from, to)
			return true, err
		}
		return false, nil
	}); done {
		return res, err
	}
	return nil, ErrLayerUnknown
}

func (s *store) DiffSize(from, to string) (int64, error) {
	var res int64 = -1
	if done, err := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		if store.Exists(to) {
			var err error
			res, err = store.DiffSize(from, to)
			return true, err
		}
		return false, nil
	}); done {
		return res, err
	}
	return -1, ErrLayerUnknown
}

func (s *store) Diff(from, to string, options *DiffOptions) (io.ReadCloser, error) {
	layerStores, err := s.allLayerStores()
	if err != nil {
		return nil, err
	}

	// NaiveDiff could cause mounts to happen without a lock, so be safe
	// and treat the .Diff operation as a Mount.
	s.graphLock.Lock()
	defer s.graphLock.Unlock()

	modified, err := s.graphLock.Modified()
	if err != nil {
		return nil, err
	}

	// We need to make sure the home mount is present when the Mount is done.
	if modified {
		s.graphDriver = nil
		s.layerStore = nil
		s.graphDriver, err = s.getGraphDriver()
		if err != nil {
			return nil, err
		}
		s.lastLoaded = time.Now()
	}

	for _, s := range layerStores {
		store := s
		if err := store.startReading(); err != nil {
			return nil, err
		}
		if store.Exists(to) {
			rc, err := store.Diff(from, to, options)
			if rc != nil && err == nil {
				wrapped := ioutils.NewReadCloserWrapper(rc, func() error {
					err := rc.Close()
					store.stopReading()
					return err
				})
				return wrapped, nil
			}
			store.stopReading()
			return rc, err
		}
		store.stopReading()
	}
	return nil, ErrLayerUnknown
}

func (s *store) ApplyDiffFromStagingDirectory(to, stagingDirectory string, diffOutput *drivers.DriverWithDifferOutput, options *drivers.ApplyDiffOpts) error {
	return s.writeToLayerStore(func(rlstore rwLayerStore) error {
		if !rlstore.Exists(to) {
			return ErrLayerUnknown
		}
		return rlstore.ApplyDiffFromStagingDirectory(to, stagingDirectory, diffOutput, options)
	})
}

func (s *store) CleanupStagingDirectory(stagingDirectory string) error {
	return s.writeToLayerStore(func(rlstore rwLayerStore) error {
		return rlstore.CleanupStagingDirectory(stagingDirectory)
	})
}

func (s *store) ApplyDiffWithDiffer(to string, options *drivers.ApplyDiffOpts, differ drivers.Differ) (*drivers.DriverWithDifferOutput, error) {
	var res *drivers.DriverWithDifferOutput
	err := s.writeToLayerStore(func(rlstore rwLayerStore) error {
		if to != "" && !rlstore.Exists(to) {
			return ErrLayerUnknown
		}
		var err error
		res, err = rlstore.ApplyDiffWithDiffer(to, options, differ)
		return err
	})
	return res, err
}

func (s *store) DifferTarget(id string) (string, error) {
	var res string
	err := s.writeToLayerStore(func(rlstore rwLayerStore) error {
		if rlstore.Exists(id) {
			var err error
			res, err = rlstore.DifferTarget(id)
			return err
		}
		return ErrLayerUnknown
	})
	return res, err
}

func (s *store) ApplyDiff(to string, diff io.Reader) (int64, error) {
	var res int64 = -1
	err := s.writeToLayerStore(func(rlstore rwLayerStore) error {
		if rlstore.Exists(to) {
			var err error
			res, err = rlstore.ApplyDiff(to, diff)
			return err
		}
		return ErrLayerUnknown
	})
	return res, err
}

func (s *store) layersByMappedDigest(m func(roLayerStore, digest.Digest) ([]Layer, error), d digest.Digest) ([]Layer, error) {
	var layers []Layer
	if _, err := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		storeLayers, err := m(store, d)
		if err != nil {
			if !errors.Is(err, ErrLayerUnknown) {
				return true, err
			}
			return false, nil
		}
		layers = append(layers, storeLayers...)
		return false, nil
	}); err != nil {
		return nil, err
	}
	if len(layers) == 0 {
		return nil, ErrLayerUnknown
	}
	return layers, nil
}

func (s *store) LayersByCompressedDigest(d digest.Digest) ([]Layer, error) {
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("looking for compressed layers matching digest %q: %w", d, err)
	}
	return s.layersByMappedDigest(func(r roLayerStore, d digest.Digest) ([]Layer, error) { return r.LayersByCompressedDigest(d) }, d)
}

func (s *store) LayersByUncompressedDigest(d digest.Digest) ([]Layer, error) {
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("looking for layers matching digest %q: %w", d, err)
	}
	return s.layersByMappedDigest(func(r roLayerStore, d digest.Digest) ([]Layer, error) { return r.LayersByUncompressedDigest(d) }, d)
}

func (s *store) LayerSize(id string) (int64, error) {
	var res int64 = -1
	if done, err := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		if store.Exists(id) {
			var err error
			res, err = store.Size(id)
			return true, err
		}
		return false, nil
	}); done {
		return res, err
	}
	return -1, ErrLayerUnknown
}

func (s *store) LayerParentOwners(id string) ([]int, []int, error) {
	rlstore, err := s.getLayerStore()
	if err != nil {
		return nil, nil, err
	}
	if err := rlstore.startReading(); err != nil {
		return nil, nil, err
	}
	defer rlstore.stopReading()
	if rlstore.Exists(id) {
		return rlstore.ParentOwners(id)
	}
	return nil, nil, ErrLayerUnknown
}

func (s *store) ContainerParentOwners(id string) ([]int, []int, error) {
	rlstore, err := s.getLayerStore()
	if err != nil {
		return nil, nil, err
	}
	rcstore, err := s.getContainerStore()
	if err != nil {
		return nil, nil, err
	}
	if err := rlstore.startReading(); err != nil {
		return nil, nil, err
	}
	defer rlstore.stopReading()
	if err := rcstore.startReading(); err != nil {
		return nil, nil, err
	}
	defer rcstore.stopReading()
	container, err := rcstore.Get(id)
	if err != nil {
		return nil, nil, err
	}
	if rlstore.Exists(container.LayerID) {
		return rlstore.ParentOwners(container.LayerID)
	}
	return nil, nil, ErrLayerUnknown
}

func (s *store) Layers() ([]Layer, error) {
	var layers []Layer
	if done, err := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		storeLayers, err := store.Layers()
		if err != nil {
			return true, err
		}
		layers = append(layers, storeLayers...)
		return false, nil
	}); done {
		return nil, err
	}
	return layers, nil
}

func (s *store) Images() ([]Image, error) {
	var images []Image
	if _, err := s.readAllImageStores(func(store roImageStore) (bool, error) {
		storeImages, err := store.Images()
		if err != nil {
			return true, err
		}
		images = append(images, storeImages...)
		return false, nil
	}); err != nil {
		return nil, err
	}
	return images, nil
}

func (s *store) Containers() ([]Container, error) {
	rcstore, err := s.getContainerStore()
	if err != nil {
		return nil, err
	}

	if err := rcstore.startReading(); err != nil {
		return nil, err
	}
	defer rcstore.stopReading()

	return rcstore.Containers()
}

func (s *store) Layer(id string) (*Layer, error) {
	var res *Layer
	if done, err := s.readAllLayerStores(func(store roLayerStore) (bool, error) {
		layer, err := store.Get(id)
		if err == nil {
			res = layer
			return true, nil
		}
		return false, nil
	}); done {
		return res, err
	}
	return nil, ErrLayerUnknown
}

func (s *store) LookupAdditionalLayer(d digest.Digest, imageref string) (AdditionalLayer, error) {
	adriver, ok := s.graphDriver.(drivers.AdditionalLayerStoreDriver)
	if !ok {
		return nil, ErrLayerUnknown
	}

	al, err := adriver.LookupAdditionalLayer(d, imageref)
	if err != nil {
		if errors.Is(err, drivers.ErrLayerUnknown) {
			return nil, ErrLayerUnknown
		}
		return nil, err
	}
	info, err := al.Info()
	if err != nil {
		return nil, err
	}
	defer info.Close()
	var layer Layer
	if err := json.NewDecoder(info).Decode(&layer); err != nil {
		return nil, err
	}
	return &additionalLayer{&layer, al, s}, nil
}

type additionalLayer struct {
	layer   *Layer
	handler drivers.AdditionalLayer
	s       *store
}

func (al *additionalLayer) UncompressedDigest() digest.Digest {
	return al.layer.UncompressedDigest
}

func (al *additionalLayer) CompressedSize() int64 {
	return al.layer.CompressedSize
}

func (al *additionalLayer) PutAs(id, parent string, names []string) (*Layer, error) {
	rlstore, err := al.s.getLayerStore()
	if err != nil {
		return nil, err
	}
	if err := rlstore.startWriting(); err != nil {
		return nil, err
	}
	defer rlstore.stopWriting()
	rlstores, err := al.s.getROLayerStores()
	if err != nil {
		return nil, err
	}

	var parentLayer *Layer
	if parent != "" {
		for _, lstore := range append([]roLayerStore{rlstore}, rlstores...) {
			if lstore != rlstore {
				if err := lstore.startReading(); err != nil {
					return nil, err
				}
				defer lstore.stopReading()
			}
			parentLayer, err = lstore.Get(parent)
			if err == nil {
				break
			}
		}
		if parentLayer == nil {
			return nil, ErrLayerUnknown
		}
	}

	return rlstore.PutAdditionalLayer(id, parentLayer, names, al.handler)
}

func (al *additionalLayer) Release() {
	al.handler.Release()
}

func (s *store) Image(id string) (*Image, error) {
	var res *Image
	if done, err := s.readAllImageStores(func(store roImageStore) (bool, error) {
		image, err := store.Get(id)
		if err == nil {
			res = image
			return true, nil
		}
		return false, nil
	}); done {
		return res, err
	}
	return nil, fmt.Errorf("locating image with ID %q: %w", id, ErrImageUnknown)
}

func (s *store) ImagesByTopLayer(id string) ([]*Image, error) {
	layer, err := s.Layer(id)
	if err != nil {
		return nil, err
	}

	images := []*Image{}
	if _, err := s.readAllImageStores(func(store roImageStore) (bool, error) {
		imageList, err := store.Images()
		if err != nil {
			return true, err
		}
		for _, image := range imageList {
			image := image
			if image.TopLayer == layer.ID || stringutils.InSlice(image.MappedTopLayers, layer.ID) {
				images = append(images, &image)
			}
		}
		return false, nil
	}); err != nil {
		return nil, err
	}
	return images, nil
}

func (s *store) ImagesByDigest(d digest.Digest) ([]*Image, error) {
	images := []*Image{}
	if _, err := s.readAllImageStores(func(store roImageStore) (bool, error) {
		imageList, err := store.ByDigest(d)
		if err != nil && !errors.Is(err, ErrImageUnknown) {
			return true, err
		}
		images = append(images, imageList...)
		return false, nil
	}); err != nil {
		return nil, err
	}
	return images, nil
}

func (s *store) Container(id string) (*Container, error) {
	rcstore, err := s.getContainerStore()
	if err != nil {
		return nil, err
	}
	if err := rcstore.startReading(); err != nil {
		return nil, err
	}
	defer rcstore.stopReading()

	return rcstore.Get(id)
}

func (s *store) ContainerLayerID(id string) (string, error) {
	rcstore, err := s.getContainerStore()
	if err != nil {
		return "", err
	}
	if err := rcstore.startReading(); err != nil {
		return "", err
	}
	defer rcstore.stopReading()
	container, err := rcstore.Get(id)
	if err != nil {
		return "", err
	}
	return container.LayerID, nil
}

func (s *store) ContainerByLayer(id string) (*Container, error) {
	layer, err := s.Layer(id)
	if err != nil {
		return nil, err
	}
	rcstore, err := s.getContainerStore()
	if err != nil {
		return nil, err
	}
	if err := rcstore.startReading(); err != nil {
		return nil, err
	}
	defer rcstore.stopReading()
	containerList, err := rcstore.Containers()
	if err != nil {
		return nil, err
	}
	for _, container := range containerList {
		if container.LayerID == layer.ID {
			return &container, nil
		}
	}

	return nil, ErrContainerUnknown
}

func (s *store) ContainerDirectory(id string) (string, error) {
	rcstore, err := s.getContainerStore()
	if err != nil {
		return "", err
	}
	if err := rcstore.startReading(); err != nil {
		return "", err
	}
	defer rcstore.stopReading()

	id, err = rcstore.Lookup(id)
	if err != nil {
		return "", err
	}

	middleDir := s.graphDriverName + "-containers"
	gcpath := filepath.Join(s.GraphRoot(), middleDir, id, "userdata")
	if err := os.MkdirAll(gcpath, 0700); err != nil {
		return "", err
	}
	return gcpath, nil
}

func (s *store) ContainerRunDirectory(id string) (string, error) {
	rcstore, err := s.getContainerStore()
	if err != nil {
		return "", err
	}

	if err := rcstore.startReading(); err != nil {
		return "", err
	}
	defer rcstore.stopReading()

	id, err = rcstore.Lookup(id)
	if err != nil {
		return "", err
	}

	middleDir := s.graphDriverName + "-containers"
	rcpath := filepath.Join(s.RunRoot(), middleDir, id, "userdata")
	if err := os.MkdirAll(rcpath, 0700); err != nil {
		return "", err
	}
	return rcpath, nil
}

func (s *store) SetContainerDirectoryFile(id, file string, data []byte) error {
	dir, err := s.ContainerDirectory(id)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0700)
	if err != nil {
		return err
	}
	return ioutils.AtomicWriteFile(filepath.Join(dir, file), data, 0600)
}

func (s *store) FromContainerDirectory(id, file string) ([]byte, error) {
	dir, err := s.ContainerDirectory(id)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, file))
}

func (s *store) SetContainerRunDirectoryFile(id, file string, data []byte) error {
	dir, err := s.ContainerRunDirectory(id)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0700)
	if err != nil {
		return err
	}
	return ioutils.AtomicWriteFile(filepath.Join(dir, file), data, 0600)
}

func (s *store) FromContainerRunDirectory(id, file string) ([]byte, error) {
	dir, err := s.ContainerRunDirectory(id)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, file))
}

func (s *store) Shutdown(force bool) ([]string, error) {
	mounted := []string{}

	rlstore, err := s.getLayerStore()
	if err != nil {
		return mounted, err
	}

	s.graphLock.Lock()
	defer s.graphLock.Unlock()

	if err := rlstore.startWriting(); err != nil {
		return nil, err
	}
	defer rlstore.stopWriting()

	layers, err := rlstore.Layers()
	if err != nil {
		return mounted, err
	}
	for _, layer := range layers {
		if layer.MountCount == 0 {
			continue
		}
		mounted = append(mounted, layer.ID)
		if force {
			for layer.MountCount > 0 {
				_, err2 := rlstore.Unmount(layer.ID, force)
				if err2 != nil {
					if err == nil {
						err = err2
					}
					break
				}
			}
		}
	}
	if len(mounted) > 0 && err == nil {
		err = fmt.Errorf("a layer is mounted: %w", ErrLayerUsedByContainer)
	}
	if err == nil {
		err = s.graphDriver.Cleanup()
		if err2 := s.graphLock.Touch(); err2 != nil {
			if err == nil {
				err = err2
			} else {
				err = fmt.Errorf("(graphLock.Touch failed: %v) %w", err2, err)
			}
		}
	}
	return mounted, err
}

// Convert a BigData key name into an acceptable file name.
func makeBigDataBaseName(key string) string {
	reader := strings.NewReader(key)
	for reader.Len() > 0 {
		ch, size, err := reader.ReadRune()
		if err != nil || size != 1 {
			break
		}
		if ch != '.' && !(ch >= '0' && ch <= '9') && !(ch >= 'a' && ch <= 'z') {
			break
		}
	}
	if reader.Len() > 0 {
		return "=" + base64.StdEncoding.EncodeToString([]byte(key))
	}
	return key
}

func stringSliceWithoutValue(slice []string, value string) []string {
	modified := make([]string, 0, len(slice))
	for _, v := range slice {
		if v == value {
			continue
		}
		modified = append(modified, v)
	}
	return modified
}

func copyStringSlice(slice []string) []string {
	if len(slice) == 0 {
		return nil
	}
	ret := make([]string, len(slice))
	copy(ret, slice)
	return ret
}

func copyStringInt64Map(m map[string]int64) map[string]int64 {
	ret := make(map[string]int64, len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}

func copyStringDigestMap(m map[string]digest.Digest) map[string]digest.Digest {
	ret := make(map[string]digest.Digest, len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}

func copyDigestSlice(slice []digest.Digest) []digest.Digest {
	if len(slice) == 0 {
		return nil
	}
	ret := make([]digest.Digest, len(slice))
	copy(ret, slice)
	return ret
}

// copyStringInterfaceMap still forces us to assume that the interface{} is
// a non-pointer scalar value
func copyStringInterfaceMap(m map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{}, len(m))
	for k, v := range m {
		ret[k] = v
	}
	return ret
}

// AutoUserNsMinSize is the minimum size for automatically created user namespaces
const AutoUserNsMinSize = 1024

// AutoUserNsMaxSize is the maximum size for automatically created user namespaces
const AutoUserNsMaxSize = 65536

// RootAutoUserNsUser is the default user used for root containers when automatically
// creating a user namespace.
const RootAutoUserNsUser = "containers"

// SetDefaultConfigFilePath sets the default configuration to the specified path, and loads the file.
// Deprecated: Use types.SetDefaultConfigFilePath, which can return an error.
func SetDefaultConfigFilePath(path string) {
	_ = types.SetDefaultConfigFilePath(path)
}

// DefaultConfigFile returns the path to the storage config file used
func DefaultConfigFile(rootless bool) (string, error) {
	return types.DefaultConfigFile(rootless)
}

// ReloadConfigurationFile parses the specified configuration file and overrides
// the configuration in storeOptions.
// Deprecated: Use types.ReloadConfigurationFile, which can return an error.
func ReloadConfigurationFile(configFile string, storeOptions *types.StoreOptions) {
	_ = types.ReloadConfigurationFile(configFile, storeOptions)
}

// GetDefaultMountOptions returns the default mountoptions defined in container/storage
func GetDefaultMountOptions() ([]string, error) {
	defaultStoreOptions, err := types.Options()
	if err != nil {
		return nil, err
	}
	return GetMountOptions(defaultStoreOptions.GraphDriverName, defaultStoreOptions.GraphDriverOptions)
}

// GetMountOptions returns the mountoptions for the specified driver and graphDriverOptions
func GetMountOptions(driver string, graphDriverOptions []string) ([]string, error) {
	mountOpts := []string{
		".mountopt",
		fmt.Sprintf("%s.mountopt", driver),
	}
	for _, option := range graphDriverOptions {
		key, val, err := parsers.ParseKeyValueOpt(option)
		if err != nil {
			return nil, err
		}
		key = strings.ToLower(key)
		for _, m := range mountOpts {
			if m == key {
				return strings.Split(val, ","), nil
			}
		}
	}
	return nil, nil
}

// Free removes the store from the list of stores
func (s *store) Free() {
	for i := 0; i < len(stores); i++ {
		if stores[i] == s {
			stores = append(stores[:i], stores[i+1:]...)
			return
		}
	}
}
