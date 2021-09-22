package buildah

import (
	"context"

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/docker"
	"github.com/containers/buildah/util"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

func importBuilderDataFromImage(ctx context.Context, store storage.Store, systemContext *types.SystemContext, imageID, containerName, containerID string) (*Builder, error) {
	if imageID == "" {
		return nil, errors.Errorf("Internal error: imageID is empty in importBuilderDataFromImage")
	}

	storeopts, err := storage.DefaultStoreOptions(false, 0)
	if err != nil {
		return nil, err
	}
	uidmap, gidmap := convertStorageIDMaps(storeopts.UIDMap, storeopts.GIDMap)

	ref, err := is.Transport.ParseStoreReference(store, imageID)
	if err != nil {
		return nil, errors.Wrapf(err, "no such image %q", imageID)
	}
	src, err := ref.NewImageSource(ctx, systemContext)
	if err != nil {
		return nil, errors.Wrapf(err, "error instantiating image source")
	}
	defer src.Close()

	imageDigest := ""
	manifestBytes, manifestType, err := src.GetManifest(ctx, nil)
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

	image, err := image.FromUnparsedImage(ctx, systemContext, image.UnparsedInstance(src, instanceDigest))
	if err != nil {
		return nil, errors.Wrapf(err, "error instantiating image for %q instance %q", transports.ImageName(ref), instanceDigest)
	}

	imageName := ""
	if img, err3 := store.Image(imageID); err3 == nil {
		if len(img.Names) > 0 {
			imageName = img.Names[0]
		}
		if img.TopLayer != "" {
			layer, err4 := store.Layer(img.TopLayer)
			if err4 != nil {
				return nil, errors.Wrapf(err4, "error reading information about image's top layer")
			}
			uidmap, gidmap = convertStorageIDMaps(layer.UIDMap, layer.GIDMap)
		}
	}

	defaultNamespaceOptions, err := DefaultNamespaceOptions()
	if err != nil {
		return nil, err
	}

	builder := &Builder{
		store:            store,
		Type:             containerType,
		FromImage:        imageName,
		FromImageID:      imageID,
		FromImageDigest:  imageDigest,
		Container:        containerName,
		ContainerID:      containerID,
		ImageAnnotations: map[string]string{},
		ImageCreatedBy:   "",
		NamespaceOptions: defaultNamespaceOptions,
		IDMappingOptions: define.IDMappingOptions{
			HostUIDMapping: len(uidmap) == 0,
			HostGIDMapping: len(uidmap) == 0,
			UIDMap:         uidmap,
			GIDMap:         gidmap,
		},
	}

	if err := builder.initConfig(ctx, image, systemContext); err != nil {
		return nil, errors.Wrapf(err, "error preparing image configuration")
	}

	return builder, nil
}

func importBuilder(ctx context.Context, store storage.Store, options ImportOptions) (*Builder, error) {
	if options.Container == "" {
		return nil, errors.Errorf("container name must be specified")
	}

	c, err := store.Container(options.Container)
	if err != nil {
		return nil, err
	}

	systemContext := getSystemContext(store, &types.SystemContext{}, options.SignaturePolicyPath)

	builder, err := importBuilderDataFromImage(ctx, store, systemContext, c.ImageID, options.Container, c.ID)
	if err != nil {
		return nil, err
	}

	if builder.FromImageID != "" {
		if d, err2 := digest.Parse(builder.FromImageID); err2 == nil {
			builder.Docker.Parent = docker.ID(d)
		} else {
			builder.Docker.Parent = docker.ID(digest.NewDigestFromHex(digest.Canonical.String(), builder.FromImageID))
		}
	}
	if builder.FromImage != "" {
		builder.Docker.ContainerConfig.Image = builder.FromImage
	}
	builder.IDMappingOptions.UIDMap, builder.IDMappingOptions.GIDMap = convertStorageIDMaps(c.UIDMap, c.GIDMap)

	err = builder.Save()
	if err != nil {
		return nil, errors.Wrapf(err, "error saving builder state")
	}

	return builder, nil
}

func importBuilderFromImage(ctx context.Context, store storage.Store, options ImportFromImageOptions) (*Builder, error) {
	if options.Image == "" {
		return nil, errors.Errorf("image name must be specified")
	}

	systemContext := getSystemContext(store, options.SystemContext, options.SignaturePolicyPath)

	_, img, err := util.FindImage(store, "", systemContext, options.Image)
	if err != nil {
		return nil, errors.Wrapf(err, "importing settings")
	}

	builder, err := importBuilderDataFromImage(ctx, store, systemContext, img.ID, "", "")
	if err != nil {
		return nil, errors.Wrapf(err, "error importing build settings from image %q", options.Image)
	}

	builder.setupLogger()
	return builder, nil
}
