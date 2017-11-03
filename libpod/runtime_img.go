package libpod

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	cp "github.com/containers/image/copy"
	dockerarchive "github.com/containers/image/docker/archive"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/docker/tarfile"
	ociarchive "github.com/containers/image/oci/archive"
	"github.com/containers/image/pkg/sysregistries"
	"github.com/containers/image/signature"
	is "github.com/containers/image/storage"
	"github.com/containers/image/tarball"
	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	digest "github.com/opencontainers/go-digest"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod/common"
)

// Runtime API

const (
	// DefaultRegistry is a prefix that we apply to an image name
	// to check docker hub first for the image
	DefaultRegistry = "docker://"
)

var (
	// DockerArchive is the transport we prepend to an image name
	// when saving to docker-archive
	DockerArchive = dockerarchive.Transport.Name()
	// OCIArchive is the transport we prepend to an image name
	// when saving to oci-archive
	OCIArchive = ociarchive.Transport.Name()
	// DirTransport is the transport for pushing and pulling
	// images to and from a directory
	DirTransport = "dir"
	// TransportNames are the supported transports in string form
	TransportNames = [...]string{DefaultRegistry, DockerArchive, OCIArchive, "ostree:", "dir:"}
	// TarballTransport is the transport for importing a tar archive
	// and creating a filesystem image
	TarballTransport = "tarball"
)

// CopyOptions contains the options given when pushing or pulling images
type CopyOptions struct {
	// Compression specifies the type of compression which is applied to
	// layer blobs.  The default is to not use compression, but
	// archive.Gzip is recommended.
	Compression archive.Compression
	// DockerRegistryOptions encapsulates settings that affect how we
	// connect or authenticate to a remote registry to which we want to
	// push the image.
	common.DockerRegistryOptions
	// SigningOptions encapsulates settings that control whether or not we
	// strip or add signatures to the image when pushing (uploading) the
	// image to a registry.
	common.SigningOptions

	// SigningPolicyPath this points to a alternative signature policy file, used mainly for testing
	SignaturePolicyPath string
	// AuthFile is the path of the cached credentials file defined by the user
	AuthFile string
	// Writer is the reportWriter for the output
	Writer io.Writer
	// Reference is the name for the image created when a tar archive is imported
	Reference string
	// ImageConfig is the Image spec for the image created when a tar archive is imported
	ImageConfig ociv1.Image
}

// Image API

// ImageFilterParams contains the filter options that may be given when outputting images
type ImageFilterParams struct {
	Dangling         string
	Label            string
	BeforeImage      time.Time
	SinceImage       time.Time
	ReferencePattern string
	ImageName        string
	ImageInput       string
}

// struct for when a user passes a short or incomplete
// image name
type imageDecomposeStruct struct {
	imageName   string
	tag         string
	registry    string
	hasRegistry bool
	transport   string
}

func (k *Image) assembleFqName() string {
	return fmt.Sprintf("%s/%s:%s", k.Registry, k.ImageName, k.Tag)
}

func (k *Image) assembleFqNameTransport() string {
	return fmt.Sprintf("%s%s/%s:%s", k.Transport, k.Registry, k.ImageName, k.Tag)
}

//Image describes basic attributes of an image
type Image struct {
	Name           string
	ID             string
	fqname         string
	hasImageLocal  bool //nolint
	runtime        *Runtime
	Registry       string
	ImageName      string
	Tag            string
	HasRegistry    bool
	Transport      string
	beenDecomposed bool
	PullName       string
	LocalName      string
}

// NewImage creates a new image object based on its name
func (r *Runtime) NewImage(name string) Image {
	return Image{
		Name:    name,
		runtime: r,
	}
}

// GetImageID returns the image ID of the image
func (k *Image) GetImageID() (string, error) {
	// If the ID field is already populated, then
	// return it.
	if k.ID != "" {
		return k.ID, nil
	}
	// If we have the name of the image locally, then
	// get the image and returns its ID
	if k.LocalName != "" {
		img, err := k.runtime.GetImage(k.LocalName)
		if err == nil {
			return img.ID, nil
		}
	}
	// If neither the ID is known and no local name
	// is know, we search it out.
	image, _ := k.GetFQName()
	img, err := k.runtime.GetImage(image)
	if err != nil {
		return "", err
	}
	return img.ID, nil
}

// GetFQName returns the fully qualified image name if it can be determined
func (k *Image) GetFQName() (string, error) {
	// Check if the fqname has already been found
	if k.fqname != "" {
		return k.fqname, nil
	}
	if err := k.Decompose(); err != nil {
		return "", err
	}
	k.fqname = k.assembleFqName()
	return k.fqname, nil
}

func (k *Image) findImageOnRegistry() error {
	searchRegistries, err := GetRegistries()

	if err != nil {
		return errors.Wrapf(err, " the image name '%s' is incomplete.", k.Name)
	}

	for _, searchRegistry := range searchRegistries {
		k.Registry = searchRegistry
		err = k.GetManifest()
		if err == nil {
			k.fqname = k.assembleFqName()
			return nil

		}
	}
	return errors.Errorf("unable to find image on any configured registries")

}

// GetManifest tries to GET an images manifest, returns nil on success and err on failure
func (k *Image) GetManifest() error {
	pullRef, err := alltransports.ParseImageName(k.assembleFqNameTransport())
	if err != nil {
		return errors.Errorf("unable to parse1 '%s'", k.assembleFqName())
	}
	imageSource, err := pullRef.NewImageSource(nil)
	if err != nil {
		return errors.Wrapf(err, "unable to create new image source")
	}
	_, _, err = imageSource.GetManifest()
	if err == nil {
		return nil
	}
	return err
}

//Decompose breaks up an image name into its parts
func (k *Image) Decompose() error {
	if k.beenDecomposed {
		return nil
	}
	k.beenDecomposed = true
	k.Transport = "docker://"
	decomposeName := k.Name
	for _, transport := range TransportNames {
		if strings.HasPrefix(k.Name, transport) {
			k.Transport = transport
			decomposeName = strings.Replace(k.Name, transport, "", -1)
			break
		}
	}
	if k.Transport == "dir:" {
		return nil
	}
	var imageError = fmt.Sprintf("unable to parse '%s'\n", decomposeName)
	imgRef, err := reference.Parse(decomposeName)
	if err != nil {
		return errors.Wrapf(err, imageError)
	}
	tagged, isTagged := imgRef.(reference.NamedTagged)
	k.Tag = "latest"
	if isTagged {
		k.Tag = tagged.Tag()
	}
	k.HasRegistry = true
	registry := reference.Domain(imgRef.(reference.Named))
	if registry == "" {
		k.HasRegistry = false
	}
	k.ImageName = reference.Path(imgRef.(reference.Named))

	// account for image names with directories in them like
	// umohnani/get-started:part1
	if k.HasRegistry {
		k.Registry = registry
		k.fqname = k.assembleFqName()
		k.PullName = k.assembleFqName()

		registries, err := getRegistries()
		if err != nil {
			return nil
		}
		if StringInSlice(k.Registry, registries) {
			return nil
		}
		// We need to check if the registry name is legit
		_, err = net.LookupAddr(k.Registry)
		if err == nil {
			return nil
		}
		// Combine the Registry and Image Name together and blank out the Registry Name
		k.ImageName = fmt.Sprintf("%s/%s", k.Registry, k.ImageName)
		k.Registry = ""

	}
	// No Registry means we check the globals registries configuration file
	// and assemble a list of candidate sources to try
	//searchRegistries, err := GetRegistries()
	err = k.findImageOnRegistry()
	k.PullName = k.assembleFqName()
	if err != nil {
		return errors.Wrapf(err, " the image name '%s' is incomplete.", k.Name)
	}
	return nil
}

func getTags(nameInput string) (reference.NamedTagged, bool, error) {
	inputRef, err := reference.Parse(nameInput)
	if err != nil {
		return nil, false, errors.Wrapf(err, "unable to obtain tag from input name")
	}
	tagged, isTagged := inputRef.(reference.NamedTagged)

	return tagged, isTagged, nil
}

// GetLocalImageName returns  the name of the image if it is local.
// It will return an empty string and error if not found.
func (k *Image) GetLocalImageName() (string, error) {
	_, err := k.runtime.GetImage(k.Name)
	if err != nil {
		return "", errors.Wrapf(err, "unable to obtain local image")
	}
	localImages, err := k.runtime.GetImages(&ImageFilterParams{})
	if err != nil {
		return "", errors.Wrapf(err, "unable to obtain local image")
	}
	_, isTagged, err := getTags(k.Name)
	if err != nil {
		return "", err
	}
	for _, image := range localImages {
		for _, name := range image.Names {
			imgRef, err := reference.Parse(name)
			if err != nil {
				continue
			}
			var imageName string
			imageNameOnly := reference.Path(imgRef.(reference.Named))
			if isTagged {
				imageNameTag, _, err := getTags(name)
				if err != nil {
					continue
				}
				imageName = fmt.Sprintf("%s:%s", imageNameOnly, imageNameTag.Tag())
			} else {
				imageName = imageNameOnly
			}

			if imageName == k.Name {
				return name, nil
			}
			imageSplit := strings.Split(imageName, "/")
			baseName := imageSplit[len(imageSplit)-1]
			if baseName == k.Name {
				return name, nil
			}
		}
	}
	fqname, _ := k.GetFQName()
	return fqname, nil
}

// HasLatest determines if we have the latest image local
func (k *Image) HasLatest() (bool, error) {
	localName, err := k.GetLocalImageName()
	if err != nil {
		return false, err
	}
	if localName == "" {
		return false, nil
	}

	fqname, err := k.GetFQName()
	if err != nil {
		return false, err
	}
	pullRef, err := alltransports.ParseImageName(fqname)
	if err != nil {
		return false, err
	}
	_, _, err = pullRef.(types.ImageSource).GetManifest()
	return false, err
}

// Pull is a wrapper function to pull and image
func (k *Image) Pull() error {
	// If the image hasn't been decomposed yet
	if !k.beenDecomposed {
		err := k.Decompose()
		if err != nil {
			return err
		}
	}
	k.runtime.PullImage(k.PullName, CopyOptions{Writer: os.Stdout, SignaturePolicyPath: k.runtime.config.SignaturePolicyPath})
	return nil
}

// GetRegistries gets the searchable registries from the global registration file.
func GetRegistries() ([]string, error) {
	registryConfigPath := ""
	envOverride := os.Getenv("REGISTRIES_CONFIG_PATH")
	if len(envOverride) > 0 {
		registryConfigPath = envOverride
	}
	searchRegistries, err := sysregistries.GetRegistries(&types.SystemContext{SystemRegistriesConfPath: registryConfigPath})
	if err != nil {
		return nil, errors.Errorf("unable to parse the registries.conf file")
	}
	return searchRegistries, nil
}

// GetInsecureRegistries obtains the list of inseure registries from the global registration file.
func GetInsecureRegistries() ([]string, error) {
	registryConfigPath := ""
	envOverride := os.Getenv("REGISTRIES_CONFIG_PATH")
	if len(envOverride) > 0 {
		registryConfigPath = envOverride
	}
	registries, err := sysregistries.GetInsecureRegistries(&types.SystemContext{SystemRegistriesConfPath: registryConfigPath})
	if err != nil {
		return nil, errors.Errorf("unable to parse the registries.conf file")
	}
	return registries, nil
}

// getRegistries returns both searchable and insecure registries from the global conf file.
func getRegistries() ([]string, error) {
	var r []string
	registries, err := GetRegistries()
	if err != nil {
		return r, err
	}
	insecureRegistries, err := GetInsecureRegistries()
	if err != nil {
		return r, err
	}
	r = append(registries, insecureRegistries...)
	return r, nil
}

// ImageFilter is a function to determine whether an image is included in
// command output. Images to be outputted are tested using the function. A true
// return will include the image, a false return will exclude it.
type ImageFilter func(*storage.Image, *types.ImageInspectInfo) bool

func (ips imageDecomposeStruct) returnFQName() string {
	return fmt.Sprintf("%s%s/%s:%s", ips.transport, ips.registry, ips.imageName, ips.tag)
}

func getRegistriesToTry(image string, store storage.Store) ([]*pullStruct, error) {
	var pStructs []*pullStruct
	var imageError = fmt.Sprintf("unable to parse '%s'\n", image)
	imgRef, err := reference.Parse(image)
	if err != nil {
		return nil, errors.Wrapf(err, imageError)
	}
	tagged, isTagged := imgRef.(reference.NamedTagged)
	tag := "latest"
	if isTagged {
		tag = tagged.Tag()
	}
	hasDomain := true
	registry := reference.Domain(imgRef.(reference.Named))
	if registry == "" {
		hasDomain = false
	}
	imageName := reference.Path(imgRef.(reference.Named))
	pImage := imageDecomposeStruct{
		imageName,
		tag,
		registry,
		hasDomain,
		"docker://",
	}
	if pImage.hasRegistry {
		// If input has a registry, we have to assume they included an image
		// name but maybe not a tag
		srcRef, err := alltransports.ParseImageName(pImage.returnFQName())
		if err != nil {
			return nil, errors.Errorf(imageError)
		}
		pStruct := &pullStruct{
			image:  srcRef.DockerReference().String(),
			srcRef: srcRef,
		}
		pStructs = append(pStructs, pStruct)
	} else {
		// No registry means we check the globals registries configuration file
		// and assemble a list of candidate sources to try
		registryConfigPath := ""
		envOverride := os.Getenv("REGISTRIES_CONFIG_PATH")
		if len(envOverride) > 0 {
			registryConfigPath = envOverride
		}
		searchRegistries, err := sysregistries.GetRegistries(&types.SystemContext{SystemRegistriesConfPath: registryConfigPath})
		if err != nil {
			return nil, errors.Wrapf(err, "unable to parse the registries.conf file and"+
				" the image name '%s' is incomplete.", imageName)
		}
		for _, searchRegistry := range searchRegistries {
			pImage.registry = searchRegistry
			srcRef, err := alltransports.ParseImageName(pImage.returnFQName())
			if err != nil {
				return nil, errors.Errorf("unable to parse '%s'", pImage.returnFQName())
			}
			pStruct := &pullStruct{
				image:  srcRef.DockerReference().String(),
				srcRef: srcRef,
			}
			pStructs = append(pStructs, pStruct)
		}
	}

	for _, pStruct := range pStructs {
		destRef, err := is.Transport.ParseStoreReference(store, pStruct.image)
		if err != nil {
			return nil, errors.Errorf("error parsing dest reference name: %v", err)
		}
		pStruct.dstRef = destRef
	}
	return pStructs, nil
}

type pullStruct struct {
	image  string
	srcRef types.ImageReference
	dstRef types.ImageReference
}

func (r *Runtime) getPullStruct(srcRef types.ImageReference, destName string) (*pullStruct, error) {
	reference := destName
	if srcRef.DockerReference() != nil {
		reference = srcRef.DockerReference().String()
	}
	destRef, err := is.Transport.ParseStoreReference(r.store, reference)
	if err != nil {
		return nil, errors.Errorf("error parsing dest reference name: %v", err)
	}
	return &pullStruct{
		image:  destName,
		srcRef: srcRef,
		dstRef: destRef,
	}, nil
}

// returns a list of pullStruct with the srcRef and DstRef based on the transport being used
func (r *Runtime) getPullListFromRef(srcRef types.ImageReference, imgName string, sc *types.SystemContext) ([]*pullStruct, error) {
	var pullStructs []*pullStruct
	splitArr := strings.Split(imgName, ":")
	archFile := splitArr[len(splitArr)-1]

	// supports pulling from docker-archive, oci, and registries
	if srcRef.Transport().Name() == DockerArchive {
		tarSource := tarfile.NewSource(archFile)
		manifest, err := tarSource.LoadTarManifest()
		if err != nil {
			return nil, errors.Errorf("error retrieving manifest.json: %v", err)
		}
		// to pull the first image stored in the tar file
		if len(manifest) == 0 {
			// use the hex of the digest if no manifest is found
			reference, err := getImageDigest(srcRef, sc)
			if err != nil {
				return nil, err
			}
			pullInfo, err := r.getPullStruct(srcRef, reference)
			if err != nil {
				return nil, err
			}
			pullStructs = append(pullStructs, pullInfo)
		} else {
			pullInfo, err := r.getPullStruct(srcRef, manifest[0].RepoTags[0])
			if err != nil {
				return nil, err
			}
			pullStructs = append(pullStructs, pullInfo)
		}
	} else if srcRef.Transport().Name() == OCIArchive {
		// retrieve the manifest from index.json to access the image name
		manifest, err := ociarchive.LoadManifestDescriptor(srcRef)
		if err != nil {
			return nil, errors.Wrapf(err, "error loading manifest for %q", srcRef)
		}

		if manifest.Annotations == nil || manifest.Annotations["org.opencontainers.image.ref.name"] == "" {
			return nil, errors.Errorf("error, archive doesn't have a name annotation. Cannot store image with no name")
		}
		pullInfo, err := r.getPullStruct(srcRef, manifest.Annotations["org.opencontainers.image.ref.name"])
		if err != nil {
			return nil, err
		}
		pullStructs = append(pullStructs, pullInfo)
	} else if srcRef.Transport().Name() == DirTransport {
		// supports pull from a directory
		image := splitArr[1]
		// remove leading "/"
		if image[:1] == "/" {
			image = image[1:]
		}
		pullInfo, err := r.getPullStruct(srcRef, image)
		if err != nil {
			return nil, err
		}
		pullStructs = append(pullStructs, pullInfo)
	} else {
		pullInfo, err := r.getPullStruct(srcRef, imgName)
		if err != nil {
			return nil, err
		}
		pullStructs = append(pullStructs, pullInfo)
	}
	return pullStructs, nil
}

// PullImage pulls an image from configured registries
// By default, only the latest tag (or a specific tag if requested) will be
// pulled. If allTags is true, all tags for the requested image will be pulled.
// Signature validation will be performed if the Runtime has been appropriately
// configured
func (r *Runtime) PullImage(imgName string, options CopyOptions) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	// PullImage copies the image from the source to the destination
	var pullStructs []*pullStruct

	signaturePolicyPath := r.config.SignaturePolicyPath
	if options.SignaturePolicyPath != "" {
		signaturePolicyPath = options.SignaturePolicyPath
	}

	sc := common.GetSystemContext(signaturePolicyPath, options.AuthFile)

	srcRef, err := alltransports.ParseImageName(imgName)
	if err != nil {
		// could be trying to pull from registry with short name
		pullStructs, err = getRegistriesToTry(imgName, r.store)
		if err != nil {
			return errors.Wrap(err, "error getting default registries to try")
		}
	} else {
		pullStructs, err = r.getPullListFromRef(srcRef, imgName, sc)
		if err != nil {
			return errors.Wrapf(err, "error getting pullStruct info to pull image %q", imgName)
		}
	}

	policyContext, err := getPolicyContext(sc)
	if err != nil {
		return err
	}
	defer policyContext.Destroy()

	copyOptions := common.GetCopyOptions(options.Writer, signaturePolicyPath, &options.DockerRegistryOptions, nil, options.SigningOptions, options.AuthFile)

	for _, imageInfo := range pullStructs {
		if options.Writer != nil {
			io.WriteString(options.Writer, fmt.Sprintf("Trying to pull %s...\n", imageInfo.image))
		}
		if err = cp.Image(policyContext, imageInfo.dstRef, imageInfo.srcRef, copyOptions); err != nil {
			if options.Writer != nil {
				io.WriteString(options.Writer, "Failed\n")
			}
		} else {
			return nil
		}
	}
	return errors.Wrapf(err, "error pulling image from %q", imgName)
}

// PushImage pushes the given image to a location described by the given path
func (r *Runtime) PushImage(source string, destination string, options CopyOptions) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	// PushImage pushes the src image to the destination
	//func PushImage(source, destination string, options CopyOptions) error {
	if source == "" || destination == "" {
		return errors.Wrapf(syscall.EINVAL, "source and destination image names must be specified")
	}

	// Get the destination Image Reference
	dest, err := alltransports.ParseImageName(destination)
	if err != nil {
		return errors.Wrapf(err, "error getting destination imageReference for %q", destination)
	}

	signaturePolicyPath := r.config.SignaturePolicyPath
	if options.SignaturePolicyPath != "" {
		signaturePolicyPath = options.SignaturePolicyPath
	}

	sc := common.GetSystemContext(signaturePolicyPath, options.AuthFile)

	policyContext, err := getPolicyContext(sc)
	if err != nil {
		return err
	}
	defer policyContext.Destroy()

	// Look up the source image, expecting it to be in local storage
	src, err := is.Transport.ParseStoreReference(r.store, source)
	if err != nil {
		return errors.Wrapf(err, "error getting source imageReference for %q", source)
	}

	copyOptions := common.GetCopyOptions(options.Writer, signaturePolicyPath, nil, &options.DockerRegistryOptions, options.SigningOptions, options.AuthFile)

	// Copy the image to the remote destination
	err = cp.Image(policyContext, dest, src, copyOptions)
	if err != nil {
		return errors.Wrapf(err, "Error copying image to the remote destination")
	}
	return nil
}

// TagImage adds a tag to the given image
func (r *Runtime) TagImage(image *storage.Image, tag string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	tags, err := r.store.Names(image.ID)
	if err != nil {
		return err
	}
	for _, key := range tags {
		if key == tag {
			return nil
		}
	}
	tags = append(tags, tag)
	return r.store.SetNames(image.ID, tags)
}

// UntagImage removes a tag from the given image
func (r *Runtime) UntagImage(image *storage.Image, tag string) (string, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return "", ErrRuntimeStopped
	}

	tags, err := r.store.Names(image.ID)
	if err != nil {
		return "", err
	}
	for i, key := range tags {
		if key == tag {
			tags[i] = tags[len(tags)-1]
			tags = tags[:len(tags)-1]
			break
		}
	}
	if err = r.store.SetNames(image.ID, tags); err != nil {
		return "", err
	}
	return tag, nil
}

// RemoveImage deletes an image from local storage
// Images being used by running containers can only be removed if force=true
func (r *Runtime) RemoveImage(image *storage.Image, force bool) (string, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return "", ErrRuntimeStopped
	}

	containersWithImage, err := r.getContainersWithImage(image.ID)
	if err != nil {
		return "", errors.Wrapf(err, "error getting containers for image %q", image.ID)
	}
	if len(containersWithImage) > 0 && len(image.Names) <= 1 {
		if force {
			if err := r.removeMultipleContainers(containersWithImage); err != nil {
				return "", err
			}
		} else {
			for _, ctr := range containersWithImage {
				return "", fmt.Errorf("Could not remove image %q (must force) - container %q is using its reference image", image.ID, ctr.ImageID)
			}
		}
	}

	if len(image.Names) > 1 && !force {
		return "", fmt.Errorf("unable to delete %s (must force) - image is referred to in multiple tags", image.ID)
	}
	// If it is forced, we have to untag the image so that it can be deleted
	image.Names = image.Names[:0]

	_, err = r.store.DeleteImage(image.ID, true)
	if err != nil {
		return "", err
	}
	return image.ID, nil
}

// GetImage retrieves an image matching the given name or hash from system
// storage
// If no matching image can be found, an error is returned
func (r *Runtime) GetImage(image string) (*storage.Image, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}
	return r.getImage(image)
}

func (r *Runtime) getImage(image string) (*storage.Image, error) {
	var img *storage.Image
	ref, err := is.Transport.ParseStoreReference(r.store, image)
	if err == nil {
		img, err = is.Transport.GetStoreImage(r.store, ref)
	}
	if err != nil {
		img2, err2 := r.store.Image(image)
		if err2 != nil {
			if ref == nil {
				return nil, errors.Wrapf(err, "error parsing reference to image %q", image)
			}
			return nil, errors.Wrapf(err, "unable to locate image %q", image)
		}
		img = img2
	}
	return img, nil
}

// GetImageRef searches for and returns a new types.Image matching the given name or ID in the given store.
func (r *Runtime) GetImageRef(image string) (types.Image, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}
	return r.getImageRef(image)

}

func (r *Runtime) getImageRef(image string) (types.Image, error) {
	img, err := r.getImage(image)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to locate image %q", image)
	}
	ref, err := is.Transport.ParseStoreReference(r.store, "@"+img.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing reference to image %q", img.ID)
	}
	imgRef, err := ref.NewImage(nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading image %q", img.ID)
	}
	return imgRef, nil
}

// GetImages retrieves all images present in storage
// Filters can be provided which will determine which images are included in the
// output. Multiple filters are handled by ANDing their output, so only images
// matching all filters are included
func (r *Runtime) GetImages(params *ImageFilterParams, filters ...ImageFilter) ([]*storage.Image, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	images, err := r.store.Images()
	if err != nil {
		return nil, err
	}

	var imagesFiltered []*storage.Image

	for _, img := range images {
		info, err := r.getImageInspectInfo(img)
		if err != nil {
			return nil, err
		}
		var names []string
		if len(img.Names) > 0 {
			names = img.Names
		} else {
			names = append(names, "<none>")
		}
		for _, name := range names {
			include := true
			if params != nil {
				params.ImageName = name
			}
			for _, filter := range filters {
				include = include && filter(&img, info)
			}

			if include {
				newImage := img
				newImage.Names = []string{name}
				imagesFiltered = append(imagesFiltered, &newImage)
			}
		}
	}

	return imagesFiltered, nil
}

// GetHistory gets the history of an image and information about its layers
func (r *Runtime) GetHistory(image string) ([]ociv1.History, []types.BlobInfo, string, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, nil, "", ErrRuntimeStopped
	}

	img, err := r.getImage(image)
	if err != nil {
		return nil, nil, "", errors.Wrapf(err, "no such image %q", image)
	}

	src, err := r.getImageRef(image)
	if err != nil {
		return nil, nil, "", errors.Wrapf(err, "error instantiating image %q", image)
	}

	oci, err := src.OCIConfig()
	if err != nil {
		return nil, nil, "", err
	}

	return oci.History, src.LayerInfos(), img.ID, nil
}

// ImportImage imports an OCI format image archive into storage as an image
func (r *Runtime) ImportImage(path string, options CopyOptions) error {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return ErrRuntimeStopped
	}

	file := TarballTransport + ":" + path
	src, err := alltransports.ParseImageName(file)
	if err != nil {
		return errors.Wrapf(err, "error parsing image name %q", path)
	}

	updater, ok := src.(tarball.ConfigUpdater)
	if !ok {
		return errors.Wrapf(err, "unexpected type, a tarball reference should implement tarball.ConfigUpdater")
	}

	annotations := make(map[string]string)

	err = updater.ConfigUpdate(options.ImageConfig, annotations)
	if err != nil {
		return errors.Wrapf(err, "error updating image config")
	}

	var reference = options.Reference
	sc := common.GetSystemContext("", "")

	// if reference not given, get the image digest
	if reference == "" {
		reference, err = getImageDigest(src, sc)
		if err != nil {
			return err
		}
	}

	policyContext, err := getPolicyContext(sc)
	if err != nil {
		return err
	}
	defer policyContext.Destroy()

	copyOptions := common.GetCopyOptions(os.Stdout, "", nil, nil, common.SigningOptions{}, "")

	dest, err := is.Transport.ParseStoreReference(r.store, reference)
	if err != nil {
		errors.Wrapf(err, "error getting image reference for %q", options.Reference)
	}

	return cp.Image(policyContext, dest, src, copyOptions)
}

// GetImageInspectInfo returns the inspect information of an image
func (r *Runtime) GetImageInspectInfo(image storage.Image) (*types.ImageInspectInfo, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}
	return r.getImageInspectInfo(image)
}

func (r *Runtime) getImageInspectInfo(image storage.Image) (*types.ImageInspectInfo, error) {
	img, err := r.getImageRef(image.ID)
	if err != nil {
		return nil, err
	}
	return img.Inspect()
}

// ParseImageFilter takes a set of images and a filter string as input, and returns the libpod.ImageFilterParams struct
func (r *Runtime) ParseImageFilter(imageInput, filter string) (*ImageFilterParams, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	if filter == "" && imageInput == "" {
		return nil, nil
	}

	var params ImageFilterParams
	params.ImageInput = imageInput

	if filter == "" && imageInput != "" {
		return &params, nil
	}

	images, err := r.store.Images()
	if err != nil {
		return nil, err
	}

	filterStrings := strings.Split(filter, ",")
	for _, param := range filterStrings {
		pair := strings.SplitN(param, "=", 2)
		switch strings.TrimSpace(pair[0]) {
		case "dangling":
			if common.IsValidBool(pair[1]) {
				params.Dangling = pair[1]
			} else {
				return nil, fmt.Errorf("invalid filter: '%s=[%s]'", pair[0], pair[1])
			}
		case "label":
			params.Label = pair[1]
		case "before":
			if img, err := findImageInSlice(images, pair[1]); err == nil {
				info, err := r.GetImageInspectInfo(img)
				if err != nil {
					return nil, err
				}
				params.BeforeImage = info.Created
			} else {
				return nil, fmt.Errorf("no such id: %s", pair[0])
			}
		case "since":
			if img, err := findImageInSlice(images, pair[1]); err == nil {
				info, err := r.GetImageInspectInfo(img)
				if err != nil {
					return nil, err
				}
				params.SinceImage = info.Created
			} else {
				return nil, fmt.Errorf("no such id: %s``", pair[0])
			}
		case "reference":
			params.ReferencePattern = pair[1]
		default:
			return nil, fmt.Errorf("invalid filter: '%s'", pair[0])
		}
	}
	return &params, nil
}

// InfoAndDigestAndSize returns the inspection info and size of the image in the given
// store and the digest of its manifest, if it has one, or "" if it doesn't.
func (r *Runtime) InfoAndDigestAndSize(img storage.Image) (*types.ImageInspectInfo, digest.Digest, int64, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if !r.valid {
		return nil, "", -1, ErrRuntimeStopped
	}

	imgRef, err := r.getImageRef("@" + img.ID)
	if err != nil {
		return nil, "", -1, errors.Wrapf(err, "error reading image %q", img.ID)
	}
	defer imgRef.Close()
	return infoAndDigestAndSize(imgRef)
}

func infoAndDigestAndSize(imgRef types.Image) (*types.ImageInspectInfo, digest.Digest, int64, error) {
	imgSize, err := imgRef.Size()
	if err != nil {
		return nil, "", -1, errors.Wrapf(err, "error reading size of image %q", transports.ImageName(imgRef.Reference()))
	}
	manifest, _, err := imgRef.Manifest()
	if err != nil {
		return nil, "", -1, errors.Wrapf(err, "error reading manifest for image %q", transports.ImageName(imgRef.Reference()))
	}
	manifestDigest := digest.Digest("")
	if len(manifest) > 0 {
		manifestDigest = digest.Canonical.FromBytes(manifest)
	}
	info, err := imgRef.Inspect()
	if err != nil {
		return nil, "", -1, errors.Wrapf(err, "error inspecting image %q", transports.ImageName(imgRef.Reference()))
	}
	return info, manifestDigest, imgSize, nil
}

// MatchesID returns true if argID is a full or partial match for id
func MatchesID(id, argID string) bool {
	return strings.HasPrefix(argID, id)
}

// MatchesReference returns true if argName is a full or partial match for name
// Partial matches will register only if they match the most specific part of the name available
// For example, take the image docker.io/library/redis:latest
// redis, library/redis, docker.io/library/redis, redis:latest, etc. will match
// But redis:alpine, ry/redis, library, and io/library/redis will not
func MatchesReference(name, argName string) bool {
	if argName == "" {
		return false
	}
	splitName := strings.Split(name, ":")
	// If the arg contains a tag, we handle it differently than if it does not
	if strings.Contains(argName, ":") {
		splitArg := strings.Split(argName, ":")
		return strings.HasSuffix(splitName[0], splitArg[0]) && (splitName[1] == splitArg[1])
	}
	return strings.HasSuffix(splitName[0], argName)
}

// ParseImageNames parses the names we've stored with an image into a list of
// tagged references and a list of references which contain digests.
func ParseImageNames(names []string) (tags, digests []string, err error) {
	for _, name := range names {
		if named, err := reference.ParseNamed(name); err == nil {
			if digested, ok := named.(reference.Digested); ok {
				canonical, err := reference.WithDigest(named, digested.Digest())
				if err == nil {
					digests = append(digests, canonical.String())
				}
			} else {
				if reference.IsNameOnly(named) {
					named = reference.TagNameOnly(named)
				}
				if tagged, ok := named.(reference.Tagged); ok {
					namedTagged, err := reference.WithTag(named, tagged.Tag())
					if err == nil {
						tags = append(tags, namedTagged.String())
					}
				}
			}
		}
	}
	return tags, digests, nil
}

// Remove nolint when used
func annotations(manifest []byte, manifestType string) map[string]string { //nolint
	annotations := make(map[string]string)
	switch manifestType {
	case ociv1.MediaTypeImageManifest:
		var m ociv1.Manifest
		if err := json.Unmarshal(manifest, &m); err == nil {
			for k, v := range m.Annotations {
				annotations[k] = v
			}
		}
	}
	return annotations
}

func findImageInSlice(images []storage.Image, ref string) (storage.Image, error) {
	for _, image := range images {
		if MatchesID(image.ID, ref) {
			return image, nil
		}
		for _, name := range image.Names {
			if MatchesReference(name, ref) {
				return image, nil
			}
		}
	}
	return storage.Image{}, errors.New("could not find image")
}

// getImageDigest creates an image object and uses the hex value of the digest as the image ID
// for parsing the store reference
func getImageDigest(src types.ImageReference, ctx *types.SystemContext) (string, error) {
	newImg, err := src.NewImage(ctx)
	if err != nil {
		return "", err
	}
	defer newImg.Close()

	digest := newImg.ConfigInfo().Digest
	if err = digest.Validate(); err != nil {
		return "", errors.Wrapf(err, "error getting config info")
	}
	return "@" + digest.Hex(), nil
}

// getPolicyContext sets up, intializes and returns a new context for the specified policy
func getPolicyContext(ctx *types.SystemContext) (*signature.PolicyContext, error) {
	policy, err := signature.DefaultPolicy(ctx)
	if err != nil {
		return nil, err
	}

	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, err
	}
	return policyContext, nil
}
