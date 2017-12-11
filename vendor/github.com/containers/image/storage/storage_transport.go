// +build !containers_image_storage_stub

package storage

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/opencontainers/go-digest"
	ddigest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

func init() {
	transports.Register(Transport)
}

var (
	// Transport is an ImageTransport that uses either a default
	// storage.Store or one that's it's explicitly told to use.
	Transport StoreTransport = &storageTransport{}
	// ErrInvalidReference is returned when ParseReference() is passed an
	// empty reference.
	ErrInvalidReference = errors.New("invalid reference")
	// ErrPathNotAbsolute is returned when a graph root is not an absolute
	// path name.
	ErrPathNotAbsolute = errors.New("path name is not absolute")
)

// StoreTransport is an ImageTransport that uses a storage.Store to parse
// references, either its own default or one that it's told to use.
type StoreTransport interface {
	types.ImageTransport
	// SetStore sets the default store for this transport.
	SetStore(storage.Store)
	// GetImage retrieves the image from the transport's store that's named
	// by the reference.
	GetImage(types.ImageReference) (*storage.Image, error)
	// GetStoreImage retrieves the image from a specified store that's named
	// by the reference.
	GetStoreImage(storage.Store, types.ImageReference) (*storage.Image, error)
	// ParseStoreReference parses a reference, overriding any store
	// specification that it may contain.
	ParseStoreReference(store storage.Store, reference string) (*storageReference, error)
	// SetDefaultUIDMap sets the default UID map to use when opening stores.
	SetDefaultUIDMap(idmap []idtools.IDMap)
	// SetDefaultGIDMap sets the default GID map to use when opening stores.
	SetDefaultGIDMap(idmap []idtools.IDMap)
	// DefaultUIDMap returns the default UID map used when opening stores.
	DefaultUIDMap() []idtools.IDMap
	// DefaultGIDMap returns the default GID map used when opening stores.
	DefaultGIDMap() []idtools.IDMap
}

type storageTransport struct {
	store         storage.Store
	defaultUIDMap []idtools.IDMap
	defaultGIDMap []idtools.IDMap
}

func (s *storageTransport) Name() string {
	// Still haven't really settled on a name.
	return "containers-storage"
}

// SetStore sets the Store object which the Transport will use for parsing
// references when information about a Store is not directly specified as part
// of the reference.  If one is not set, the library will attempt to initialize
// one with default settings when a reference needs to be parsed.  Calling
// SetStore does not affect previously parsed references.
func (s *storageTransport) SetStore(store storage.Store) {
	s.store = store
}

// SetDefaultUIDMap sets the default UID map to use when opening stores.
func (s *storageTransport) SetDefaultUIDMap(idmap []idtools.IDMap) {
	s.defaultUIDMap = idmap
}

// SetDefaultGIDMap sets the default GID map to use when opening stores.
func (s *storageTransport) SetDefaultGIDMap(idmap []idtools.IDMap) {
	s.defaultGIDMap = idmap
}

// DefaultUIDMap returns the default UID map used when opening stores.
func (s *storageTransport) DefaultUIDMap() []idtools.IDMap {
	return s.defaultUIDMap
}

// DefaultGIDMap returns the default GID map used when opening stores.
func (s *storageTransport) DefaultGIDMap() []idtools.IDMap {
	return s.defaultGIDMap
}

// ParseStoreReference takes a name or an ID, tries to figure out which it is
// relative to the given store, and returns it in a reference object.
func (s storageTransport) ParseStoreReference(store storage.Store, ref string) (*storageReference, error) {
	var name reference.Named
	var sum digest.Digest
	var err error
	if ref == "" {
		return nil, ErrInvalidReference
	}
	if ref[0] == '[' {
		// Ignore the store specifier.
		closeIndex := strings.IndexRune(ref, ']')
		if closeIndex < 1 {
			return nil, ErrInvalidReference
		}
		ref = ref[closeIndex+1:]
	}
	refInfo := strings.SplitN(ref, "@", 2)
	if len(refInfo) == 1 {
		// A name.
		name, err = reference.ParseNormalizedNamed(refInfo[0])
		if err != nil {
			return nil, err
		}
	} else if len(refInfo) == 2 {
		// An ID, possibly preceded by a name.
		if refInfo[0] != "" {
			name, err = reference.ParseNormalizedNamed(refInfo[0])
			if err != nil {
				return nil, err
			}
		}
		sum, err = digest.Parse(refInfo[1])
		if err != nil || sum.Validate() != nil {
			sum, err = digest.Parse("sha256:" + refInfo[1])
			if err != nil || sum.Validate() != nil {
				return nil, err
			}
		}
	} else { // Coverage: len(refInfo) is always 1 or 2
		// Anything else: store specified in a form we don't
		// recognize.
		return nil, ErrInvalidReference
	}
	optionsList := ""
	options := store.GraphOptions()
	if len(options) > 0 {
		optionsList = ":" + strings.Join(options, ",")
	}
	storeSpec := "[" + store.GraphDriverName() + "@" + store.GraphRoot() + "+" + store.RunRoot() + optionsList + "]"
	id := ""
	if sum.Validate() == nil {
		id = sum.Hex()
	}
	refname := ""
	if name != nil {
		name = reference.TagNameOnly(name)
		refname = verboseName(name)
	}
	if refname == "" {
		logrus.Debugf("parsed reference to id into %q", storeSpec+"@"+id)
	} else if id == "" {
		logrus.Debugf("parsed reference to refname into %q", storeSpec+refname)
	} else {
		logrus.Debugf("parsed reference to refname@id into %q", storeSpec+refname+"@"+id)
	}
	return newReference(storageTransport{store: store, defaultUIDMap: s.defaultUIDMap, defaultGIDMap: s.defaultGIDMap}, refname, id, name), nil
}

func (s *storageTransport) GetStore() (storage.Store, error) {
	// Return the transport's previously-set store.  If we don't have one
	// of those, initialize one now.
	if s.store == nil {
		options := storage.DefaultStoreOptions
		options.UIDMap = s.defaultUIDMap
		options.GIDMap = s.defaultGIDMap
		store, err := storage.GetStore(options)
		if err != nil {
			return nil, err
		}
		s.store = store
	}
	return s.store, nil
}

// ParseReference takes a name and/or an ID ("_name_"/"@_id_"/"_name_@_id_"),
// possibly prefixed with a store specifier in the form "[_graphroot_]" or
// "[_driver_@_graphroot_]" or "[_driver_@_graphroot_+_runroot_]" or
// "[_driver_@_graphroot_:_options_]" or "[_driver_@_graphroot_+_runroot_:_options_]",
// tries to figure out which it is, and returns it in a reference object.
func (s *storageTransport) ParseReference(reference string) (types.ImageReference, error) {
	var store storage.Store
	// Check if there's a store location prefix.  If there is, then it
	// needs to match a store that was previously initialized using
	// storage.GetStore(), or be enough to let the storage library fill out
	// the rest using knowledge that it has from elsewhere.
	if reference[0] == '[' {
		closeIndex := strings.IndexRune(reference, ']')
		if closeIndex < 1 {
			return nil, ErrInvalidReference
		}
		storeSpec := reference[1:closeIndex]
		reference = reference[closeIndex+1:]
		// Peel off a "driver@" from the start.
		driverInfo := ""
		driverSplit := strings.SplitN(storeSpec, "@", 2)
		if len(driverSplit) != 2 {
			if storeSpec == "" {
				return nil, ErrInvalidReference
			}
		} else {
			driverInfo = driverSplit[0]
			if driverInfo == "" {
				return nil, ErrInvalidReference
			}
			storeSpec = driverSplit[1]
			if storeSpec == "" {
				return nil, ErrInvalidReference
			}
		}
		// Peel off a ":options" from the end.
		var options []string
		optionsSplit := strings.SplitN(storeSpec, ":", 2)
		if len(optionsSplit) == 2 {
			options = strings.Split(optionsSplit[1], ",")
			storeSpec = optionsSplit[0]
		}
		// Peel off a "+runroot" from the new end.
		runRootInfo := ""
		runRootSplit := strings.SplitN(storeSpec, "+", 2)
		if len(runRootSplit) == 2 {
			runRootInfo = runRootSplit[1]
			storeSpec = runRootSplit[0]
		}
		// The rest is our graph root.
		rootInfo := storeSpec
		// Check that any paths are absolute paths.
		if rootInfo != "" && !filepath.IsAbs(rootInfo) {
			return nil, ErrPathNotAbsolute
		}
		if runRootInfo != "" && !filepath.IsAbs(runRootInfo) {
			return nil, ErrPathNotAbsolute
		}
		store2, err := storage.GetStore(storage.StoreOptions{
			GraphDriverName:    driverInfo,
			GraphRoot:          rootInfo,
			RunRoot:            runRootInfo,
			GraphDriverOptions: options,
			UIDMap:             s.defaultUIDMap,
			GIDMap:             s.defaultGIDMap,
		})
		if err != nil {
			return nil, err
		}
		store = store2
	} else {
		// We didn't have a store spec, so use the default.
		store2, err := s.GetStore()
		if err != nil {
			return nil, err
		}
		store = store2
	}
	return s.ParseStoreReference(store, reference)
}

func (s storageTransport) GetStoreImage(store storage.Store, ref types.ImageReference) (*storage.Image, error) {
	dref := ref.DockerReference()
	if dref == nil {
		if sref, ok := ref.(*storageReference); ok {
			if sref.id != "" {
				if img, err := store.Image(sref.id); err == nil {
					return img, nil
				}
			}
		}
		return nil, ErrInvalidReference
	}
	return store.Image(verboseName(dref))
}

func (s *storageTransport) GetImage(ref types.ImageReference) (*storage.Image, error) {
	store, err := s.GetStore()
	if err != nil {
		return nil, err
	}
	return s.GetStoreImage(store, ref)
}

func (s storageTransport) ValidatePolicyConfigurationScope(scope string) error {
	// Check that there's a store location prefix.  Values we're passed are
	// expected to come from PolicyConfigurationIdentity or
	// PolicyConfigurationNamespaces, so if there's no store location,
	// something's wrong.
	if scope[0] != '[' {
		return ErrInvalidReference
	}
	// Parse the store location prefix.
	closeIndex := strings.IndexRune(scope, ']')
	if closeIndex < 1 {
		return ErrInvalidReference
	}
	storeSpec := scope[1:closeIndex]
	scope = scope[closeIndex+1:]
	storeInfo := strings.SplitN(storeSpec, "@", 2)
	if len(storeInfo) == 1 && storeInfo[0] != "" {
		// One component: the graph root.
		if !filepath.IsAbs(storeInfo[0]) {
			return ErrPathNotAbsolute
		}
	} else if len(storeInfo) == 2 && storeInfo[0] != "" && storeInfo[1] != "" {
		// Two components: the driver type and the graph root.
		if !filepath.IsAbs(storeInfo[1]) {
			return ErrPathNotAbsolute
		}
	} else {
		// Anything else: scope specified in a form we don't
		// recognize.
		return ErrInvalidReference
	}
	// That might be all of it, and that's okay.
	if scope == "" {
		return nil
	}
	// But if there is anything left, it has to be a name, with or without
	// a tag, with or without an ID, since we don't return namespace values
	// that are just bare IDs.
	scopeInfo := strings.SplitN(scope, "@", 2)
	if len(scopeInfo) == 1 && scopeInfo[0] != "" {
		_, err := reference.ParseNormalizedNamed(scopeInfo[0])
		if err != nil {
			return err
		}
	} else if len(scopeInfo) == 2 && scopeInfo[0] != "" && scopeInfo[1] != "" {
		_, err := reference.ParseNormalizedNamed(scopeInfo[0])
		if err != nil {
			return err
		}
		_, err = ddigest.Parse("sha256:" + scopeInfo[1])
		if err != nil {
			return err
		}
	} else {
		return ErrInvalidReference
	}
	return nil
}

func verboseName(name reference.Named) string {
	name = reference.TagNameOnly(name)
	tag := ""
	if tagged, ok := name.(reference.NamedTagged); ok {
		tag = ":" + tagged.Tag()
	}
	return name.Name() + tag
}
