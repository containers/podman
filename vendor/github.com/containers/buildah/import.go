package buildah

import (
	"context"

	"github.com/containers/buildah/docker"
	"github.com/containers/buildah/util"
	"github.com/containers/image/manifest"
	is "github.com/containers/image/storage"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/opencontainers/go-digest"
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
	src, err2 := ref.NewImage(ctx, systemContext)
	if err2 != nil {
		return nil, errors.Wrapf(err2, "error instantiating image")
	}
	defer src.Close()

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

	imageDigest := ""
	if manifestBytes, _, err := src.Manifest(ctx); err == nil {
		if manifestDigest, err := manifest.Digest(manifestBytes); err == nil {
			imageDigest = manifestDigest.String()
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
		IDMappingOptions: IDMappingOptions{
			HostUIDMapping: len(uidmap) == 0,
			HostGIDMapping: len(uidmap) == 0,
			UIDMap:         uidmap,
			GIDMap:         gidmap,
		},
	}

	if err := builder.initConfig(ctx, src); err != nil {
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
		return nil, errors.Wrapf(err, "error locating image %q for importing settings", options.Image)
	}

	builder, err := importBuilderDataFromImage(ctx, store, systemContext, img.ID, "", "")
	if err != nil {
		return nil, errors.Wrapf(err, "error importing build settings from image %q", options.Image)
	}

	return builder, nil
}
