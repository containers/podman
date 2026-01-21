package manifests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/containers/common/pkg/manifests"
	"github.com/containers/common/pkg/retry"
	"github.com/containers/common/pkg/supplemented"
	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/signature/signer"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/lockfile"
	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

const (
	defaultMaxRetries = 3
)

const instancesData = "instances.json"

// LookupReferenceFunc return an image reference based on the specified one.
// The returned reference can return custom ImageSource or ImageDestination
// objects which intercept or filter blobs, manifests, and signatures as
// they are read and written.
type LookupReferenceFunc func(ref types.ImageReference) (types.ImageReference, error)

// ErrListImageUnknown is returned when we attempt to create an image reference
// for a List that has not yet been saved to an image.
var ErrListImageUnknown = errors.New("unable to determine which image holds the manifest list")

type list struct {
	manifests.List
	instances map[digest.Digest]string
}

// List is a manifest list or image index, either created using Create(), or
// loaded from local storage using LoadFromImage().
type List interface {
	manifests.List
	SaveToImage(store storage.Store, imageID string, names []string, mimeType string) (string, error)
	Reference(store storage.Store, multiple cp.ImageListSelection, instances []digest.Digest) (types.ImageReference, error)
	Push(ctx context.Context, dest types.ImageReference, options PushOptions) (reference.Canonical, digest.Digest, error)
	Add(ctx context.Context, sys *types.SystemContext, ref types.ImageReference, all bool) (digest.Digest, error)
}

// PushOptions includes various settings which are needed for pushing the
// manifest list and its instances.
type PushOptions struct {
	Store                            storage.Store
	SystemContext                    *types.SystemContext  // github.com/containers/image/types.SystemContext
	ImageListSelection               cp.ImageListSelection // set to either CopySystemImage, CopyAllImages, or CopySpecificImages
	Instances                        []digest.Digest       // instances to copy if ImageListSelection == CopySpecificImages
	ReportWriter                     io.Writer             // will be used to log the writing of the list and any blobs
	Signers                          []*signer.Signer      // if non-empty, asks for signatures to be added during the copy using the provided signers.
	SignBy                           string                // fingerprint of GPG key to use to sign images
	SignPassphrase                   string                // passphrase to use when signing with the key ID from SignBy.
	SignBySigstorePrivateKeyFile     string                // if non-empty, asks for a signature to be added during the copy, using a sigstore private key file at the provided path.
	SignSigstorePrivateKeyPassphrase []byte                // passphrase to use when signing with SignBySigstorePrivateKeyFile.
	RemoveSignatures                 bool                  // true to discard signatures in images
	ManifestType                     string                // the format to use when saving the list - possible options are oci, v2s1, and v2s2
	SourceFilter                     LookupReferenceFunc   // filter the list source
	AddCompression                   []string              // add existing instances with requested compression algorithms to manifest list
	ForceCompressionFormat           bool                  // force push with requested compression ignoring the blobs which can be reused.
	// Maximum number of retries with exponential backoff when facing
	// transient network errors. Default 3.
	MaxRetries *uint
	// RetryDelay used for the exponential back off of MaxRetries.
	RetryDelay *time.Duration
}

// Create creates a new list containing information about the specified image,
// computing its manifest's digest, and retrieving OS and architecture
// information from its configuration blob.  Returns the new list, and the
// instanceDigest for the initial image.
func Create() List {
	return &list{
		List:      manifests.Create(),
		instances: make(map[digest.Digest]string),
	}
}

// LoadFromImage reads the manifest list or image index, and additional
// information about where the various instances that it contains live, from an
// image record with the specified ID in local storage.
func LoadFromImage(store storage.Store, image string) (string, List, error) {
	img, err := store.Image(image)
	if err != nil {
		return "", nil, fmt.Errorf("locating image %q for loading manifest list: %w", image, err)
	}
	manifestBytes, err := store.ImageBigData(img.ID, storage.ImageDigestManifestBigDataNamePrefix)
	if err != nil {
		return "", nil, fmt.Errorf("locating image %q for loading manifest list: %w", image, err)
	}
	manifestList, err := manifests.FromBlob(manifestBytes)
	if err != nil {
		return "", nil, err
	}
	list := &list{
		List:      manifestList,
		instances: make(map[digest.Digest]string),
	}
	instancesBytes, err := store.ImageBigData(img.ID, instancesData)
	if err != nil {
		return "", nil, fmt.Errorf("locating image %q for loading instance list: %w", image, err)
	}
	if err := json.Unmarshal(instancesBytes, &list.instances); err != nil {
		return "", nil, fmt.Errorf("decoding instance list for image %q: %w", image, err)
	}
	list.instances[""] = img.ID
	return img.ID, list, err
}

// SaveToImage saves the manifest list or image index as the manifest of an
// Image record with the specified names in local storage, generating a random
// image ID if none is specified.  It also stores information about where the
// images whose manifests are included in the list can be found.
func (l *list) SaveToImage(store storage.Store, imageID string, names []string, mimeType string) (string, error) {
	manifestBytes, err := l.List.Serialize(mimeType)
	if err != nil {
		return "", err
	}
	instancesBytes, err := json.Marshal(&l.instances)
	if err != nil {
		return "", err
	}
	img, err := store.CreateImage(imageID, names, "", "", &storage.ImageOptions{})
	if err == nil || errors.Is(err, storage.ErrDuplicateID) {
		created := (err == nil)
		if created {
			imageID = img.ID
			l.instances[""] = img.ID
		}
		err := store.SetImageBigData(imageID, storage.ImageDigestManifestBigDataNamePrefix, manifestBytes, manifest.Digest)
		if err != nil {
			if created {
				if _, err2 := store.DeleteImage(img.ID, true); err2 != nil {
					logrus.Errorf("Deleting image %q after failing to save manifest for it", img.ID)
				}
			}
			return "", fmt.Errorf("saving manifest list to image %q: %w", imageID, err)
		}
		err = store.SetImageBigData(imageID, instancesData, instancesBytes, nil)
		if err != nil {
			if created {
				if _, err2 := store.DeleteImage(img.ID, true); err2 != nil {
					logrus.Errorf("Deleting image %q after failing to save instance locations for it", img.ID)
				}
			}
			return "", fmt.Errorf("saving instance list to image %q: %w", imageID, err)
		}
		return imageID, nil
	}
	return "", fmt.Errorf("creating image to hold manifest list: %w", err)
}

// Reference returns an image reference for the composite image being built
// in the list, or an error if the list has never been saved to a local image.
func (l *list) Reference(store storage.Store, multiple cp.ImageListSelection, instances []digest.Digest) (types.ImageReference, error) {
	if l.instances[""] == "" {
		return nil, fmt.Errorf("building reference to list: %w", ErrListImageUnknown)
	}
	s, err := is.Transport.ParseStoreReference(store, l.instances[""])
	if err != nil {
		return nil, fmt.Errorf("creating ImageReference from image %q: %w", l.instances[""], err)
	}
	references := make([]types.ImageReference, 0, len(l.instances))
	whichInstances := make([]digest.Digest, 0, len(l.instances))
	switch multiple {
	case cp.CopyAllImages, cp.CopySystemImage:
		for instance := range l.instances {
			if instance != "" {
				whichInstances = append(whichInstances, instance)
			}
		}
	case cp.CopySpecificImages:
		for instance := range l.instances {
			for _, allowed := range instances {
				if instance == allowed {
					whichInstances = append(whichInstances, instance)
				}
			}
		}
	}
	for _, instance := range whichInstances {
		imageName := l.instances[instance]
		ref, err := alltransports.ParseImageName(imageName)
		if err != nil {
			return nil, fmt.Errorf("creating ImageReference from image %q: %w", imageName, err)
		}
		references = append(references, ref)
	}
	return supplemented.Reference(s, references, multiple, instances), nil
}

// Push saves the manifest list and whichever blobs are needed to a destination location.
func (l *list) Push(ctx context.Context, dest types.ImageReference, options PushOptions) (reference.Canonical, digest.Digest, error) {
	// Load the system signing policy.
	pushPolicy, err := signature.DefaultPolicy(options.SystemContext)
	if err != nil {
		return nil, "", fmt.Errorf("obtaining default signature policy: %w", err)
	}

	// Override the settings for local storage to make sure that we can always read the source "image".
	pushPolicy.Transports[is.Transport.Name()] = storageAllowedPolicyScopes

	policyContext, err := signature.NewPolicyContext(pushPolicy)
	if err != nil {
		return nil, "", fmt.Errorf("creating new signature policy context: %w", err)
	}
	defer func() {
		if err2 := policyContext.Destroy(); err2 != nil {
			logrus.Errorf("Destroying signature policy context: %v", err2)
		}
	}()

	// If we were given a media type that corresponds to a multiple-images
	// type, reset it to a valid corresponding single-image type, since we
	// already expect the image library to infer the list type from the
	// image type that we're telling it to force.
	singleImageManifestType := options.ManifestType
	switch singleImageManifestType {
	case v1.MediaTypeImageIndex:
		singleImageManifestType = v1.MediaTypeImageManifest
	case manifest.DockerV2ListMediaType:
		singleImageManifestType = manifest.DockerV2Schema2MediaType
	}

	// Build a source reference for our list and grab bag full of blobs.
	src, err := l.Reference(options.Store, options.ImageListSelection, options.Instances)
	if err != nil {
		return nil, "", err
	}
	if options.SourceFilter != nil {
		if src, err = options.SourceFilter(src); err != nil {
			return nil, "", err
		}
	}
	compressionVariants, err := prepareAddWithCompression(options.AddCompression)
	if err != nil {
		return nil, "", err
	}
	copyOptions := &cp.Options{
		ImageListSelection:               options.ImageListSelection,
		Instances:                        options.Instances,
		SourceCtx:                        options.SystemContext,
		DestinationCtx:                   options.SystemContext,
		ReportWriter:                     options.ReportWriter,
		RemoveSignatures:                 options.RemoveSignatures,
		Signers:                          options.Signers,
		SignBy:                           options.SignBy,
		SignPassphrase:                   options.SignPassphrase,
		SignBySigstorePrivateKeyFile:     options.SignBySigstorePrivateKeyFile,
		SignSigstorePrivateKeyPassphrase: options.SignSigstorePrivateKeyPassphrase,
		ForceManifestMIMEType:            singleImageManifestType,
		EnsureCompressionVariantsExist:   compressionVariants,
		ForceCompressionFormat:           options.ForceCompressionFormat,
	}

	retryOptions := retry.Options{}
	retryOptions.MaxRetry = defaultMaxRetries
	if options.MaxRetries != nil {
		retryOptions.MaxRetry = int(*options.MaxRetries)
	}
	if options.RetryDelay != nil {
		retryOptions.Delay = *options.RetryDelay
	}

	// Copy whatever we were asked to copy.
	var manifestDigest digest.Digest
	f := func() error {
		opts := copyOptions
		var manifestBytes []byte
		var digest digest.Digest
		var err error
		if manifestBytes, err = cp.Image(ctx, policyContext, dest, src, opts); err == nil {
			if digest, err = manifest.Digest(manifestBytes); err == nil {
				manifestDigest = digest
			}
		}
		return err
	}
	err = retry.IfNecessary(ctx, f, &retryOptions)
	return nil, manifestDigest, err
}

func prepareAddWithCompression(variants []string) ([]cp.OptionCompressionVariant, error) {
	res := []cp.OptionCompressionVariant{}
	for _, name := range variants {
		algo, err := compression.AlgorithmByName(name)
		if err != nil {
			return nil, fmt.Errorf("requested algorithm %s is not supported for replication: %w", name, err)
		}
		res = append(res, cp.OptionCompressionVariant{Algorithm: algo})
	}
	return res, nil
}

// Add adds information about the specified image to the list, computing the
// image's manifest's digest, retrieving OS and architecture information from
// the image's configuration, and recording the image's reference so that it
// can be found at push-time.  Returns the instanceDigest for the image.  If
// the reference points to an image list, either all instances are added (if
// "all" is true), or the instance which matches "sys" (if "all" is false) will
// be added.
func (l *list) Add(ctx context.Context, sys *types.SystemContext, ref types.ImageReference, all bool) (digest.Digest, error) {
	src, err := ref.NewImageSource(ctx, sys)
	if err != nil {
		return "", fmt.Errorf("setting up to read manifest and configuration from %q: %w", transports.ImageName(ref), err)
	}
	defer src.Close()

	type instanceInfo struct {
		instanceDigest                       *digest.Digest
		OS, Architecture, OSVersion, Variant string
		Features, OSFeatures, Annotations    []string
		Size                                 int64
	}
	var instanceInfos []instanceInfo
	var manifestDigest digest.Digest

	primaryManifestBytes, primaryManifestType, err := src.GetManifest(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("reading manifest from %q: %w", transports.ImageName(ref), err)
	}

	if manifest.MIMETypeIsMultiImage(primaryManifestType) {
		lists, err := manifests.FromBlob(primaryManifestBytes)
		if err != nil {
			return "", fmt.Errorf("parsing manifest list in %q: %w", transports.ImageName(ref), err)
		}
		if all {
			for i, instance := range lists.OCIv1().Manifests {
				platform := instance.Platform
				if platform == nil {
					platform = &v1.Platform{}
				}
				instanceDigest := instance.Digest
				instanceInfo := instanceInfo{
					instanceDigest: &instanceDigest,
					OS:             platform.OS,
					Architecture:   platform.Architecture,
					OSVersion:      platform.OSVersion,
					Variant:        platform.Variant,
					Features:       append([]string{}, lists.Docker().Manifests[i].Platform.Features...),
					OSFeatures:     append([]string{}, platform.OSFeatures...),
					Size:           instance.Size,
				}
				instanceInfos = append(instanceInfos, instanceInfo)
			}
		} else {
			list, err := manifest.ListFromBlob(primaryManifestBytes, primaryManifestType)
			if err != nil {
				return "", fmt.Errorf("parsing manifest list in %q: %w", transports.ImageName(ref), err)
			}
			instanceDigest, err := list.ChooseInstance(sys)
			if err != nil {
				return "", fmt.Errorf("selecting image from manifest list in %q: %w", transports.ImageName(ref), err)
			}
			added := false
			for i, instance := range lists.OCIv1().Manifests {
				if instance.Digest != instanceDigest {
					continue
				}
				platform := instance.Platform
				if platform == nil {
					platform = &v1.Platform{}
				}
				instanceInfo := instanceInfo{
					instanceDigest: &instanceDigest,
					OS:             platform.OS,
					Architecture:   platform.Architecture,
					OSVersion:      platform.OSVersion,
					Variant:        platform.Variant,
					Features:       append([]string{}, lists.Docker().Manifests[i].Platform.Features...),
					OSFeatures:     append([]string{}, platform.OSFeatures...),
					Size:           instance.Size,
				}
				instanceInfos = append(instanceInfos, instanceInfo)
				added = true
			}
			if !added {
				instanceInfo := instanceInfo{
					instanceDigest: &instanceDigest,
				}
				instanceInfos = append(instanceInfos, instanceInfo)
			}
		}
	} else {
		instanceInfo := instanceInfo{
			instanceDigest: nil,
		}
		instanceInfos = append(instanceInfos, instanceInfo)
	}

	for _, instanceInfo := range instanceInfos {
		if instanceInfo.OS == "" || instanceInfo.Architecture == "" {
			img, err := image.FromUnparsedImage(ctx, sys, image.UnparsedInstance(src, instanceInfo.instanceDigest))
			if err != nil {
				return "", fmt.Errorf("reading configuration blob from %q: %w", transports.ImageName(ref), err)
			}
			config, err := img.OCIConfig(ctx)
			if err != nil {
				return "", fmt.Errorf("reading info about config blob from %q: %w", transports.ImageName(ref), err)
			}
			if instanceInfo.OS == "" {
				instanceInfo.OS = config.OS
				instanceInfo.OSVersion = config.OSVersion
				instanceInfo.OSFeatures = config.OSFeatures
			}
			if instanceInfo.Architecture == "" {
				instanceInfo.Architecture = config.Architecture
				instanceInfo.Variant = config.Variant
			}
		}
		manifestBytes, manifestType, err := src.GetManifest(ctx, instanceInfo.instanceDigest)
		if err != nil {
			return "", fmt.Errorf("reading manifest from %q, instance %q: %w", transports.ImageName(ref), instanceInfo.instanceDigest, err)
		}
		if instanceInfo.instanceDigest == nil {
			manifestDigest, err = manifest.Digest(manifestBytes)
			if err != nil {
				return "", fmt.Errorf("computing digest of manifest from %q: %w", transports.ImageName(ref), err)
			}
			instanceInfo.instanceDigest = &manifestDigest
			instanceInfo.Size = int64(len(manifestBytes))
		} else if manifestDigest == "" {
			manifestDigest = *instanceInfo.instanceDigest
		}
		err = l.List.AddInstance(*instanceInfo.instanceDigest, instanceInfo.Size, manifestType, instanceInfo.OS, instanceInfo.Architecture, instanceInfo.OSVersion, instanceInfo.OSFeatures, instanceInfo.Variant, instanceInfo.Features, instanceInfo.Annotations)
		if err != nil {
			return "", fmt.Errorf("adding instance with digest %q: %w", *instanceInfo.instanceDigest, err)
		}
		if _, ok := l.instances[*instanceInfo.instanceDigest]; !ok {
			l.instances[*instanceInfo.instanceDigest] = transports.ImageName(ref)
		}
	}

	return manifestDigest, nil
}

// Remove filters out any instances in the list which match the specified digest.
func (l *list) Remove(instanceDigest digest.Digest) error {
	err := l.List.Remove(instanceDigest)
	if err == nil {
		delete(l.instances, instanceDigest)
	}
	return err
}

// LockerForImage returns a Locker for a given image record.  It's recommended
// that processes which use LoadFromImage() to load a list from an image and
// then use that list's SaveToImage() method to save a modified version of the
// list to that image record use this lock to avoid accidentally wiping out
// changes that another process is also attempting to make.
func LockerForImage(store storage.Store, image string) (lockfile.Locker, error) { // nolint:staticcheck
	img, err := store.Image(image)
	if err != nil {
		return nil, fmt.Errorf("locating image %q for locating lock: %w", image, err)
	}
	d := digest.NewDigestFromEncoded(digest.Canonical, img.ID)
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("coercing image ID for %q into a digest: %w", image, err)
	}
	return store.GetDigestLock(d)
}
