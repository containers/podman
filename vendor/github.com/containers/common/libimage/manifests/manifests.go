package manifests

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"io"

	"github.com/containers/common/pkg/manifests"
	"github.com/containers/common/pkg/supplemented"
	cp "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const instancesData = "instances.json"

// ErrListImageUnknown is returned when we attempt to create an image reference
// for a List that has not yet been saved to an image.
var ErrListImageUnknown = stderrors.New("unable to determine which image holds the manifest list")

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
	Store              storage.Store
	SystemContext      *types.SystemContext  // github.com/containers/image/types.SystemContext
	ImageListSelection cp.ImageListSelection // set to either CopySystemImage, CopyAllImages, or CopySpecificImages
	Instances          []digest.Digest       // instances to copy if ImageListSelection == CopySpecificImages
	ReportWriter       io.Writer             // will be used to log the writing of the list and any blobs
	SignBy             string                // fingerprint of GPG key to use to sign images
	RemoveSignatures   bool                  // true to discard signatures in images
	ManifestType       string                // the format to use when saving the list - possible options are oci, v2s1, and v2s2
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
		return "", nil, errors.Wrapf(err, "error locating image %q for loading manifest list", image)
	}
	manifestBytes, err := store.ImageBigData(img.ID, storage.ImageDigestManifestBigDataNamePrefix)
	if err != nil {
		return "", nil, errors.Wrapf(err, "error locating image %q for loading manifest list", image)
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
		return "", nil, errors.Wrapf(err, "error locating image %q for loading instance list", image)
	}
	if err := json.Unmarshal(instancesBytes, &list.instances); err != nil {
		return "", nil, errors.Wrapf(err, "error decoding instance list for image %q", image)
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
	if err == nil || errors.Cause(err) == storage.ErrDuplicateID {
		created := (err == nil)
		if created {
			imageID = img.ID
			l.instances[""] = img.ID
		}
		err := store.SetImageBigData(imageID, storage.ImageDigestManifestBigDataNamePrefix, manifestBytes, manifest.Digest)
		if err != nil {
			if created {
				if _, err2 := store.DeleteImage(img.ID, true); err2 != nil {
					logrus.Errorf("error deleting image %q after failing to save manifest for it", img.ID)
				}
			}
			return "", errors.Wrapf(err, "error saving manifest list to image %q", imageID)
		}
		err = store.SetImageBigData(imageID, instancesData, instancesBytes, nil)
		if err != nil {
			if created {
				if _, err2 := store.DeleteImage(img.ID, true); err2 != nil {
					logrus.Errorf("error deleting image %q after failing to save instance locations for it", img.ID)
				}
			}
			return "", errors.Wrapf(err, "error saving instance list to image %q", imageID)
		}
		return imageID, nil
	}
	return "", errors.Wrapf(err, "error creating image to hold manifest list")
}

// Reference returns an image reference for the composite image being built
// in the list, or an error if the list has never been saved to a local image.
func (l *list) Reference(store storage.Store, multiple cp.ImageListSelection, instances []digest.Digest) (types.ImageReference, error) {
	if l.instances[""] == "" {
		return nil, errors.Wrap(ErrListImageUnknown, "error building reference to list")
	}
	s, err := is.Transport.ParseStoreReference(store, l.instances[""])
	if err != nil {
		return nil, errors.Wrapf(err, "error creating ImageReference from image %q", l.instances[""])
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
			return nil, errors.Wrapf(err, "error creating ImageReference from image %q", imageName)
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
		return nil, "", errors.Wrapf(err, "error obtaining default signature policy")
	}

	// Override the settings for local storage to make sure that we can always read the source "image".
	pushPolicy.Transports[is.Transport.Name()] = storageAllowedPolicyScopes

	policyContext, err := signature.NewPolicyContext(pushPolicy)
	if err != nil {
		return nil, "", errors.Wrapf(err, "error creating new signature policy context")
	}
	defer func() {
		if err2 := policyContext.Destroy(); err2 != nil {
			logrus.Errorf("error destroying signature policy context: %v", err2)
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
	copyOptions := &cp.Options{
		ImageListSelection:    options.ImageListSelection,
		Instances:             options.Instances,
		SourceCtx:             options.SystemContext,
		DestinationCtx:        options.SystemContext,
		ReportWriter:          options.ReportWriter,
		RemoveSignatures:      options.RemoveSignatures,
		SignBy:                options.SignBy,
		ForceManifestMIMEType: singleImageManifestType,
	}

	// Copy whatever we were asked to copy.
	manifestBytes, err := cp.Image(ctx, policyContext, dest, src, copyOptions)
	if err != nil {
		return nil, "", err
	}
	manifestDigest, err := manifest.Digest(manifestBytes)
	if err != nil {
		return nil, "", err
	}
	return nil, manifestDigest, nil
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
		return "", errors.Wrapf(err, "error setting up to read manifest and configuration from %q", transports.ImageName(ref))
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
		return "", errors.Wrapf(err, "error reading manifest from %q", transports.ImageName(ref))
	}

	if manifest.MIMETypeIsMultiImage(primaryManifestType) {
		lists, err := manifests.FromBlob(primaryManifestBytes)
		if err != nil {
			return "", errors.Wrapf(err, "error parsing manifest list in %q", transports.ImageName(ref))
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
				return "", errors.Wrapf(err, "error parsing manifest list in %q", transports.ImageName(ref))
			}
			instanceDigest, err := list.ChooseInstance(sys)
			if err != nil {
				return "", errors.Wrapf(err, "error selecting image from manifest list in %q", transports.ImageName(ref))
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
				return "", errors.Wrapf(err, "error reading configuration blob from %q", transports.ImageName(ref))
			}
			config, err := img.OCIConfig(ctx)
			if err != nil {
				return "", errors.Wrapf(err, "error reading info about config blob from %q", transports.ImageName(ref))
			}
			if instanceInfo.OS == "" {
				instanceInfo.OS = config.OS
			}
			if instanceInfo.Architecture == "" {
				instanceInfo.Architecture = config.Architecture
			}
		}
		manifestBytes, manifestType, err := src.GetManifest(ctx, instanceInfo.instanceDigest)
		if err != nil {
			return "", errors.Wrapf(err, "error reading manifest from %q, instance %q", transports.ImageName(ref), instanceInfo.instanceDigest)
		}
		if instanceInfo.instanceDigest == nil {
			manifestDigest, err = manifest.Digest(manifestBytes)
			if err != nil {
				return "", errors.Wrapf(err, "error computing digest of manifest from %q", transports.ImageName(ref))
			}
			instanceInfo.instanceDigest = &manifestDigest
			instanceInfo.Size = int64(len(manifestBytes))
		} else {
			if manifestDigest == "" {
				manifestDigest = *instanceInfo.instanceDigest
			}
		}
		err = l.List.AddInstance(*instanceInfo.instanceDigest, instanceInfo.Size, manifestType, instanceInfo.OS, instanceInfo.Architecture, instanceInfo.OSVersion, instanceInfo.OSFeatures, instanceInfo.Variant, instanceInfo.Features, instanceInfo.Annotations)
		if err != nil {
			return "", errors.Wrapf(err, "error adding instance with digest %q", *instanceInfo.instanceDigest)
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
		if _, needToDelete := l.instances[instanceDigest]; needToDelete {
			delete(l.instances, instanceDigest)
		}
	}
	return err
}
