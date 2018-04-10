package buildah

import (
	is "github.com/containers/image/storage"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/podman/docker"
	"github.com/projectatomic/libpod/libpod/buildah/util"
)

func importBuilderDataFromImage(store storage.Store, systemContext *types.SystemContext, imageID, containerName, containerID string) (*Builder, error) {
	manifest := []byte{}
	config := []byte{}
	imageName := ""

	if imageID != "" {
		ref, err := is.Transport.ParseStoreReference(store, imageID)
		if err != nil {
			return nil, errors.Wrapf(err, "no such image %q", imageID)
		}
		src, err2 := ref.NewImage(systemContext)
		if err2 != nil {
			return nil, errors.Wrapf(err2, "error instantiating image")
		}
		defer src.Close()
		config, err = src.ConfigBlob()
		if err != nil {
			return nil, errors.Wrapf(err, "error reading image configuration")
		}
		manifest, _, err = src.Manifest()
		if err != nil {
			return nil, errors.Wrapf(err, "error reading image manifest")
		}
		if img, err3 := store.Image(imageID); err3 == nil {
			if len(img.Names) > 0 {
				imageName = img.Names[0]
			}
		}
	}

	builder := &Builder{
		store:            store,
		Type:             containerType,
		FromImage:        imageName,
		FromImageID:      imageID,
		Config:           config,
		Manifest:         manifest,
		Container:        containerName,
		ContainerID:      containerID,
		ImageAnnotations: map[string]string{},
		ImageCreatedBy:   "",
	}

	builder.initConfig()

	return builder, nil
}

func importBuilder(store storage.Store, options ImportOptions) (*Builder, error) {
	if options.Container == "" {
		return nil, errors.Errorf("container name must be specified")
	}

	c, err := store.Container(options.Container)
	if err != nil {
		return nil, err
	}

	systemContext := getSystemContext(&types.SystemContext{}, options.SignaturePolicyPath)

	builder, err := importBuilderDataFromImage(store, systemContext, c.ImageID, options.Container, c.ID)
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

	err = builder.Save()
	if err != nil {
		return nil, errors.Wrapf(err, "error saving builder state")
	}

	return builder, nil
}

func importBuilderFromImage(store storage.Store, options ImportFromImageOptions) (*Builder, error) {
	var img *storage.Image
	var err error

	if options.Image == "" {
		return nil, errors.Errorf("image name must be specified")
	}

	systemContext := getSystemContext(options.SystemContext, options.SignaturePolicyPath)

	for _, image := range util.ResolveName(options.Image, "", systemContext, store) {
		img, err = util.FindImage(store, image)
		if err != nil {
			continue
		}

		builder, err2 := importBuilderDataFromImage(store, systemContext, img.ID, "", "")
		if err2 != nil {
			return nil, errors.Wrapf(err2, "error importing build settings from image %q", options.Image)
		}

		return builder, nil
	}
	return nil, errors.Wrapf(err, "error locating image %q for importing settings", options.Image)
}
