package libimage

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/pkg/shortnames"
	storageTransport "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	deepcopy "github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// RuntimeOptions allow for creating a customized Runtime.
type RuntimeOptions struct {
	// The base system context of the runtime which will be used throughout
	// the entire lifespan of the Runtime.  Certain options in some
	// functions may override specific fields.
	SystemContext *types.SystemContext
}

// setRegistriesConfPath sets the registries.conf path for the specified context.
func setRegistriesConfPath(systemContext *types.SystemContext) {
	if systemContext.SystemRegistriesConfPath != "" {
		return
	}
	if envOverride, ok := os.LookupEnv("CONTAINERS_REGISTRIES_CONF"); ok {
		systemContext.SystemRegistriesConfPath = envOverride
		return
	}
	if envOverride, ok := os.LookupEnv("REGISTRIES_CONFIG_PATH"); ok {
		systemContext.SystemRegistriesConfPath = envOverride
		return
	}
}

// Runtime is responsible for image management and storing them in a containers
// storage.
type Runtime struct {
	// Use to send events out to users.
	eventChannel chan *Event
	// Underlying storage store.
	store storage.Store
	// Global system context.  No pointer to simplify copying and modifying
	// it.
	systemContext types.SystemContext
}

// Returns a copy of the runtime's system context.
func (r *Runtime) systemContextCopy() *types.SystemContext {
	var sys types.SystemContext
	deepcopy.Copy(&sys, &r.systemContext)
	return &sys
}

// EventChannel creates a buffered channel for events that the Runtime will use
// to write events to.  Callers are expected to read from the channel in a
// timely manner.
// Can be called once for a given Runtime.
func (r *Runtime) EventChannel() chan *Event {
	if r.eventChannel != nil {
		return r.eventChannel
	}
	r.eventChannel = make(chan *Event, 100)
	return r.eventChannel
}

// RuntimeFromStore returns a Runtime for the specified store.
func RuntimeFromStore(store storage.Store, options *RuntimeOptions) (*Runtime, error) {
	if options == nil {
		options = &RuntimeOptions{}
	}

	var systemContext types.SystemContext
	if options.SystemContext != nil {
		systemContext = *options.SystemContext
	} else {
		systemContext = types.SystemContext{}
	}

	setRegistriesConfPath(&systemContext)

	if systemContext.BlobInfoCacheDir == "" {
		systemContext.BlobInfoCacheDir = filepath.Join(store.GraphRoot(), "cache")
	}

	return &Runtime{
		store:         store,
		systemContext: systemContext,
	}, nil
}

// RuntimeFromStoreOptions returns a return for the specified store options.
func RuntimeFromStoreOptions(runtimeOptions *RuntimeOptions, storeOptions *storage.StoreOptions) (*Runtime, error) {
	if storeOptions == nil {
		storeOptions = &storage.StoreOptions{}
	}
	store, err := storage.GetStore(*storeOptions)
	if err != nil {
		return nil, err
	}
	storageTransport.Transport.SetStore(store)
	return RuntimeFromStore(store, runtimeOptions)
}

// Shutdown attempts to free any kernel resources which are being used by the
// underlying driver.  If "force" is true, any mounted (i.e., in use) layers
// are unmounted beforehand.  If "force" is not true, then layers being in use
// is considered to be an error condition.
func (r *Runtime) Shutdown(force bool) error {
	_, err := r.store.Shutdown(force)
	if r.eventChannel != nil {
		close(r.eventChannel)
	}
	return err
}

// storageToImage transforms a storage.Image to an Image.
func (r *Runtime) storageToImage(storageImage *storage.Image, ref types.ImageReference) *Image {
	return &Image{
		runtime:          r,
		storageImage:     storageImage,
		storageReference: ref,
	}
}

// Exists returns true if the specicifed image exists in the local containers
// storage.  Note that it may return false if an image corrupted.
func (r *Runtime) Exists(name string) (bool, error) {
	image, _, err := r.LookupImage(name, &LookupImageOptions{IgnorePlatform: true})
	if err != nil && errors.Cause(err) != storage.ErrImageUnknown {
		return false, err
	}
	if image == nil {
		return false, nil
	}
	// Inspect the image to make sure if it's corrupted or not.
	if _, err := image.Inspect(context.Background(), false); err != nil {
		logrus.Errorf("Image %s exists in local storage but may be corrupted: %v", name, err)
		return false, nil
	}
	return true, nil
}

// LookupImageOptions allow for customizing local image lookups.
type LookupImageOptions struct {
	// If set, the image will be purely looked up by name.  No matching to
	// the current platform will be performed.  This can be helpful when
	// the platform does not matter, for instance, for image removal.
	IgnorePlatform bool

	// If set, do not look for items/instances in the manifest list that
	// match the current platform but return the manifest list as is.
	lookupManifest bool
}

// Lookup Image looks up `name` in the local container storage matching the
// specified SystemContext.  Returns the image and the name it has been found
// with.  Note that name may also use the `containers-storage:` prefix used to
// refer to the containers-storage transport.  Returns storage.ErrImageUnknown
// if the image could not be found.
//
// If the specified name uses the `containers-storage` transport, the resolved
// name is empty.
func (r *Runtime) LookupImage(name string, options *LookupImageOptions) (*Image, string, error) {
	logrus.Debugf("Looking up image %q in local containers storage", name)

	if options == nil {
		options = &LookupImageOptions{}
	}

	// If needed extract the name sans transport.
	storageRef, err := alltransports.ParseImageName(name)
	if err == nil {
		if storageRef.Transport().Name() != storageTransport.Transport.Name() {
			return nil, "", errors.Errorf("unsupported transport %q for looking up local images", storageRef.Transport().Name())
		}
		img, err := storageTransport.Transport.GetStoreImage(r.store, storageRef)
		if err != nil {
			return nil, "", err
		}
		logrus.Debugf("Found image %q in local containers storage (%s)", name, storageRef.StringWithinTransport())
		return r.storageToImage(img, storageRef), "", nil
	}

	originalName := name
	idByDigest := false
	if strings.HasPrefix(name, "sha256:") {
		// Strip off the sha256 prefix so it can be parsed later on.
		idByDigest = true
		name = strings.TrimPrefix(name, "sha256:")
	}

	// First, check if we have an exact match in the storage. Maybe an ID
	// or a fully-qualified image name.
	img, err := r.lookupImageInLocalStorage(name, name, options)
	if err != nil {
		return nil, "", err
	}
	if img != nil {
		return img, originalName, nil
	}

	// If the name clearly referred to a local image, there's nothing we can
	// do anymore.
	if storageRef != nil || idByDigest {
		return nil, "", errors.Wrap(storage.ErrImageUnknown, originalName)
	}

	// Second, try out the candidates as resolved by shortnames. This takes
	// "localhost/" prefixed images into account as well.
	candidates, err := shortnames.ResolveLocally(&r.systemContext, name)
	if err != nil {
		return nil, "", errors.Wrap(storage.ErrImageUnknown, originalName)
	}
	// Backwards compat: normalize to docker.io as some users may very well
	// rely on that.
	if dockerNamed, err := reference.ParseDockerRef(name); err == nil {
		candidates = append(candidates, dockerNamed)
	}

	for _, candidate := range candidates {
		img, err := r.lookupImageInLocalStorage(name, candidate.String(), options)
		if err != nil {
			return nil, "", err
		}
		if img != nil {
			return img, candidate.String(), err
		}
	}

	return r.lookupImageInDigestsAndRepoTags(originalName, options)
}

// lookupImageInLocalStorage looks up the specified candidate for name in the
// storage and checks whether it's matching the system context.
func (r *Runtime) lookupImageInLocalStorage(name, candidate string, options *LookupImageOptions) (*Image, error) {
	logrus.Debugf("Trying %q ...", candidate)
	img, err := r.store.Image(candidate)
	if err != nil && errors.Cause(err) != storage.ErrImageUnknown {
		return nil, err
	}
	if img == nil {
		return nil, nil
	}
	ref, err := storageTransport.Transport.ParseStoreReference(r.store, img.ID)
	if err != nil {
		return nil, err
	}

	image := r.storageToImage(img, ref)
	logrus.Debugf("Found image %q as %q in local containers storage", name, candidate)

	// If we referenced a manifest list, we need to check whether we can
	// find a matching instance in the local containers storage.
	isManifestList, err := image.IsManifestList(context.Background())
	if err != nil {
		if errors.Cause(err) == os.ErrNotExist {
			// We must be tolerant toward corrupted images.
			// See containers/podman commit fd9dd7065d44.
			logrus.Warnf("error determining if an image is a manifest list: %v, ignoring the error", err)
			return image, nil
		}
		return nil, err
	}
	if options.lookupManifest {
		if isManifestList {
			return image, nil
		}
		return nil, errors.Wrapf(ErrNotAManifestList, candidate)
	}

	if isManifestList {
		logrus.Debugf("Candidate %q is a manifest list, looking up matching instance", candidate)
		manifestList, err := image.ToManifestList()
		if err != nil {
			return nil, err
		}
		instance, err := manifestList.LookupInstance(context.Background(), "", "", "")
		if err != nil {
			// NOTE: If we are not looking for a specific platform
			// and already found the manifest list, then return it
			// instead of the error.
			if options.IgnorePlatform {
				return image, nil
			}
			return nil, errors.Wrap(storage.ErrImageUnknown, err.Error())
		}
		ref, err = storageTransport.Transport.ParseStoreReference(r.store, "@"+instance.ID())
		if err != nil {
			return nil, err
		}
		image = instance
	}

	if options.IgnorePlatform {
		return image, nil
	}

	matches, err := imageReferenceMatchesContext(context.Background(), ref, &r.systemContext)
	if err != nil {
		return nil, err
	}

	// NOTE: if the user referenced by ID we must optimistically assume
	// that they know what they're doing.  Given, we already did the
	// manifest limbo above, we may already have resolved it.
	if !matches && !strings.HasPrefix(image.ID(), candidate) {
		return nil, nil
	}
	// Also print the string within the storage transport.  That may aid in
	// debugging when using additional stores since we see explicitly where
	// the store is and which driver (options) are used.
	logrus.Debugf("Found image %q as %q in local containers storage (%s)", name, candidate, ref.StringWithinTransport())
	return image, nil
}

// lookupImageInDigestsAndRepoTags attempts to match name against any image in
// the local containers storage.  If name is digested, it will be compared
// against image digests.  Otherwise, it will be looked up in the repo tags.
func (r *Runtime) lookupImageInDigestsAndRepoTags(name string, options *LookupImageOptions) (*Image, string, error) {
	// Until now, we've tried very hard to find an image but now it is time
	// for limbo.  If the image includes a digest that we couldn't detect
	// verbatim in the storage, we must have a look at all digests of all
	// images.  Those may change over time (e.g., via manifest lists).
	// Both Podman and Buildah want us to do that dance.
	allImages, err := r.ListImages(context.Background(), nil, nil)
	if err != nil {
		return nil, "", err
	}

	if !shortnames.IsShortName(name) {
		named, err := reference.ParseNormalizedNamed(name)
		if err != nil {
			return nil, "", err
		}
		digested, hasDigest := named.(reference.Digested)
		if !hasDigest {
			return nil, "", errors.Wrap(storage.ErrImageUnknown, name)
		}

		logrus.Debug("Looking for image with matching recorded digests")
		digest := digested.Digest()
		for _, image := range allImages {
			for _, d := range image.Digests() {
				if d == digest {
					return image, name, nil
				}
			}
		}

		return nil, "", errors.Wrap(storage.ErrImageUnknown, name)
	}

	// Podman compat: if we're looking for a short name but couldn't
	// resolve it via the registries.conf dance, we need to look at *all*
	// images and check if the name we're looking for matches a repo tag.
	// Split the name into a repo/tag pair
	split := strings.SplitN(name, ":", 2)
	repo := split[0]
	tag := ""
	if len(split) == 2 {
		tag = split[1]
	}
	for _, image := range allImages {
		named, err := image.inRepoTags(repo, tag)
		if err != nil {
			return nil, "", err
		}
		if named == nil {
			continue
		}
		img, err := r.lookupImageInLocalStorage(name, named.String(), options)
		if err != nil {
			return nil, "", err
		}
		if img != nil {
			return img, named.String(), err
		}
	}

	return nil, "", errors.Wrap(storage.ErrImageUnknown, name)
}

// ResolveName resolves the specified name.  If the name resolves to a local
// image, the fully resolved name will be returned.  Otherwise, the name will
// be properly normalized.
//
// Note that an empty string is returned as is.
func (r *Runtime) ResolveName(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	image, resolvedName, err := r.LookupImage(name, &LookupImageOptions{IgnorePlatform: true})
	if err != nil && errors.Cause(err) != storage.ErrImageUnknown {
		return "", err
	}

	if image != nil && !strings.HasPrefix(image.ID(), resolvedName) {
		return resolvedName, err
	}

	normalized, err := NormalizeName(name)
	if err != nil {
		return "", err
	}

	return normalized.String(), nil
}

// imageReferenceMatchesContext return true if the specified reference matches
// the platform (os, arch, variant) as specified by the system context.
func imageReferenceMatchesContext(ctx context.Context, ref types.ImageReference, sys *types.SystemContext) (bool, error) {
	if sys == nil {
		return true, nil
	}
	img, err := ref.NewImage(ctx, sys)
	if err != nil {
		return false, err
	}
	defer img.Close()
	data, err := img.Inspect(ctx)
	if err != nil {
		return false, err
	}
	osChoice := sys.OSChoice
	if osChoice == "" {
		osChoice = runtime.GOOS
	}
	arch := sys.ArchitectureChoice
	if arch == "" {
		arch = runtime.GOARCH
	}
	if osChoice == data.Os && arch == data.Architecture {
		if sys.VariantChoice == "" || sys.VariantChoice == data.Variant {
			return true, nil
		}
	}
	return false, nil
}

// ListImagesOptions allow for customizing listing images.
type ListImagesOptions struct {
	// Filters to filter the listed images.  Supported filters are
	// * after,before,since=image
	// * dangling=true,false
	// * intermediate=true,false (useful for pruning images)
	// * id=id
	// * label=key[=value]
	// * readonly=true,false
	// * reference=name[:tag] (wildcards allowed)
	Filters []string
}

// ListImages lists images in the local container storage.  If names are
// specified, only images with the specified names are looked up and filtered.
func (r *Runtime) ListImages(ctx context.Context, names []string, options *ListImagesOptions) ([]*Image, error) {
	if options == nil {
		options = &ListImagesOptions{}
	}

	var images []*Image
	if len(names) > 0 {
		lookupOpts := LookupImageOptions{IgnorePlatform: true}
		for _, name := range names {
			image, _, err := r.LookupImage(name, &lookupOpts)
			if err != nil {
				return nil, err
			}
			images = append(images, image)
		}
	} else {
		storageImages, err := r.store.Images()
		if err != nil {
			return nil, err
		}
		for i := range storageImages {
			images = append(images, r.storageToImage(&storageImages[i], nil))
		}
	}

	var filters []filterFunc
	if len(options.Filters) > 0 {
		compiledFilters, err := r.compileImageFilters(ctx, options.Filters)
		if err != nil {
			return nil, err
		}
		filters = append(filters, compiledFilters...)
	}

	return filterImages(images, filters)
}

// RemoveImagesOptions allow for customizing image removal.
type RemoveImagesOptions struct {
	// Force will remove all containers from the local storage that are
	// using a removed image.  Use RemoveContainerFunc for a custom logic.
	// If set, all child images will be removed as well.
	Force bool
	// RemoveContainerFunc allows for a custom logic for removing
	// containers using a specific image.  By default, all containers in
	// the local containers storage will be removed (if Force is set).
	RemoveContainerFunc RemoveContainerFunc
	// Filters to filter the removed images.  Supported filters are
	// * after,before,since=image
	// * dangling=true,false
	// * intermediate=true,false (useful for pruning images)
	// * id=id
	// * label=key[=value]
	// * readonly=true,false
	// * reference=name[:tag] (wildcards allowed)
	Filters []string
	// The RemoveImagesReport will include the size of the removed image.
	// This information may be useful when pruning images to figure out how
	// much space was freed. However, computing the size of an image is
	// comparatively expensive, so it is made optional.
	WithSize bool
}

// RemoveImages removes images specified by names.  All images are expected to
// exist in the local containers storage.
//
// If an image has more names than one name, the image will be untagged with
// the specified name.  RemoveImages returns a slice of untagged and removed
// images.
//
// Note that most errors are non-fatal and collected into `rmErrors` return
// value.
func (r *Runtime) RemoveImages(ctx context.Context, names []string, options *RemoveImagesOptions) (reports []*RemoveImageReport, rmErrors []error) {
	if options == nil {
		options = &RemoveImagesOptions{}
	}

	// The logic here may require some explanation.  Image removal is
	// surprisingly complex since it is recursive (intermediate parents are
	// removed) and since multiple items in `names` may resolve to the
	// *same* image.  On top, the data in the containers storage is shared,
	// so we need to be careful and the code must be robust.  That is why
	// users can only remove images via this function; the logic may be
	// complex but the execution path is clear.

	// Bundle an image with a possible empty slice of names to untag.  That
	// allows for a decent untagging logic and to bundle multiple
	// references to the same *Image (and circumvent consistency issues).
	type deleteMe struct {
		image        *Image
		referencedBy []string
	}

	appendError := func(err error) {
		rmErrors = append(rmErrors, err)
	}

	orderedIDs := []string{}                // determinism and relative order
	deleteMap := make(map[string]*deleteMe) // ID -> deleteMe

	// Look up images in the local containers storage and fill out
	// orderedIDs and the deleteMap.
	switch {
	case len(names) > 0:
		lookupOptions := LookupImageOptions{IgnorePlatform: true}
		for _, name := range names {
			img, resolvedName, err := r.LookupImage(name, &lookupOptions)
			if err != nil {
				appendError(err)
				continue
			}
			dm, exists := deleteMap[img.ID()]
			if !exists {
				orderedIDs = append(orderedIDs, img.ID())
				dm = &deleteMe{image: img}
				deleteMap[img.ID()] = dm
			}
			dm.referencedBy = append(dm.referencedBy, resolvedName)
		}
		if len(orderedIDs) == 0 {
			return nil, rmErrors
		}

	default:
		filteredImages, err := r.ListImages(ctx, nil, &ListImagesOptions{Filters: options.Filters})
		if err != nil {
			appendError(err)
			return nil, rmErrors
		}
		for _, img := range filteredImages {
			orderedIDs = append(orderedIDs, img.ID())
			deleteMap[img.ID()] = &deleteMe{image: img}
		}
	}

	// Now remove the images in the given order.
	rmMap := make(map[string]*RemoveImageReport)
	for _, id := range orderedIDs {
		del, exists := deleteMap[id]
		if !exists {
			appendError(errors.Errorf("internal error: ID %s not in found in image-deletion map", id))
			continue
		}
		if len(del.referencedBy) == 0 {
			del.referencedBy = []string{""}
		}
		for _, ref := range del.referencedBy {
			if err := del.image.remove(ctx, rmMap, ref, options); err != nil {
				appendError(err)
				continue
			}
		}
	}

	// Finally, we can assemble the reports slice.
	for _, id := range orderedIDs {
		report, exists := rmMap[id]
		if exists {
			reports = append(reports, report)
		}
	}

	return reports, rmErrors
}
