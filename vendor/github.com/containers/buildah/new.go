package buildah

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/pkg/blobcache"
	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/shortnames"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	digest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/openshift/imagebuilder"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// BaseImageFakeName is the "name" of a source image which we interpret
	// as "no image".
	BaseImageFakeName = imagebuilder.NoBaseImageSpecifier
)

func getImageName(name string, img *storage.Image) string {
	imageName := name
	if len(img.Names) > 0 {
		imageName = img.Names[0]
		// When the image used by the container is a tagged image
		// the container name might be set to the original image instead of
		// the image given in the "from" command line.
		// This loop is supposed to fix this.
		for _, n := range img.Names {
			if strings.Contains(n, name) {
				imageName = n
				break
			}
		}
	}
	return imageName
}

func imageNamePrefix(imageName string) string {
	prefix := imageName
	s := strings.Split(prefix, ":")
	if len(s) > 0 {
		prefix = s[0]
	}
	s = strings.Split(prefix, "/")
	if len(s) > 0 {
		prefix = s[len(s)-1]
	}
	s = strings.Split(prefix, "@")
	if len(s) > 0 {
		prefix = s[0]
	}
	return prefix
}

func newContainerIDMappingOptions(idmapOptions *define.IDMappingOptions) storage.IDMappingOptions {
	var options storage.IDMappingOptions
	if idmapOptions != nil {
		options.HostUIDMapping = idmapOptions.HostUIDMapping
		options.HostGIDMapping = idmapOptions.HostGIDMapping
		uidmap, gidmap := convertRuntimeIDMaps(idmapOptions.UIDMap, idmapOptions.GIDMap)
		if len(uidmap) > 0 && len(gidmap) > 0 {
			options.UIDMap = uidmap
			options.GIDMap = gidmap
		} else {
			options.HostUIDMapping = true
			options.HostGIDMapping = true
		}
	}
	return options
}

func containerNameExist(name string, containers []storage.Container) bool {
	for _, container := range containers {
		for _, cname := range container.Names {
			if cname == name {
				return true
			}
		}
	}
	return false
}

func findUnusedContainer(name string, containers []storage.Container) string {
	suffix := 1
	tmpName := name
	for containerNameExist(tmpName, containers) {
		tmpName = fmt.Sprintf("%s-%d", name, suffix)
		suffix++
	}
	return tmpName
}

func newBuilder(ctx context.Context, store storage.Store, options BuilderOptions) (*Builder, error) {
	var (
		ref types.ImageReference
		img *storage.Image
		err error
	)

	if options.FromImage == BaseImageFakeName {
		options.FromImage = ""
	}

	systemContext := getSystemContext(store, options.SystemContext, options.SignaturePolicyPath)

	if options.FromImage != "" && options.FromImage != "scratch" {
		imageRuntime, err := libimage.RuntimeFromStore(store, &libimage.RuntimeOptions{SystemContext: systemContext})
		if err != nil {
			return nil, err
		}

		pullPolicy, err := config.ParsePullPolicy(options.PullPolicy.String())
		if err != nil {
			return nil, err
		}

		// Note: options.Format does *not* relate to the image we're
		// about to pull (see tests/digests.bats).  So we're not
		// forcing a MIMEType in the pullOptions below.
		pullOptions := libimage.PullOptions{}
		pullOptions.RetryDelay = &options.PullRetryDelay
		pullOptions.OciDecryptConfig = options.OciDecryptConfig
		pullOptions.SignaturePolicyPath = options.SignaturePolicyPath
		pullOptions.Writer = options.ReportWriter

		maxRetries := uint(options.MaxPullRetries)
		pullOptions.MaxRetries = &maxRetries

		if options.BlobDirectory != "" {
			pullOptions.DestinationLookupReferenceFunc = blobcache.CacheLookupReferenceFunc(options.BlobDirectory, types.PreserveOriginal)
		}

		pulledImages, err := imageRuntime.Pull(ctx, options.FromImage, pullPolicy, &pullOptions)
		if err != nil {
			return nil, err
		}
		if len(pulledImages) > 0 {
			img = pulledImages[0].StorageImage()
			ref, err = pulledImages[0].StorageReference()
			if err != nil {
				return nil, err
			}
		}
	}

	imageSpec := options.FromImage
	imageID := ""
	imageDigest := ""
	topLayer := ""
	if img != nil {
		imageSpec = getImageName(imageNamePrefix(imageSpec), img)
		imageID = img.ID
		topLayer = img.TopLayer
	}
	var src types.Image
	if ref != nil {
		srcSrc, err := ref.NewImageSource(ctx, systemContext)
		if err != nil {
			return nil, errors.Wrapf(err, "error instantiating image for %q", transports.ImageName(ref))
		}
		defer srcSrc.Close()
		manifestBytes, manifestType, err := srcSrc.GetManifest(ctx, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "error loading image manifest for %q", transports.ImageName(ref))
		}
		if manifestDigest, err := manifest.Digest(manifestBytes); err == nil {
			imageDigest = manifestDigest.String()
		}
		var instanceDigest *digest.Digest
		if manifest.MIMETypeIsMultiImage(manifestType) {
			list, err := manifest.ListFromBlob(manifestBytes, manifestType)
			if err != nil {
				return nil, errors.Wrapf(err, "error parsing image manifest for %q as list", transports.ImageName(ref))
			}
			instance, err := list.ChooseInstance(systemContext)
			if err != nil {
				return nil, errors.Wrapf(err, "error finding an appropriate image in manifest list %q", transports.ImageName(ref))
			}
			instanceDigest = &instance
		}
		src, err = image.FromUnparsedImage(ctx, systemContext, image.UnparsedInstance(srcSrc, instanceDigest))
		if err != nil {
			return nil, errors.Wrapf(err, "error instantiating image for %q instance %q", transports.ImageName(ref), instanceDigest)
		}
	}

	name := "working-container"
	if options.Container != "" {
		name = options.Container
	} else {
		if imageSpec != "" {
			name = imageNamePrefix(imageSpec) + "-" + name
		}
	}
	var container *storage.Container
	tmpName := name
	if options.Container == "" {
		containers, err := store.Containers()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to check for container names")
		}
		tmpName = findUnusedContainer(tmpName, containers)
	}

	conflict := 100
	for {
		coptions := storage.ContainerOptions{
			LabelOpts:        options.CommonBuildOpts.LabelOpts,
			IDMappingOptions: newContainerIDMappingOptions(options.IDMappingOptions),
			Volatile:         true,
		}
		container, err = store.CreateContainer("", []string{tmpName}, imageID, "", "", &coptions)
		if err == nil {
			name = tmpName
			break
		}
		if errors.Cause(err) != storage.ErrDuplicateName || options.Container != "" {
			return nil, errors.Wrapf(err, "error creating container")
		}
		tmpName = fmt.Sprintf("%s-%d", name, rand.Int()%conflict)
		conflict = conflict * 10
	}
	defer func() {
		if err != nil {
			if err2 := store.DeleteContainer(container.ID); err2 != nil {
				logrus.Errorf("error deleting container %q: %v", container.ID, err2)
			}
		}
	}()

	uidmap, gidmap := convertStorageIDMaps(container.UIDMap, container.GIDMap)

	defaultNamespaceOptions, err := DefaultNamespaceOptions()
	if err != nil {
		return nil, err
	}

	namespaceOptions := defaultNamespaceOptions
	namespaceOptions.AddOrReplace(options.NamespaceOptions...)

	// Set the base-image annotations as suggested by the OCI image spec.
	imageAnnotations := map[string]string{}
	imageAnnotations[v1.AnnotationBaseImageDigest] = imageDigest
	if !shortnames.IsShortName(imageSpec) {
		// If the base image could be resolved to a fully-qualified
		// image name, let's set it.
		imageAnnotations[v1.AnnotationBaseImageName] = imageSpec
	}

	builder := &Builder{
		store:                 store,
		Type:                  containerType,
		FromImage:             imageSpec,
		FromImageID:           imageID,
		FromImageDigest:       imageDigest,
		Container:             name,
		ContainerID:           container.ID,
		ImageAnnotations:      imageAnnotations,
		ImageCreatedBy:        "",
		ProcessLabel:          container.ProcessLabel(),
		MountLabel:            container.MountLabel(),
		DefaultMountsFilePath: options.DefaultMountsFilePath,
		Isolation:             options.Isolation,
		NamespaceOptions:      namespaceOptions,
		ConfigureNetwork:      options.ConfigureNetwork,
		CNIPluginPath:         options.CNIPluginPath,
		CNIConfigDir:          options.CNIConfigDir,
		IDMappingOptions: define.IDMappingOptions{
			HostUIDMapping: len(uidmap) == 0,
			HostGIDMapping: len(uidmap) == 0,
			UIDMap:         uidmap,
			GIDMap:         gidmap,
		},
		Capabilities:    copyStringSlice(options.Capabilities),
		CommonBuildOpts: options.CommonBuildOpts,
		TopLayer:        topLayer,
		Args:            options.Args,
		Format:          options.Format,
		TempVolumes:     map[string]bool{},
		Devices:         options.Devices,
	}

	if options.Mount {
		_, err = builder.Mount(container.MountLabel())
		if err != nil {
			return nil, errors.Wrapf(err, "error mounting build container %q", builder.ContainerID)
		}
	}

	if err := builder.initConfig(ctx, src, systemContext); err != nil {
		return nil, errors.Wrapf(err, "error preparing image configuration")
	}
	err = builder.Save()
	if err != nil {
		return nil, errors.Wrapf(err, "error saving builder state for container %q", builder.ContainerID)
	}

	return builder, nil
}
