//go:build !remote

package libimage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/containers/common/libimage/define"
	"github.com/containers/common/libimage/manifests"
	imageCopy "github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	structcopier "github.com/jinzhu/copier"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// NOTE: the abstractions and APIs here are a first step to further merge
// `libimage/manifests` into `libimage`.

// ErrNotAManifestList indicates that an image was found in the local
// containers storage but it is not a manifest list as requested.
var ErrNotAManifestList = errors.New("image is not a manifest list")

// ManifestList represents a manifest list (Docker) or an image index (OCI) in
// the local containers storage.
type ManifestList struct {
	// NOTE: the *List* suffix is intentional as the term "manifest" is
	// used ambiguously across the ecosystem.  It may refer to the (JSON)
	// manifest of an ordinary image OR to a manifest *list* (Docker) or to
	// image index (OCI).
	// It's a bit more work when typing but without ambiguity.

	// The underlying image in the containers storage.
	image *Image

	// The underlying manifest list.
	list manifests.List
}

// ID returns the ID of the manifest list.
func (m *ManifestList) ID() string {
	return m.image.ID()
}

// CreateManifestList creates a new empty manifest list with the specified
// name.
func (r *Runtime) CreateManifestList(name string) (*ManifestList, error) {
	normalized, err := NormalizeName(name)
	if err != nil {
		return nil, err
	}

	list := manifests.Create()
	listID, err := list.SaveToImage(r.store, "", []string{normalized.String()}, manifest.DockerV2ListMediaType)
	if err != nil {
		return nil, err
	}

	mList, err := r.LookupManifestList(listID)
	if err != nil {
		return nil, err
	}

	return mList, nil
}

// LookupManifestList looks up a manifest list with the specified name in the
// containers storage.
func (r *Runtime) LookupManifestList(name string) (*ManifestList, error) {
	image, list, err := r.lookupManifestList(name)
	if err != nil {
		return nil, err
	}
	return &ManifestList{image: image, list: list}, nil
}

func (r *Runtime) lookupManifestList(name string) (*Image, manifests.List, error) {
	lookupOptions := &LookupImageOptions{
		lookupManifest: true,
	}
	image, _, err := r.LookupImage(name, lookupOptions)
	if err != nil {
		return nil, nil, err
	}
	if err := image.reload(); err != nil {
		return nil, nil, err
	}
	list, err := image.getManifestList()
	if err != nil {
		return nil, nil, err
	}
	return image, list, nil
}

// ToManifestList converts the image into a manifest list.  An error is thrown
// if the image is no manifest list.
func (i *Image) ToManifestList() (*ManifestList, error) {
	list, err := i.getManifestList()
	if err != nil {
		return nil, err
	}
	return &ManifestList{image: i, list: list}, nil
}

// LookupInstance looks up an instance of the manifest list matching the
// specified platform.  The local machine's platform is used if left empty.
func (m *ManifestList) LookupInstance(ctx context.Context, architecture, os, variant string) (*Image, error) {
	sys := m.image.runtime.systemContextCopy()
	if architecture != "" {
		sys.ArchitectureChoice = architecture
	}
	if os != "" {
		sys.OSChoice = os
	}
	if architecture != "" {
		sys.VariantChoice = variant
	}

	// Now look at the *manifest* and select a matching instance.
	rawManifest, manifestType, err := m.image.Manifest(ctx)
	if err != nil {
		return nil, err
	}
	list, err := manifest.ListFromBlob(rawManifest, manifestType)
	if err != nil {
		return nil, err
	}
	instanceDigest, err := list.ChooseInstance(sys)
	if err != nil {
		return nil, err
	}

	allImages, err := m.image.runtime.ListImages(ctx, nil, nil)
	if err != nil {
		return nil, err
	}

	for _, image := range allImages {
		for _, imageDigest := range append(image.Digests(), image.Digest()) {
			if imageDigest == instanceDigest {
				return image, nil
			}
		}
	}

	return nil, fmt.Errorf("could not find image instance %s of manifest list %s in local containers storage: %w", instanceDigest, m.ID(), storage.ErrImageUnknown)
}

// Saves the specified manifest list and reloads it from storage with the new ID.
func (m *ManifestList) saveAndReload() error {
	newID, err := m.list.SaveToImage(m.image.runtime.store, m.image.ID(), nil, "")
	if err != nil {
		return err
	}

	// Make sure to reload the image from the containers storage to fetch
	// the latest data (e.g., new or delete digests).
	if err := m.image.reload(); err != nil {
		return err
	}
	image, list, err := m.image.runtime.lookupManifestList(newID)
	if err != nil {
		return err
	}
	m.image = image
	m.list = list
	return nil
}

// Reload the image and list instances from storage
func (m *ManifestList) reload() error {
	listID := m.ID()
	if err := m.image.reload(); err != nil {
		return err
	}
	image, list, err := m.image.runtime.lookupManifestList(listID)
	if err != nil {
		return err
	}
	m.image = image
	m.list = list
	return nil
}

// getManifestList is a helper to obtain a manifest list
func (i *Image) getManifestList() (manifests.List, error) {
	_, list, err := manifests.LoadFromImage(i.runtime.store, i.ID())
	return list, err
}

// IsManifestList returns true if the image is a manifest list (Docker) or an
// image index (OCI).  This information may be critical to make certain
// execution paths more robust (e.g., suppress certain errors).
func (i *Image) IsManifestList(ctx context.Context) (bool, error) {
	// FIXME: due to `ImageDigestBigDataKey` we'll always check the
	// _last-written_ manifest which is causing issues for multi-arch image
	// pulls.
	//
	// See https://github.com/containers/common/pull/1505#discussion_r1242677279.
	ref, err := i.StorageReference()
	if err != nil {
		return false, err
	}
	imgRef, err := ref.NewImageSource(ctx, i.runtime.systemContextCopy())
	if err != nil {
		return false, err
	}
	_, manifestType, err := imgRef.GetManifest(ctx, nil)
	if err != nil {
		return false, err
	}
	return manifest.MIMETypeIsMultiImage(manifestType), nil
}

// Inspect returns a dockerized version of the manifest list.
func (m *ManifestList) Inspect() (*define.ManifestListData, error) {
	inspectList := define.ManifestListData{}
	// Copy the fields from the Docker-format version of the list.
	dockerFormat := m.list.Docker()
	err := structcopier.Copy(&inspectList, &dockerFormat)
	if err != nil {
		return &inspectList, err
	}
	// Get OCI-specific fields from the OCIv1-format version of the list
	// and copy them to the inspect data.
	ociFormat := m.list.OCIv1()
	inspectList.ArtifactType = ociFormat.ArtifactType
	inspectList.Annotations = ociFormat.Annotations
	for i, manifest := range ociFormat.Manifests {
		inspectList.Manifests[i].Annotations = manifest.Annotations
		inspectList.Manifests[i].ArtifactType = manifest.ArtifactType
		if manifest.URLs != nil {
			inspectList.Manifests[i].URLs = slices.Clone(manifest.URLs)
		}
		inspectList.Manifests[i].Data = manifest.Data
		inspectList.Manifests[i].Files, err = m.list.Files(manifest.Digest)
		if err != nil {
			return &inspectList, err
		}
	}
	if ociFormat.Subject != nil {
		platform := ociFormat.Subject.Platform
		if platform == nil {
			platform = &imgspecv1.Platform{}
		}
		var osFeatures []string
		if platform.OSFeatures != nil {
			osFeatures = slices.Clone(platform.OSFeatures)
		}
		inspectList.Subject = &define.ManifestListDescriptor{
			Platform: manifest.Schema2PlatformSpec{
				OS:           platform.OS,
				Architecture: platform.Architecture,
				OSVersion:    platform.OSVersion,
				Variant:      platform.Variant,
				OSFeatures:   osFeatures,
			},
			Schema2Descriptor: manifest.Schema2Descriptor{
				MediaType: ociFormat.Subject.MediaType,
				Digest:    ociFormat.Subject.Digest,
				Size:      ociFormat.Subject.Size,
				URLs:      ociFormat.Subject.URLs,
			},
			Annotations:  ociFormat.Subject.Annotations,
			ArtifactType: ociFormat.Subject.ArtifactType,
			Data:         ociFormat.Subject.Data,
		}
	}
	// Set MediaType to mirror the value we'd use when saving the list
	// using defaults, instead of forcing it to one or the other by
	// using the value from one version or the other that we explicitly
	// requested above.
	serialized, err := m.list.Serialize("")
	if err != nil {
		return &inspectList, err
	}
	var typed struct {
		MediaType string `json:"mediaType,omitempty"`
	}
	if err := json.Unmarshal(serialized, &typed); err != nil {
		return &inspectList, err
	}
	if typed.MediaType != "" {
		inspectList.MediaType = typed.MediaType
	}
	return &inspectList, nil
}

// Options for adding an image or artifact to a manifest list.
type ManifestListAddOptions struct {
	// Add all images to the list if the to-be-added image itself is a
	// manifest list.
	All bool `json:"all"`
	// containers-auth.json(5) file to use when authenticating against
	// container registries.
	AuthFilePath string
	// Path to the certificates directory.
	CertDirPath string
	// Allow contacting registries over HTTP, or HTTPS with failed TLS
	// verification. Note that this does not affect other TLS connections.
	InsecureSkipTLSVerify types.OptionalBool
	// Username to use when authenticating at a container registry.
	Username string
	// Password to use when authenticating at a container registry.
	Password string
}

// Add adds one or more manifests to the manifest list and returns the digest
// of the added instance.
func (m *ManifestList) Add(ctx context.Context, name string, options *ManifestListAddOptions) (digest.Digest, error) {
	if options == nil {
		options = &ManifestListAddOptions{}
	}

	ref, err := alltransports.ParseImageName(name)
	if err != nil {
		withDocker := fmt.Sprintf("%s://%s", docker.Transport.Name(), name)
		ref, err = alltransports.ParseImageName(withDocker)
		if err != nil {
			return "", err
		}
	}

	// Now massage in the copy-related options into the system context.
	systemContext := m.image.runtime.systemContextCopy()
	if options.AuthFilePath != "" {
		systemContext.AuthFilePath = options.AuthFilePath
	}
	if options.CertDirPath != "" {
		systemContext.DockerCertPath = options.CertDirPath
	}
	if options.InsecureSkipTLSVerify != types.OptionalBoolUndefined {
		systemContext.DockerInsecureSkipTLSVerify = options.InsecureSkipTLSVerify
		systemContext.OCIInsecureSkipTLSVerify = options.InsecureSkipTLSVerify == types.OptionalBoolTrue
		systemContext.DockerDaemonInsecureSkipTLSVerify = options.InsecureSkipTLSVerify == types.OptionalBoolTrue
	}
	if options.Username != "" {
		systemContext.DockerAuthConfig = &types.DockerAuthConfig{
			Username: options.Username,
			Password: options.Password,
		}
	}
	locker, err := manifests.LockerForImage(m.image.runtime.store, m.ID())
	if err != nil {
		return "", err
	}
	locker.Lock()
	defer locker.Unlock()
	// Make sure to reload the image from the containers storage to fetch
	// the latest data (e.g., new or delete digests).
	if err := m.reload(); err != nil {
		return "", err
	}
	newDigest, err := m.list.Add(ctx, systemContext, ref, options.All)
	if err != nil {
		return "", err
	}

	// Write the changes to disk.
	if err := m.saveAndReload(); err != nil {
		return "", err
	}
	return newDigest, nil
}

// Options for creating an artifact manifest for one or more files and adding
// the artifact manifest to a manifest list.
type ManifestListAddArtifactOptions struct {
	// The artifactType to set in the artifact manifest.
	Type *string `json:"artifact_type"`
	// The mediaType to set in the config.MediaType field in the artifact manifest.
	ConfigType string `json:"artifact_config_type"`
	// Content to point to from the config field in the artifact manifest.
	Config string `json:"artifact_config"`
	// The mediaType to set in the layer descriptors in the artifact manifest.
	LayerType string `json:"artifact_layer_type"`
	// Whether or not to suppress the org.opencontainers.image.title annotation in layer descriptors.
	ExcludeTitles bool `json:"exclude_layer_titles"`
	// Annotations to set in the artifact manifest.
	Annotations map[string]string `json:"annotations"`
	// Subject to set in the artifact manifest.
	Subject string `json:"subject"`
}

// Add adds one or more manifests to the manifest list and returns the digest
// of the added instance.
func (m *ManifestList) AddArtifact(ctx context.Context, options *ManifestListAddArtifactOptions, files ...string) (digest.Digest, error) {
	if options == nil {
		options = &ManifestListAddArtifactOptions{}
	}
	opts := manifests.AddArtifactOptions{
		ManifestArtifactType: options.Type,
		Annotations:          maps.Clone(options.Annotations),
		ExcludeTitles:        options.ExcludeTitles,
	}
	if options.ConfigType != "" {
		opts.ConfigDescriptor = &imgspecv1.Descriptor{
			MediaType: options.ConfigType,
			Digest:    imgspecv1.DescriptorEmptyJSON.Digest,
			Size:      imgspecv1.DescriptorEmptyJSON.Size,
			Data:      slices.Clone(imgspecv1.DescriptorEmptyJSON.Data),
		}
	}
	if options.Config != "" {
		if opts.ConfigDescriptor == nil {
			opts.ConfigDescriptor = &imgspecv1.Descriptor{
				MediaType: imgspecv1.MediaTypeImageConfig,
			}
		}
		opts.ConfigDescriptor.Digest = digest.FromString(options.Config)
		opts.ConfigDescriptor.Size = int64(len(options.Config))
		opts.ConfigDescriptor.Data = slices.Clone([]byte(options.Config))
	}
	if opts.ConfigDescriptor == nil {
		empty := imgspecv1.DescriptorEmptyJSON
		opts.ConfigDescriptor = &empty
	}
	if options.LayerType != "" {
		opts.LayerMediaType = &options.LayerType
	}
	if options.Subject != "" {
		ref, err := alltransports.ParseImageName(options.Subject)
		if err != nil {
			withDocker := fmt.Sprintf("%s://%s", docker.Transport.Name(), options.Subject)
			ref, err = alltransports.ParseImageName(withDocker)
			if err != nil {
				image, _, err := m.image.runtime.LookupImage(options.Subject, &LookupImageOptions{ManifestList: true})
				if err != nil {
					return "", fmt.Errorf("locating subject for artifact manifest: %w", err)
				}
				ref = image.storageReference
			}
		}
		opts.SubjectReference = ref
	}

	// Lock the image record where this list lives.
	locker, err := manifests.LockerForImage(m.image.runtime.store, m.ID())
	if err != nil {
		return "", err
	}
	locker.Lock()
	defer locker.Unlock()

	systemContext := m.image.runtime.systemContextCopy()

	// Make sure to reload the image from the containers storage to fetch
	// the latest data (e.g., new or delete digests).
	if err := m.reload(); err != nil {
		return "", err
	}
	newDigest, err := m.list.AddArtifact(ctx, systemContext, opts, files...)
	if err != nil {
		return "", err
	}

	// Write the changes to disk.
	if err := m.saveAndReload(); err != nil {
		return "", err
	}
	return newDigest, nil
}

// Options for annotating a manifest list.
type ManifestListAnnotateOptions struct {
	// Add the specified annotations to the added image.
	Annotations map[string]string
	// Add the specified architecture to the added image.
	Architecture string
	// Add the specified features to the added image.
	Features []string
	// Add the specified OS to the added image.
	OS string
	// Add the specified OS features to the added image.
	OSFeatures []string
	// Add the specified OS version to the added image.
	OSVersion string
	// Add the specified variant to the added image.
	Variant string
	// Add the specified annotations to the index itself.
	IndexAnnotations map[string]string
	// Set the subject to which the index refers.
	Subject string
}

// Annotate an image instance specified by `d` in the manifest list.
func (m *ManifestList) AnnotateInstance(d digest.Digest, options *ManifestListAnnotateOptions) error {
	ctx := context.Background()

	if options == nil {
		return nil
	}

	if len(options.OS) > 0 {
		if err := m.list.SetOS(d, options.OS); err != nil {
			return err
		}
	}
	if len(options.OSVersion) > 0 {
		if err := m.list.SetOSVersion(d, options.OSVersion); err != nil {
			return err
		}
	}
	if len(options.Features) > 0 {
		if err := m.list.SetFeatures(d, options.Features); err != nil {
			return err
		}
	}
	if len(options.OSFeatures) > 0 {
		if err := m.list.SetOSFeatures(d, options.OSFeatures); err != nil {
			return err
		}
	}
	if len(options.Architecture) > 0 {
		if err := m.list.SetArchitecture(d, options.Architecture); err != nil {
			return err
		}
	}
	if len(options.Variant) > 0 {
		if err := m.list.SetVariant(d, options.Variant); err != nil {
			return err
		}
	}
	if len(options.Annotations) > 0 {
		if err := m.list.SetAnnotations(&d, options.Annotations); err != nil {
			return err
		}
	}
	if len(options.IndexAnnotations) > 0 {
		if err := m.list.SetAnnotations(nil, options.IndexAnnotations); err != nil {
			return err
		}
	}
	if options.Subject != "" {
		ref, err := alltransports.ParseImageName(options.Subject)
		if err != nil {
			withDocker := fmt.Sprintf("%s://%s", docker.Transport.Name(), options.Subject)
			ref, err = alltransports.ParseImageName(withDocker)
			if err != nil {
				image, _, err := m.image.runtime.LookupImage(options.Subject, &LookupImageOptions{ManifestList: true})
				if err != nil {
					return fmt.Errorf("locating subject for image index: %w", err)
				}
				ref = image.storageReference
			}
		}
		src, err := ref.NewImageSource(ctx, &m.image.runtime.systemContext)
		if err != nil {
			return err
		}
		defer src.Close()
		subjectManifestBytes, subjectManifestType, err := src.GetManifest(ctx, nil)
		if err != nil {
			return err
		}
		subjectManifestDigest, err := manifest.Digest(subjectManifestBytes)
		if err != nil {
			return err
		}
		var subjectArtifactType string
		if !manifest.MIMETypeIsMultiImage(subjectManifestType) {
			var subjectManifest imgspecv1.Manifest
			if json.Unmarshal(subjectManifestBytes, &subjectManifest) == nil {
				subjectArtifactType = subjectManifest.ArtifactType
			}
		}
		descriptor := &imgspecv1.Descriptor{
			MediaType:    subjectManifestType,
			ArtifactType: subjectArtifactType,
			Digest:       subjectManifestDigest,
			Size:         int64(len(subjectManifestBytes)),
		}
		if err := m.list.SetSubject(descriptor); err != nil {
			return err
		}
	}

	// Write the changes to disk.
	return m.saveAndReload()
}

// RemoveInstance removes the instance specified by `d` from the manifest list.
// Returns the new ID of the image.
func (m *ManifestList) RemoveInstance(d digest.Digest) error {
	if err := m.list.Remove(d); err != nil {
		return err
	}

	// Write the changes to disk.
	return m.saveAndReload()
}

// ManifestListPushOptions allow for customizing pushing a manifest list.
type ManifestListPushOptions struct {
	CopyOptions

	// For tweaking the list selection.
	ImageListSelection imageCopy.ImageListSelection
	// Use when selecting only specific imags.
	Instances []digest.Digest
	// Add existing instances with requested compression algorithms to manifest list
	AddCompression []string
}

// Push pushes a manifest to the specified destination.
func (m *ManifestList) Push(ctx context.Context, destination string, options *ManifestListPushOptions) (digest.Digest, error) {
	if options == nil {
		options = &ManifestListPushOptions{}
	}

	dest, err := alltransports.ParseImageName(destination)
	if err != nil {
		oldErr := err
		dest, err = alltransports.ParseImageName("docker://" + destination)
		if err != nil {
			return "", oldErr
		}
	}

	if m.image.runtime.eventChannel != nil {
		defer m.image.runtime.writeEvent(&Event{ID: m.ID(), Name: destination, Time: time.Now(), Type: EventTypeImagePush})
	}

	// NOTE: we're using the logic in copier to create a proper
	// types.SystemContext. This prevents us from having an error prone
	// code duplicate here.
	copier, err := m.image.runtime.newCopier(&options.CopyOptions)
	if err != nil {
		return "", err
	}
	defer copier.close()

	pushOptions := manifests.PushOptions{
		AddCompression:                   options.AddCompression,
		Store:                            m.image.runtime.store,
		SystemContext:                    copier.systemContext,
		ImageListSelection:               options.ImageListSelection,
		Instances:                        options.Instances,
		ReportWriter:                     options.Writer,
		Signers:                          options.Signers,
		SignBy:                           options.SignBy,
		SignPassphrase:                   options.SignPassphrase,
		SignBySigstorePrivateKeyFile:     options.SignBySigstorePrivateKeyFile,
		SignSigstorePrivateKeyPassphrase: options.SignSigstorePrivateKeyPassphrase,
		RemoveSignatures:                 options.RemoveSignatures,
		ManifestType:                     options.ManifestMIMEType,
		MaxRetries:                       options.MaxRetries,
		RetryDelay:                       options.RetryDelay,
		ForceCompressionFormat:           options.ForceCompressionFormat,
	}

	_, d, err := m.list.Push(ctx, dest, pushOptions)
	return d, err
}
