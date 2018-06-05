// +build !containers_image_storage_stub

package storage

import (
	"context"
	"strings"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// A storageReference holds an arbitrary name and/or an ID, which is a 32-byte
// value hex-encoded into a 64-character string, and a reference to a Store
// where an image is, or would be, kept.
// Either "named" or "id" must be set.
type storageReference struct {
	transport storageTransport
	named     reference.Named // may include a tag and/or a digest
	id        string
}

func newReference(transport storageTransport, named reference.Named, id string) (*storageReference, error) {
	if named == nil && id == "" {
		return nil, ErrInvalidReference
	}
	// We take a copy of the transport, which contains a pointer to the
	// store that it used for resolving this reference, so that the
	// transport that we'll return from Transport() won't be affected by
	// further calls to the original transport's SetStore() method.
	return &storageReference{
		transport: transport,
		named:     named,
		id:        id,
	}, nil
}

// imageMatchesRepo returns true iff image.Names contains an element with the same repo as ref
func imageMatchesRepo(image *storage.Image, ref reference.Named) bool {
	repo := ref.Name()
	for _, name := range image.Names {
		if named, err := reference.ParseNormalizedNamed(name); err == nil {
			if named.Name() == repo {
				return true
			}
		}
	}
	return false
}

// Resolve the reference's name to an image ID in the store, if there's already
// one present with the same name or ID, and return the image.
func (s *storageReference) resolveImage() (*storage.Image, error) {
	var loadedImage *storage.Image
	if s.id == "" {
		// Look for an image that has the expanded reference name as an explicit Name value.
		image, err := s.transport.store.Image(s.named.String())
		if image != nil && err == nil {
			loadedImage = image
			s.id = image.ID
		}
	}
	if s.id == "" && s.named != nil {
		if digested, ok := s.named.(reference.Digested); ok {
			// Look for an image with the specified digest that has the same name,
			// though possibly with a different tag or digest, as a Name value, so
			// that the canonical reference can be implicitly resolved to the image.
			images, err := s.transport.store.ImagesByDigest(digested.Digest())
			if images != nil && err == nil {
				for _, image := range images {
					if imageMatchesRepo(image, s.named) {
						loadedImage = image
						s.id = image.ID
						break
					}
				}
			}
		}
	}
	if s.id == "" {
		logrus.Debugf("reference %q does not resolve to an image ID", s.StringWithinTransport())
		return nil, errors.Wrapf(ErrNoSuchImage, "reference %q does not resolve to an image ID", s.StringWithinTransport())
	}
	if loadedImage == nil {
		img, err := s.transport.store.Image(s.id)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading image %q", s.id)
		}
		loadedImage = img
	}
	if s.named != nil {
		if !imageMatchesRepo(loadedImage, s.named) {
			logrus.Errorf("no image matching reference %q found", s.StringWithinTransport())
			return nil, ErrNoSuchImage
		}
	}
	return loadedImage, nil
}

// Return a Transport object that defaults to using the same store that we used
// to build this reference object.
func (s storageReference) Transport() types.ImageTransport {
	return &storageTransport{
		store:         s.transport.store,
		defaultUIDMap: s.transport.defaultUIDMap,
		defaultGIDMap: s.transport.defaultGIDMap,
	}
}

// Return a name with a tag or digest, if we have either, else return it bare.
func (s storageReference) DockerReference() reference.Named {
	return s.named
}

// Return a name with a tag, prefixed with the graph root and driver name, to
// disambiguate between images which may be present in multiple stores and
// share only their names.
func (s storageReference) StringWithinTransport() string {
	optionsList := ""
	options := s.transport.store.GraphOptions()
	if len(options) > 0 {
		optionsList = ":" + strings.Join(options, ",")
	}
	res := "[" + s.transport.store.GraphDriverName() + "@" + s.transport.store.GraphRoot() + "+" + s.transport.store.RunRoot() + optionsList + "]"
	if s.named != nil {
		res = res + s.named.String()
	}
	if s.id != "" {
		res = res + "@" + s.id
	}
	return res
}

func (s storageReference) PolicyConfigurationIdentity() string {
	res := "[" + s.transport.store.GraphDriverName() + "@" + s.transport.store.GraphRoot() + "]"
	if s.named != nil {
		res = res + s.named.String()
	}
	if s.id != "" {
		res = res + "@" + s.id
	}
	return res
}

// Also accept policy that's tied to the combination of the graph root and
// driver name, to apply to all images stored in the Store, and to just the
// graph root, in case we're using multiple drivers in the same directory for
// some reason.
func (s storageReference) PolicyConfigurationNamespaces() []string {
	storeSpec := "[" + s.transport.store.GraphDriverName() + "@" + s.transport.store.GraphRoot() + "]"
	driverlessStoreSpec := "[" + s.transport.store.GraphRoot() + "]"
	namespaces := []string{}
	if s.named != nil {
		if s.id != "" {
			// The reference without the ID is also a valid namespace.
			namespaces = append(namespaces, storeSpec+s.named.String())
		}
		tagged, isTagged := s.named.(reference.Tagged)
		_, isDigested := s.named.(reference.Digested)
		if isTagged && isDigested { // s.named is "name:tag@digest"; add a "name:tag" parent namespace.
			namespaces = append(namespaces, storeSpec+s.named.Name()+":"+tagged.Tag())
		}
		components := strings.Split(s.named.Name(), "/")
		for len(components) > 0 {
			namespaces = append(namespaces, storeSpec+strings.Join(components, "/"))
			components = components[:len(components)-1]
		}
	}
	namespaces = append(namespaces, storeSpec)
	namespaces = append(namespaces, driverlessStoreSpec)
	return namespaces
}

// NewImage returns a types.ImageCloser for this reference, possibly specialized for this ImageTransport.
// The caller must call .Close() on the returned ImageCloser.
// NOTE: If any kind of signature verification should happen, build an UnparsedImage from the value returned by NewImageSource,
// verify that UnparsedImage, and convert it into a real Image via image.FromUnparsedImage.
// WARNING: This may not do the right thing for a manifest list, see image.FromSource for details.
func (s storageReference) NewImage(ctx context.Context, sys *types.SystemContext) (types.ImageCloser, error) {
	return newImage(ctx, sys, s)
}

func (s storageReference) DeleteImage(ctx context.Context, sys *types.SystemContext) error {
	img, err := s.resolveImage()
	if err != nil {
		return err
	}
	layers, err := s.transport.store.DeleteImage(img.ID, true)
	if err == nil {
		logrus.Debugf("deleted image %q", img.ID)
		for _, layer := range layers {
			logrus.Debugf("deleted layer %q", layer)
		}
	}
	return err
}

func (s storageReference) NewImageSource(ctx context.Context, sys *types.SystemContext) (types.ImageSource, error) {
	return newImageSource(s)
}

func (s storageReference) NewImageDestination(ctx context.Context, sys *types.SystemContext) (types.ImageDestination, error) {
	return newImageDestination(s)
}
