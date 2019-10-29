package image

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/directory"
	dockerarchive "github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	ociarchive "github.com/containers/image/v5/oci/archive"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/tarball"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/libpod/driver"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/inspect"
	"github.com/containers/libpod/pkg/registries"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Image is the primary struct for dealing with images
// It is still very much a work in progress
type Image struct {
	// Adding these two structs for now but will cull when we near
	// completion of this library.
	imgRef    types.Image
	imgSrcRef types.ImageSource
	inspect.ImageData
	inspect.ImageResult
	inspectInfo  *types.ImageInspectInfo
	InputName    string
	image        *storage.Image
	imageruntime *Runtime
}

// Runtime contains the store
type Runtime struct {
	store               storage.Store
	SignaturePolicyPath string
	EventsLogFilePath   string
	EventsLogger        string
	Eventer             events.Eventer
}

// InfoImage keep information of Image along with all associated layers
type InfoImage struct {
	// ID of image
	ID string
	// Tags of image
	Tags []string
	// Layers stores all layers of image.
	Layers []LayerInfo
}

// ErrRepoTagNotFound is the error returned when the image id given doesn't match a rep tag in store
var ErrRepoTagNotFound = stderrors.New("unable to match user input to any specific repotag")

// ErrImageIsBareList is the error returned when the image is just a list or index
var ErrImageIsBareList = stderrors.New("image contains a manifest list or image index, but no runnable image")

// NewImageRuntimeFromStore creates an ImageRuntime based on a provided store
func NewImageRuntimeFromStore(store storage.Store) *Runtime {
	return &Runtime{
		store: store,
	}
}

// NewImageRuntimeFromOptions creates an Image Runtime including the store given
// store options
func NewImageRuntimeFromOptions(options storage.StoreOptions) (*Runtime, error) {
	store, err := setStore(options)
	if err != nil {
		return nil, err
	}

	return &Runtime{
		store: store,
	}, nil
}

func setStore(options storage.StoreOptions) (storage.Store, error) {
	store, err := storage.GetStore(options)
	if err != nil {
		return nil, err
	}
	is.Transport.SetStore(store)
	return store, nil
}

// newFromStorage creates a new image object from a storage.Image
func (ir *Runtime) newFromStorage(img *storage.Image) *Image {
	image := Image{
		InputName:    img.ID,
		imageruntime: ir,
		image:        img,
	}
	return &image
}

// NewFromLocal creates a new image object that is intended
// to only deal with local images already in the store (or
// its aliases)
func (ir *Runtime) NewFromLocal(name string) (*Image, error) {
	image := Image{
		InputName:    name,
		imageruntime: ir,
	}
	localImage, err := image.getLocalImage()
	if err != nil {
		return nil, err
	}
	image.image = localImage
	return &image, nil
}

// New creates a new image object where the image could be local
// or remote
func (ir *Runtime) New(ctx context.Context, name, signaturePolicyPath, authfile string, writer io.Writer, dockeroptions *DockerRegistryOptions, signingoptions SigningOptions, label *string, pullType util.PullType) (*Image, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "newImage")
	span.SetTag("type", "runtime")
	defer span.Finish()

	// We don't know if the image is local or not ... check local first
	newImage := Image{
		InputName:    name,
		imageruntime: ir,
	}
	if pullType != util.PullImageAlways {
		localImage, err := newImage.getLocalImage()
		if err == nil {
			newImage.image = localImage
			return &newImage, nil
		} else if pullType == util.PullImageNever {
			return nil, err
		}
	}

	// The image is not local
	if signaturePolicyPath == "" {
		signaturePolicyPath = ir.SignaturePolicyPath
	}
	imageName, err := ir.pullImageFromHeuristicSource(ctx, name, writer, authfile, signaturePolicyPath, signingoptions, dockeroptions, label)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to pull %s", name)
	}

	newImage.InputName = imageName[0]
	img, err := newImage.getLocalImage()
	if err != nil {
		return nil, errors.Wrapf(err, "error retrieving local image after pulling %s", name)
	}
	newImage.image = img
	return &newImage, nil
}

// LoadFromArchiveReference creates a new image object for images pulled from a tar archive and the like (podman load)
// This function is needed because it is possible for a tar archive to have multiple tags for one image
func (ir *Runtime) LoadFromArchiveReference(ctx context.Context, srcRef types.ImageReference, signaturePolicyPath string, writer io.Writer) ([]*Image, error) {
	var newImages []*Image

	if signaturePolicyPath == "" {
		signaturePolicyPath = ir.SignaturePolicyPath
	}
	imageNames, err := ir.pullImageFromReference(ctx, srcRef, writer, "", signaturePolicyPath, SigningOptions{}, &DockerRegistryOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "unable to pull %s", transports.ImageName(srcRef))
	}

	for _, name := range imageNames {
		newImage := Image{
			InputName:    name,
			imageruntime: ir,
		}
		img, err := newImage.getLocalImage()
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving local image after pulling %s", name)
		}
		newImage.image = img
		newImages = append(newImages, &newImage)
	}
	ir.newImageEvent(events.LoadFromArchive, "")
	return newImages, nil
}

// Shutdown closes down the storage and require a bool arg as to
// whether it should do so forcibly.
func (ir *Runtime) Shutdown(force bool) error {
	_, err := ir.store.Shutdown(force)
	return err
}

func (i *Image) reloadImage() error {
	newImage, err := i.imageruntime.getImage(i.ID())
	if err != nil {
		return errors.Wrapf(err, "unable to reload image")
	}
	i.image = newImage.image
	return nil
}

// stringSha256 strips sha256 from user input
func stripSha256(name string) string {
	if strings.HasPrefix(name, "sha256:") && len(name) > 7 {
		return name[7:]
	}
	return name
}

// getLocalImage resolves an unknown input describing an image and
// returns a storage.Image or an error. It is used by NewFromLocal.
func (i *Image) getLocalImage() (*storage.Image, error) {
	imageError := fmt.Sprintf("unable to find '%s' in local storage", i.InputName)
	if i.InputName == "" {
		return nil, errors.Errorf("input name is blank")
	}
	// Check if the input name has a transport and if so strip it
	dest, err := alltransports.ParseImageName(i.InputName)
	if err == nil && dest.DockerReference() != nil {
		i.InputName = dest.DockerReference().String()
	}

	img, err := i.imageruntime.getImage(stripSha256(i.InputName))
	if err == nil {
		return img.image, err
	}

	// container-storage wasn't able to find it in its current form
	// check if the input name has a tag, and if not, run it through
	// again
	decomposedImage, err := decompose(i.InputName)
	if err != nil {
		return nil, err
	}

	// The image has a registry name in it and we made sure we looked for it locally
	// with a tag.  It cannot be local.
	if decomposedImage.hasRegistry {
		return nil, errors.Wrapf(ErrNoSuchImage, imageError)
	}
	// if the image is saved with the repository localhost, searching with localhost prepended is necessary
	// We don't need to strip the sha because we have already determined it is not an ID
	ref, err := decomposedImage.referenceWithRegistry(DefaultLocalRegistry)
	if err != nil {
		return nil, err
	}
	img, err = i.imageruntime.getImage(ref.String())
	if err == nil {
		return img.image, err
	}

	// grab all the local images
	images, err := i.imageruntime.GetImages()
	if err != nil {
		return nil, err
	}

	// check the repotags of all images for a match
	repoImage, err := findImageInRepotags(decomposedImage, images)
	if err == nil {
		return repoImage, nil
	}

	return nil, errors.Wrapf(ErrNoSuchImage, err.Error())
}

// ID returns the image ID as a string
func (i *Image) ID() string {
	return i.image.ID
}

// IsReadOnly returns whether the image ID comes from a local store
func (i *Image) IsReadOnly() bool {
	return i.image.ReadOnly
}

// Digest returns the image's digest
func (i *Image) Digest() digest.Digest {
	return i.image.Digest
}

// Digests returns the image's digests
func (i *Image) Digests() []digest.Digest {
	return i.image.Digests
}

// GetManifest returns the image's manifest as a byte array
// and manifest type as a string.
func (i *Image) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	imgSrcRef, err := i.toImageSourceRef(ctx)
	if err != nil {
		return nil, "", err
	}
	return imgSrcRef.GetManifest(ctx, instanceDigest)
}

// Manifest returns the image's manifest as a byte array
// and manifest type as a string.
func (i *Image) Manifest(ctx context.Context) ([]byte, string, error) {
	imgRef, err := i.toImageRef(ctx)
	if err != nil {
		return nil, "", err
	}
	return imgRef.Manifest(ctx)
}

// Names returns a string array of names associated with the image, which may be a mixture of tags and digests
func (i *Image) Names() []string {
	return i.image.Names
}

// RepoTags returns a string array of repotags associated with the image
func (i *Image) RepoTags() ([]string, error) {
	var repoTags []string
	for _, name := range i.Names() {
		named, err := reference.ParseNormalizedNamed(name)
		if err != nil {
			return nil, err
		}
		if tagged, isTagged := named.(reference.NamedTagged); isTagged {
			repoTags = append(repoTags, tagged.String())
		}
	}
	return repoTags, nil
}

// RepoDigests returns a string array of repodigests associated with the image
func (i *Image) RepoDigests() ([]string, error) {
	var repoDigests []string
	added := make(map[string]struct{})

	for _, name := range i.Names() {
		for _, imageDigest := range append(i.Digests(), i.Digest()) {
			if imageDigest == "" {
				continue
			}

			named, err := reference.ParseNormalizedNamed(name)
			if err != nil {
				return nil, err
			}

			canonical, err := reference.WithDigest(reference.TrimNamed(named), imageDigest)
			if err != nil {
				return nil, err
			}

			if _, alreadyInList := added[canonical.String()]; !alreadyInList {
				repoDigests = append(repoDigests, canonical.String())
				added[canonical.String()] = struct{}{}
			}
		}
	}
	sort.Strings(repoDigests)
	return repoDigests, nil
}

// Created returns the time the image was created
func (i *Image) Created() time.Time {
	return i.image.Created
}

// TopLayer returns the top layer id as a string
func (i *Image) TopLayer() string {
	return i.image.TopLayer
}

// Remove an image; container removal for the image must be done
// outside the context of images
// TODO: the force param does nothing as of now. Need to move container
// handling logic here eventually.
func (i *Image) Remove(ctx context.Context, force bool) error {
	parent, err := i.GetParent(ctx)
	if err != nil {
		return err
	}
	if _, err := i.imageruntime.store.DeleteImage(i.ID(), true); err != nil {
		return err
	}
	i.newImageEvent(events.Remove)
	for parent != nil {
		nextParent, err := parent.GetParent(ctx)
		if err != nil {
			return err
		}
		children, err := parent.GetChildren(ctx)
		if err != nil {
			return err
		}
		// Do not remove if image is a base image and is not untagged, or if
		// the image has more children.
		if len(children) > 0 || len(parent.Names()) > 0 {
			return nil
		}
		id := parent.ID()
		if _, err := i.imageruntime.store.DeleteImage(id, true); err != nil {
			logrus.Debugf("unable to remove intermediate image %q: %v", id, err)
		} else {
			fmt.Println(id)
		}
		parent = nextParent
	}
	return nil
}

// getImage retrieves an image matching the given name or hash from system
// storage
// If no matching image can be found, an error is returned
func (ir *Runtime) getImage(image string) (*Image, error) {
	var img *storage.Image
	ref, err := is.Transport.ParseStoreReference(ir.store, image)
	if err == nil {
		img, err = is.Transport.GetStoreImage(ir.store, ref)
	}
	if err != nil {
		img2, err2 := ir.store.Image(image)
		if err2 != nil {
			if ref == nil {
				return nil, errors.Wrapf(err, "error parsing reference to image %q", image)
			}
			return nil, errors.Wrapf(err, "unable to locate image %q", image)
		}
		img = img2
	}
	newImage := ir.newFromStorage(img)
	return newImage, nil
}

// GetImages retrieves all images present in storage
func (ir *Runtime) GetImages() ([]*Image, error) {
	return ir.getImages(false)
}

// GetRWImages retrieves all read/write images present in storage
func (ir *Runtime) GetRWImages() ([]*Image, error) {
	return ir.getImages(true)
}

// getImages retrieves all images present in storage
func (ir *Runtime) getImages(rwOnly bool) ([]*Image, error) {
	var newImages []*Image
	images, err := ir.store.Images()
	if err != nil {
		return nil, err
	}
	for _, i := range images {
		if rwOnly && i.ReadOnly {
			continue
		}
		// iterating over these, be careful to not iterate on the literal
		// pointer.
		image := i
		img := ir.newFromStorage(&image)
		newImages = append(newImages, img)
	}
	return newImages, nil
}

// getImageDigest creates an image object and uses the hex value of the digest as the image ID
// for parsing the store reference
func getImageDigest(ctx context.Context, src types.ImageReference, sc *types.SystemContext) (string, error) {
	newImg, err := src.NewImage(ctx, sc)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := newImg.Close(); err != nil {
			logrus.Errorf("failed to close image: %q", err)
		}
	}()
	imageDigest := newImg.ConfigInfo().Digest
	if err = imageDigest.Validate(); err != nil {
		return "", errors.Wrapf(err, "error getting config info")
	}
	return "@" + imageDigest.Hex(), nil
}

// normalizedTag returns the canonical version of tag for use in Image.Names()
func normalizedTag(tag string) (reference.Named, error) {
	decomposedTag, err := decompose(tag)
	if err != nil {
		return nil, err
	}
	// If the input doesn't specify a registry, set the registry to localhost
	var ref reference.Named
	if !decomposedTag.hasRegistry {
		ref, err = decomposedTag.referenceWithRegistry(DefaultLocalRegistry)
		if err != nil {
			return nil, err
		}
	} else {
		ref, err = decomposedTag.normalizedReference()
		if err != nil {
			return nil, err
		}
	}
	// If the input does not have a tag, we need to add one (latest)
	ref = reference.TagNameOnly(ref)
	return ref, nil
}

// TagImage adds a tag to the given image
func (i *Image) TagImage(tag string) error {
	if err := i.reloadImage(); err != nil {
		return err
	}
	ref, err := normalizedTag(tag)
	if err != nil {
		return err
	}
	tags := i.Names()
	if util.StringInSlice(ref.String(), tags) {
		return nil
	}
	tags = append(tags, ref.String())
	if err := i.imageruntime.store.SetNames(i.ID(), tags); err != nil {
		return err
	}
	if err := i.reloadImage(); err != nil {
		return err
	}
	i.newImageEvent(events.Tag)
	return nil
}

// UntagImage removes a tag from the given image
func (i *Image) UntagImage(tag string) error {
	if err := i.reloadImage(); err != nil {
		return err
	}
	var newTags []string
	tags := i.Names()
	if !util.StringInSlice(tag, tags) {
		return nil
	}
	for _, t := range tags {
		if tag != t {
			newTags = append(newTags, t)
		}
	}
	if err := i.imageruntime.store.SetNames(i.ID(), newTags); err != nil {
		return err
	}
	if err := i.reloadImage(); err != nil {
		return err
	}
	i.newImageEvent(events.Untag)
	return nil
}

// PushImageToHeuristicDestination pushes the given image to "destination", which is heuristically parsed.
// Use PushImageToReference if the destination is known precisely.
func (i *Image) PushImageToHeuristicDestination(ctx context.Context, destination, manifestMIMEType, authFile, digestFile, signaturePolicyPath string, writer io.Writer, forceCompress bool, signingOptions SigningOptions, dockerRegistryOptions *DockerRegistryOptions, additionalDockerArchiveTags []reference.NamedTagged) error {
	if destination == "" {
		return errors.Wrapf(syscall.EINVAL, "destination image name must be specified")
	}

	// Get the destination Image Reference
	dest, err := alltransports.ParseImageName(destination)
	if err != nil {
		if hasTransport(destination) {
			return errors.Wrapf(err, "error getting destination imageReference for %q", destination)
		}
		// Try adding the images default transport
		destination2 := DefaultTransport + destination
		dest, err = alltransports.ParseImageName(destination2)
		if err != nil {
			return err
		}
	}
	return i.PushImageToReference(ctx, dest, manifestMIMEType, authFile, digestFile, signaturePolicyPath, writer, forceCompress, signingOptions, dockerRegistryOptions, additionalDockerArchiveTags)
}

// PushImageToReference pushes the given image to a location described by the given path
func (i *Image) PushImageToReference(ctx context.Context, dest types.ImageReference, manifestMIMEType, authFile, digestFile, signaturePolicyPath string, writer io.Writer, forceCompress bool, signingOptions SigningOptions, dockerRegistryOptions *DockerRegistryOptions, additionalDockerArchiveTags []reference.NamedTagged) error {
	sc := GetSystemContext(signaturePolicyPath, authFile, forceCompress)
	sc.BlobInfoCacheDir = filepath.Join(i.imageruntime.store.GraphRoot(), "cache")

	policyContext, err := getPolicyContext(sc)
	if err != nil {
		return err
	}
	defer func() {
		if err := policyContext.Destroy(); err != nil {
			logrus.Errorf("failed to destroy policy context: %q", err)
		}
	}()

	// Look up the source image, expecting it to be in local storage
	src, err := is.Transport.ParseStoreReference(i.imageruntime.store, i.ID())
	if err != nil {
		return errors.Wrapf(err, "error getting source imageReference for %q", i.InputName)
	}
	copyOptions := getCopyOptions(sc, writer, nil, dockerRegistryOptions, signingOptions, manifestMIMEType, additionalDockerArchiveTags)
	copyOptions.DestinationCtx.SystemRegistriesConfPath = registries.SystemRegistriesConfPath() // FIXME: Set this more globally.  Probably no reason not to have it in every types.SystemContext, and to compute the value just once in one place.
	// Copy the image to the remote destination
	manifestBytes, err := cp.Image(ctx, policyContext, dest, src, copyOptions)
	if err != nil {
		return errors.Wrapf(err, "Error copying image to the remote destination")
	}
	digest, err := manifest.Digest(manifestBytes)
	if err != nil {
		return errors.Wrapf(err, "error computing digest of manifest of new image %q", transports.ImageName(dest))
	}

	logrus.Debugf("Successfully pushed %s with digest %s", transports.ImageName(dest), digest.String())

	if digestFile != "" {
		if err = ioutil.WriteFile(digestFile, []byte(digest.String()), 0644); err != nil {
			return errors.Wrapf(err, "failed to write digest to file %q", digestFile)
		}
	}
	i.newImageEvent(events.Push)
	return nil
}

// MatchesID returns a bool based on if the input id
// matches the image's id
// TODO: This isn't used anywhere, so remove it
func (i *Image) MatchesID(id string) bool {
	return strings.HasPrefix(i.ID(), id)
}

// ToImageRef returns an image reference type from an image
// TODO: Hopefully we can remove this exported function for mheon
func (i *Image) ToImageRef(ctx context.Context) (types.Image, error) {
	return i.toImageRef(ctx)
}

// toImageSourceRef returns an ImageSource Reference type from an image
func (i *Image) toImageSourceRef(ctx context.Context) (types.ImageSource, error) {
	if i == nil {
		return nil, errors.Errorf("cannot convert nil image to image source reference")
	}
	if i.imgSrcRef == nil {
		ref, err := is.Transport.ParseStoreReference(i.imageruntime.store, "@"+i.ID())
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing reference to image %q", i.ID())
		}
		imgSrcRef, err := ref.NewImageSource(ctx, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading image %q as image source", i.ID())
		}
		i.imgSrcRef = imgSrcRef
	}
	return i.imgSrcRef, nil
}

//Size returns the size of the image
func (i *Image) Size(ctx context.Context) (*uint64, error) {
	if i.image == nil {
		localImage, err := i.getLocalImage()
		if err != nil {
			return nil, err
		}
		i.image = localImage
	}
	if sum, err := i.imageruntime.store.ImageSize(i.ID()); err == nil && sum >= 0 {
		usum := uint64(sum)
		return &usum, nil
	}
	return nil, errors.Errorf("unable to determine size")
}

// toImageRef returns an Image Reference type from an image
func (i *Image) toImageRef(ctx context.Context) (types.Image, error) {
	if i == nil {
		return nil, errors.Errorf("cannot convert nil image to image reference")
	}
	imgSrcRef, err := i.toImageSourceRef(ctx)
	if err != nil {
		return nil, err
	}
	if i.imgRef == nil {
		systemContext := &types.SystemContext{}
		unparsedDefaultInstance := image.UnparsedInstance(imgSrcRef, nil)
		imgRef, err := image.FromUnparsedImage(ctx, systemContext, unparsedDefaultInstance)
		if err != nil {
			// check for a "tried-to-treat-a-bare-list-like-a-runnable-image" problem, else
			// return info about the not-a-bare-list runnable image part of this storage.Image
			if manifestBytes, manifestType, err2 := imgSrcRef.GetManifest(ctx, nil); err2 == nil {
				if manifest.MIMETypeIsMultiImage(manifestType) {
					if list, err3 := manifest.ListFromBlob(manifestBytes, manifestType); err3 == nil {
						switch manifestType {
						case ociv1.MediaTypeImageIndex:
							err = errors.Wrapf(ErrImageIsBareList, "%q is an image index", i.InputName)
						case manifest.DockerV2ListMediaType:
							err = errors.Wrapf(ErrImageIsBareList, "%q is a manifest list", i.InputName)
						default:
							err = errors.Wrapf(ErrImageIsBareList, "%q", i.InputName)
						}
						for _, instanceDigest := range list.Instances() {
							instance := instanceDigest
							unparsedInstance := image.UnparsedInstance(imgSrcRef, &instance)
							if imgRef2, err4 := image.FromUnparsedImage(ctx, systemContext, unparsedInstance); err4 == nil {
								imgRef = imgRef2
								err = nil
								break
							}
						}
					}
				}
			}
			if err != nil {
				return nil, errors.Wrapf(err, "error reading image %q as image", i.ID())
			}
		}
		i.imgRef = imgRef
	}
	return i.imgRef, nil
}

// DriverData gets the driver data from the store on a layer
func (i *Image) DriverData() (*driver.Data, error) {
	return driver.GetDriverData(i.imageruntime.store, i.TopLayer())
}

// Layer returns the image's top layer
func (i *Image) Layer() (*storage.Layer, error) {
	return i.imageruntime.store.Layer(i.image.TopLayer)
}

// History contains the history information of an image
type History struct {
	ID        string     `json:"id"`
	Created   *time.Time `json:"created"`
	CreatedBy string     `json:"createdBy"`
	Size      int64      `json:"size"`
	Comment   string     `json:"comment"`
}

// History gets the history of an image and the IDs of images that are part of
// its history
func (i *Image) History(ctx context.Context) ([]*History, error) {
	img, err := i.toImageRef(ctx)
	if err != nil {
		if errors.Cause(err) == ErrImageIsBareList {
			return nil, nil
		}
		return nil, err
	}
	oci, err := img.OCIConfig(ctx)
	if err != nil {
		return nil, err
	}

	// Use our layers list to find images that use any of them (or no
	// layer, since every base layer is derived from an empty layer) as its
	// topmost layer.
	interestingLayers := make(map[string]bool)
	var layer *storage.Layer
	if i.TopLayer() != "" {
		if layer, err = i.imageruntime.store.Layer(i.TopLayer()); err != nil {
			return nil, err
		}
	}
	interestingLayers[""] = true
	for layer != nil {
		interestingLayers[layer.ID] = true
		if layer.Parent == "" {
			break
		}
		layer, err = i.imageruntime.store.Layer(layer.Parent)
		if err != nil {
			return nil, err
		}
	}

	// Get the IDs of the images that share some of our layers.  Hopefully
	// this step means that we'll be able to avoid reading the
	// configuration of every single image in local storage later on.
	images, err := i.imageruntime.GetImages()
	if err != nil {
		return nil, errors.Wrapf(err, "error getting images from store")
	}
	interestingImages := make([]*Image, 0, len(images))
	for i := range images {
		if interestingLayers[images[i].TopLayer()] {
			interestingImages = append(interestingImages, images[i])
		}
	}

	// Build a list of image IDs that correspond to our history entries.
	historyImages := make([]*Image, len(oci.History))
	if len(oci.History) > 0 {
		// The starting image shares its whole history with itself.
		historyImages[len(historyImages)-1] = i
		for i := range interestingImages {
			image, err := images[i].ociv1Image(ctx)
			if err != nil {
				return nil, errors.Wrapf(err, "error getting image configuration for image %q", images[i].ID())
			}
			// If the candidate has a longer history or no history
			// at all, then it doesn't share the portion of our
			// history that we're interested in matching with other
			// images.
			if len(image.History) == 0 || len(image.History) > len(historyImages) {
				continue
			}
			// If we don't include all of the layers that the
			// candidate image does (i.e., our rootfs didn't look
			// like its rootfs at any point), then it can't be part
			// of our history.
			if len(image.RootFS.DiffIDs) > len(oci.RootFS.DiffIDs) {
				continue
			}
			candidateLayersAreUsed := true
			for i := range image.RootFS.DiffIDs {
				if image.RootFS.DiffIDs[i] != oci.RootFS.DiffIDs[i] {
					candidateLayersAreUsed = false
					break
				}
			}
			if !candidateLayersAreUsed {
				continue
			}
			// If the candidate's entire history is an initial
			// portion of our history, then we're based on it,
			// either directly or indirectly.
			sharedHistory := historiesMatch(oci.History, image.History)
			if sharedHistory == len(image.History) {
				historyImages[sharedHistory-1] = images[i]
			}
		}
	}

	var (
		size       int64
		sizeCount  = 1
		allHistory []*History
	)

	for i := len(oci.History) - 1; i >= 0; i-- {
		imageID := "<missing>"
		if historyImages[i] != nil {
			imageID = historyImages[i].ID()
		}
		if !oci.History[i].EmptyLayer {
			size = img.LayerInfos()[len(img.LayerInfos())-sizeCount].Size
			sizeCount++
		}
		allHistory = append(allHistory, &History{
			ID:        imageID,
			Created:   oci.History[i].Created,
			CreatedBy: oci.History[i].CreatedBy,
			Size:      size,
			Comment:   oci.History[i].Comment,
		})
	}
	return allHistory, nil
}

// Dangling returns a bool if the image is "dangling"
func (i *Image) Dangling() bool {
	return len(i.Names()) == 0
}

// Labels returns the image's labels
func (i *Image) Labels(ctx context.Context) (map[string]string, error) {
	imgInspect, err := i.imageInspectInfo(ctx)
	if err != nil {
		return nil, nil
	}
	return imgInspect.Labels, nil
}

// GetLabel Returns a case-insensitive match of a given label
func (i *Image) GetLabel(ctx context.Context, label string) (string, error) {
	imageLabels, err := i.Labels(ctx)
	if err != nil {
		return "", err
	}
	for k, v := range imageLabels {
		if strings.ToLower(k) == strings.ToLower(label) {
			return v, nil
		}
	}
	return "", nil
}

// Annotations returns the annotations of an image
func (i *Image) Annotations(ctx context.Context) (map[string]string, error) {
	imageManifest, manifestType, err := i.Manifest(ctx)
	if err != nil {
		imageManifest, manifestType, err = i.GetManifest(ctx, nil)
		if err != nil {
			return nil, err
		}
	}
	annotations := make(map[string]string)
	switch manifestType {
	case ociv1.MediaTypeImageManifest:
		var m ociv1.Manifest
		if err := json.Unmarshal(imageManifest, &m); err == nil {
			for k, v := range m.Annotations {
				annotations[k] = v
			}
		}
	}
	return annotations, nil
}

// ociv1Image converts an image to an imgref and then returns its config blob
// converted to an ociv1 image type
func (i *Image) ociv1Image(ctx context.Context) (*ociv1.Image, error) {
	imgRef, err := i.toImageRef(ctx)
	if err != nil {
		return nil, err
	}
	return imgRef.OCIConfig(ctx)
}

func (i *Image) imageInspectInfo(ctx context.Context) (*types.ImageInspectInfo, error) {
	if i.inspectInfo == nil {
		ic, err := i.toImageRef(ctx)
		if err != nil {
			return nil, err
		}
		imgInspect, err := ic.Inspect(ctx)
		if err != nil {
			return nil, err
		}
		i.inspectInfo = imgInspect
	}
	return i.inspectInfo, nil
}

// Inspect returns an image's inspect data
func (i *Image) Inspect(ctx context.Context) (*inspect.ImageData, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "imageInspect")
	span.SetTag("type", "image")
	defer span.Finish()

	ociv1Img, err := i.ociv1Image(ctx)
	if err != nil {
		ociv1Img = &ociv1.Image{}
	}
	info, err := i.imageInspectInfo(ctx)
	if err != nil {
		info = &types.ImageInspectInfo{}
	}
	annotations, err := i.Annotations(ctx)
	if err != nil {
		return nil, err
	}

	size := int64(-1)
	if usize, err := i.Size(ctx); err == nil {
		size = int64(*usize)
	}

	repoTags, err := i.RepoTags()
	if err != nil {
		return nil, err
	}

	repoDigests, err := i.RepoDigests()
	if err != nil {
		return nil, err
	}

	driver, err := i.DriverData()
	if err != nil {
		return nil, err
	}

	_, manifestType, err := i.GetManifest(ctx, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to determine manifest type")
	}
	comment, err := i.Comment(ctx, manifestType)
	if err != nil {
		return nil, err
	}

	data := &inspect.ImageData{
		ID:           i.ID(),
		RepoTags:     repoTags,
		RepoDigests:  repoDigests,
		Comment:      comment,
		Created:      ociv1Img.Created,
		Author:       ociv1Img.Author,
		Architecture: ociv1Img.Architecture,
		Os:           ociv1Img.OS,
		Config:       &ociv1Img.Config,
		Version:      info.DockerVersion,
		Size:         size,
		VirtualSize:  size,
		Annotations:  annotations,
		Digest:       i.Digest(),
		Labels:       info.Labels,
		RootFS: &inspect.RootFS{
			Type:   ociv1Img.RootFS.Type,
			Layers: ociv1Img.RootFS.DiffIDs,
		},
		GraphDriver:  driver,
		ManifestType: manifestType,
		User:         ociv1Img.Config.User,
		History:      ociv1Img.History,
	}
	return data, nil
}

// Import imports and image into the store and returns an image
func (ir *Runtime) Import(ctx context.Context, path, reference string, writer io.Writer, signingOptions SigningOptions, imageConfig ociv1.Image) (*Image, error) {
	src, err := tarball.Transport.ParseReference(path)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing image name %q", path)
	}

	updater, ok := src.(tarball.ConfigUpdater)
	if !ok {
		return nil, errors.Wrapf(err, "unexpected type, a tarball reference should implement tarball.ConfigUpdater")
	}

	annotations := make(map[string]string)

	//	config imgspecv1.Image
	err = updater.ConfigUpdate(imageConfig, annotations)
	if err != nil {
		return nil, errors.Wrapf(err, "error updating image config")
	}

	sc := GetSystemContext("", "", false)

	// if reference not given, get the image digest
	if reference == "" {
		reference, err = getImageDigest(ctx, src, sc)
		if err != nil {
			return nil, err
		}
	}
	policyContext, err := getPolicyContext(sc)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := policyContext.Destroy(); err != nil {
			logrus.Errorf("failed to destroy policy context: %q", err)
		}
	}()
	copyOptions := getCopyOptions(sc, writer, nil, nil, signingOptions, "", nil)
	dest, err := is.Transport.ParseStoreReference(ir.store, reference)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting image reference for %q", reference)
	}
	_, err = cp.Image(ctx, policyContext, dest, src, copyOptions)
	if err != nil {
		return nil, err
	}
	newImage, err := ir.NewFromLocal(reference)
	if err == nil {
		newImage.newImageEvent(events.Import)
	}
	return newImage, err
}

// MatchRepoTag takes a string and tries to match it against an
// image's repotags
func (i *Image) MatchRepoTag(input string) (string, error) {
	results := make(map[int][]string)
	var maxCount int
	// first check if we have an exact match with the input
	if util.StringInSlice(input, i.Names()) {
		return input, nil
	}
	// next check if we are missing the tag
	dcImage, err := decompose(input)
	if err != nil {
		return "", err
	}
	imageRegistry, imageName, imageSuspiciousTagValueForSearch := dcImage.suspiciousRefNameTagValuesForSearch()
	for _, repoName := range i.Names() {
		count := 0
		dcRepoName, err := decompose(repoName)
		if err != nil {
			return "", err
		}
		repoNameRegistry, repoNameName, repoNameSuspiciousTagValueForSearch := dcRepoName.suspiciousRefNameTagValuesForSearch()
		if repoNameRegistry == imageRegistry && imageRegistry != "" {
			count++
		}
		if repoNameName == imageName && imageName != "" {
			count++
		} else if splitString(repoNameName) == splitString(imageName) {
			count++
		}
		if repoNameSuspiciousTagValueForSearch == imageSuspiciousTagValueForSearch {
			count++
		}
		results[count] = append(results[count], repoName)
		if count > maxCount {
			maxCount = count
		}
	}
	if maxCount == 0 {
		return "", ErrRepoTagNotFound
	}
	if len(results[maxCount]) > 1 {
		return "", errors.Errorf("user input matched multiple repotags for the image")
	}
	return results[maxCount][0], nil
}

// splitString splits input string by / and returns the last array item
func splitString(input string) string {
	split := strings.Split(input, "/")
	return split[len(split)-1]
}

// IsParent goes through the layers in the store and checks if i.TopLayer is
// the parent of any other layer in store. Double check that image with that
// layer exists as well.
func (i *Image) IsParent(ctx context.Context) (bool, error) {
	children, err := i.getChildren(ctx, 1)
	if err != nil {
		if errors.Cause(err) == ErrImageIsBareList {
			return false, nil
		}
		return false, err
	}
	return len(children) > 0, nil
}

// historiesMatch returns the number of entries in the histories which have the
// same contents
func historiesMatch(a, b []imgspecv1.History) int {
	i := 0
	for i < len(a) && i < len(b) {
		if a[i].Created != nil && b[i].Created == nil {
			return i
		}
		if a[i].Created == nil && b[i].Created != nil {
			return i
		}
		if a[i].Created != nil && b[i].Created != nil {
			if !a[i].Created.Equal(*(b[i].Created)) {
				return i
			}
		}
		if a[i].CreatedBy != b[i].CreatedBy {
			return i
		}
		if a[i].Author != b[i].Author {
			return i
		}
		if a[i].Comment != b[i].Comment {
			return i
		}
		if a[i].EmptyLayer != b[i].EmptyLayer {
			return i
		}
		i++
	}
	return i
}

// areParentAndChild checks diff ID and history in the two images and return
// true if the second should be considered to be directly based on the first
func areParentAndChild(parent, child *imgspecv1.Image) bool {
	// the child and candidate parent should share all of the
	// candidate parent's diff IDs, which together would have
	// controlled which layers were used
	if len(parent.RootFS.DiffIDs) > len(child.RootFS.DiffIDs) {
		return false
	}
	childUsesCandidateDiffs := true
	for i := range parent.RootFS.DiffIDs {
		if child.RootFS.DiffIDs[i] != parent.RootFS.DiffIDs[i] {
			childUsesCandidateDiffs = false
			break
		}
	}
	if !childUsesCandidateDiffs {
		return false
	}
	// the child should have the same history as the parent, plus
	// one more entry
	if len(parent.History)+1 != len(child.History) {
		return false
	}
	if historiesMatch(parent.History, child.History) != len(parent.History) {
		return false
	}
	return true
}

// GetParent returns the image ID of the parent. Return nil if a parent is not found.
func (i *Image) GetParent(ctx context.Context) (*Image, error) {
	var childLayer *storage.Layer
	images, err := i.imageruntime.GetImages()
	if err != nil {
		return nil, err
	}
	if i.TopLayer() != "" {
		if childLayer, err = i.imageruntime.store.Layer(i.TopLayer()); err != nil {
			return nil, err
		}
	}
	// fetch the configuration for the child image
	child, err := i.ociv1Image(ctx)
	if err != nil {
		if errors.Cause(err) == ErrImageIsBareList {
			return nil, nil
		}
		return nil, err
	}
	for _, img := range images {
		if img.ID() == i.ID() {
			continue
		}
		candidateLayer := img.TopLayer()
		// as a child, our top layer, if we have one, is either the
		// candidate parent's layer, or one that's derived from it, so
		// skip over any candidate image where we know that isn't the
		// case
		if childLayer != nil {
			// The child has at least one layer, so a parent would
			// have a top layer that's either the same as the child's
			// top layer or the top layer's recorded parent layer,
			// which could be an empty value.
			if candidateLayer != childLayer.Parent && candidateLayer != childLayer.ID {
				continue
			}
		} else {
			// The child has no layers, but the candidate does.
			if candidateLayer != "" {
				continue
			}
		}
		// fetch the configuration for the candidate image
		candidate, err := img.ociv1Image(ctx)
		if err != nil {
			return nil, err
		}
		// compare them
		if areParentAndChild(candidate, child) {
			return img, nil
		}
	}
	return nil, nil
}

// GetChildren returns a list of the imageIDs that depend on the image
func (i *Image) GetChildren(ctx context.Context) ([]string, error) {
	children, err := i.getChildren(ctx, 0)
	if err != nil {
		if errors.Cause(err) == ErrImageIsBareList {
			return nil, nil
		}
		return nil, err
	}
	return children, nil
}

// getChildren returns a list of at most "max" imageIDs that depend on the image
func (i *Image) getChildren(ctx context.Context, max int) ([]string, error) {
	var children []string

	if _, err := i.toImageRef(ctx); err != nil {
		return nil, nil
	}

	images, err := i.imageruntime.GetImages()
	if err != nil {
		return nil, err
	}

	// fetch the configuration for the parent image
	parent, err := i.ociv1Image(ctx)
	if err != nil {
		return nil, err
	}
	parentLayer := i.TopLayer()

	for _, img := range images {
		if img.ID() == i.ID() {
			continue
		}
		if img.TopLayer() == "" {
			if parentLayer != "" {
				// this image has no layers, but we do, so
				// it can't be derived from this one
				continue
			}
		} else {
			candidateLayer, err := img.Layer()
			if err != nil {
				return nil, err
			}
			// if this image's top layer is not our top layer, and is not
			// based on our top layer, we can skip it
			if candidateLayer.Parent != parentLayer && candidateLayer.ID != parentLayer {
				continue
			}
		}
		// fetch the configuration for the candidate image
		candidate, err := img.ociv1Image(ctx)
		if err != nil {
			return nil, err
		}
		// compare them
		if areParentAndChild(parent, candidate) {
			children = append(children, img.ID())
		}
		// if we're not building an exhaustive list, maybe we're done?
		if max > 0 && len(children) >= max {
			break
		}
	}
	return children, nil
}

// InputIsID returns a bool if the user input for an image
// is the image's partial or full id
func (i *Image) InputIsID() bool {
	return strings.HasPrefix(i.ID(), i.InputName)
}

// Containers a list of container IDs associated with the image
func (i *Image) Containers() ([]string, error) {
	containers, err := i.imageruntime.store.Containers()
	if err != nil {
		return nil, err
	}
	var imageContainers []string
	for _, c := range containers {
		if c.ImageID == i.ID() {
			imageContainers = append(imageContainers, c.ID)
		}
	}
	return imageContainers, err
}

// Comment returns the Comment for an image depending on its ManifestType
func (i *Image) Comment(ctx context.Context, manifestType string) (string, error) {
	if manifestType == manifest.DockerV2Schema2MediaType {
		imgRef, err := i.toImageRef(ctx)
		if err != nil {
			return "", errors.Wrapf(err, "unable to create image reference from image")
		}
		blob, err := imgRef.ConfigBlob(ctx)
		if err != nil {
			return "", errors.Wrapf(err, "unable to get config blob from image")
		}
		b := manifest.Schema2Image{}
		if err := json.Unmarshal(blob, &b); err != nil {
			return "", err
		}
		return b.Comment, nil
	}
	ociv1Img, err := i.ociv1Image(ctx)
	if err != nil {
		if errors.Cause(err) == ErrImageIsBareList {
			return "", nil
		}
		return "", err
	}
	if len(ociv1Img.History) > 0 {
		return ociv1Img.History[0].Comment, nil
	}
	return "", nil
}

// Save writes a container image to the filesystem
func (i *Image) Save(ctx context.Context, source, format, output string, moreTags []string, quiet, compress bool) error {
	var (
		writer       io.Writer
		destRef      types.ImageReference
		manifestType string
		err          error
	)

	if quiet {
		writer = os.Stderr
	}
	switch format {
	case "oci-archive":
		destImageName := imageNameForSaveDestination(i, source)
		destRef, err = ociarchive.NewReference(output, destImageName) // destImageName may be ""
		if err != nil {
			return errors.Wrapf(err, "error getting OCI archive ImageReference for (%q, %q)", output, destImageName)
		}
	case "oci-dir":
		destRef, err = directory.NewReference(output)
		if err != nil {
			return errors.Wrapf(err, "error getting directory ImageReference for %q", output)
		}
		manifestType = imgspecv1.MediaTypeImageManifest
	case "docker-dir":
		destRef, err = directory.NewReference(output)
		if err != nil {
			return errors.Wrapf(err, "error getting directory ImageReference for %q", output)
		}
		manifestType = manifest.DockerV2Schema2MediaType
	case "docker-archive", "":
		dst := output
		destImageName := imageNameForSaveDestination(i, source)
		if destImageName != "" {
			dst = fmt.Sprintf("%s:%s", dst, destImageName)
		}
		destRef, err = dockerarchive.ParseReference(dst) // FIXME? Add dockerarchive.NewReference
		if err != nil {
			return errors.Wrapf(err, "error getting Docker archive ImageReference for %q", dst)
		}
	default:
		return errors.Errorf("unknown format option %q", format)
	}
	// supports saving multiple tags to the same tar archive
	var additionaltags []reference.NamedTagged
	if len(moreTags) > 0 {
		additionaltags, err = GetAdditionalTags(moreTags)
		if err != nil {
			return err
		}
	}
	if err := i.PushImageToReference(ctx, destRef, manifestType, "", "", "", writer, compress, SigningOptions{}, &DockerRegistryOptions{}, additionaltags); err != nil {
		return errors.Wrapf(err, "unable to save %q", source)
	}
	i.newImageEvent(events.Save)
	return nil
}

// GetConfigBlob returns a schema2image.  If the image is not a schema2, then
// it will return an error
func (i *Image) GetConfigBlob(ctx context.Context) (*manifest.Schema2Image, error) {
	imageRef, err := i.toImageRef(ctx)
	if err != nil {
		return nil, err
	}
	b, err := imageRef.ConfigBlob(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get config blob for %s", i.ID())
	}
	blob := manifest.Schema2Image{}
	if err := json.Unmarshal(b, &blob); err != nil {
		return nil, errors.Wrapf(err, "unable to parse image blob for %s", i.ID())
	}
	return &blob, nil

}

// GetHealthCheck returns a HealthConfig for an image.  This function only works with
// schema2 images.
func (i *Image) GetHealthCheck(ctx context.Context) (*manifest.Schema2HealthConfig, error) {
	configBlob, err := i.GetConfigBlob(ctx)
	if err != nil {
		return nil, err
	}
	return configBlob.ContainerConfig.Healthcheck, nil
}

// newImageEvent creates a new event based on an image
func (ir *Runtime) newImageEvent(status events.Status, name string) {
	e := events.NewEvent(status)
	e.Type = events.Image
	e.Name = name
	if err := ir.Eventer.Write(e); err != nil {
		logrus.Infof("unable to write event to %s", ir.EventsLogFilePath)
	}
}

// newImageEvent creates a new event based on an image
func (i *Image) newImageEvent(status events.Status) {
	e := events.NewEvent(status)
	e.ID = i.ID()
	e.Type = events.Image
	if len(i.Names()) > 0 {
		e.Name = i.Names()[0]
	}
	if err := i.imageruntime.Eventer.Write(e); err != nil {
		logrus.Infof("unable to write event to %s", i.imageruntime.EventsLogFilePath)
	}
}

// LayerInfo keeps information of single layer
type LayerInfo struct {
	// Layer ID
	ID string
	// Parent ID of current layer.
	ParentID string
	// ChildID of current layer.
	// there can be multiple children in case of fork
	ChildID []string
	// RepoTag will have image repo names, if layer is top layer of image
	RepoTags []string
	// Size stores Uncompressed size of layer.
	Size int64
}

// GetLayersMapWithImageInfo returns map of image-layers, with associated information like RepoTags, parent and list of child layers.
func GetLayersMapWithImageInfo(imageruntime *Runtime) (map[string]*LayerInfo, error) {

	// Memory allocated to store map of layers with key LayerID.
	// Map will build dependency chain with ParentID and ChildID(s)
	layerInfoMap := make(map[string]*LayerInfo)

	// scan all layers & fill size and parent id for each layer in layerInfoMap
	layers, err := imageruntime.store.Layers()
	if err != nil {
		return nil, err
	}
	for _, layer := range layers {
		_, ok := layerInfoMap[layer.ID]
		if !ok {
			layerInfoMap[layer.ID] = &LayerInfo{
				ID:       layer.ID,
				Size:     layer.UncompressedSize,
				ParentID: layer.Parent,
			}
		} else {
			return nil, fmt.Errorf("detected multiple layers with the same ID %q", layer.ID)
		}
	}

	// scan all layers & add all childs for each layers to layerInfo
	for _, layer := range layers {
		_, ok := layerInfoMap[layer.ID]
		if ok {
			if layer.Parent != "" {
				layerInfoMap[layer.Parent].ChildID = append(layerInfoMap[layer.Parent].ChildID, layer.ID)
			}
		} else {
			return nil, fmt.Errorf("lookup error: layer-id  %s, not found", layer.ID)
		}
	}

	// Add the Repo Tags to Top layer of each image.
	imgs, err := imageruntime.store.Images()
	if err != nil {
		return nil, err
	}
	layerInfoMap[""] = &LayerInfo{}
	for _, img := range imgs {
		e, ok := layerInfoMap[img.TopLayer]
		if !ok {
			return nil, fmt.Errorf("top-layer for image %s not found local store", img.ID)
		}
		e.RepoTags = append(e.RepoTags, img.Names...)
	}
	return layerInfoMap, nil
}

// BuildImageHierarchyMap stores hierarchy of images such that all parent layers using which image is built are stored in imageInfo
// Layers are added such that  (Start)RootLayer->...intermediate Parent Layer(s)-> TopLayer(End)
func BuildImageHierarchyMap(imageInfo *InfoImage, layerMap map[string]*LayerInfo, layerID string) error {
	if layerID == "" {
		return nil
	}
	ll, ok := layerMap[layerID]
	if !ok {
		return fmt.Errorf("lookup error: layerid  %s not found", layerID)
	}
	if err := BuildImageHierarchyMap(imageInfo, layerMap, ll.ParentID); err != nil {
		return err
	}

	imageInfo.Layers = append(imageInfo.Layers, *ll)
	return nil
}
