package abi

import (
	"context"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/pkg/errors"
)

func (ir *ImageEngine) List(ctx context.Context, opts entities.ImageListOptions) ([]*entities.ImageSummary, error) {
	listImagesOptions := &libimage.ListImagesOptions{
		Filters: opts.Filter,
	}
	if !opts.All {
		// Filter intermediate images unless we want to list *all*.
		// NOTE: it's a positive filter, so `intermediate=false` means
		// to display non-intermediate images.
		listImagesOptions.Filters = append(listImagesOptions.Filters, "intermediate=false")
	}

	images, err := ir.Libpod.LibimageRuntime().ListImages(ctx, nil, listImagesOptions)
	if err != nil {
		return nil, err
	}

	summaries := []*entities.ImageSummary{}
	for _, img := range images {
		repoDigests, err := img.RepoDigests()
		if err != nil {
			return nil, errors.Wrapf(err, "getting repoDigests from image %q", img.ID())
		}
		isDangling, err := img.IsDangling(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "error checking if image %q is dangling", img.ID())
		}

		e := entities.ImageSummary{
			ID: img.ID(),
			// TODO: libpod/image didn't set it but libimage should
			// ConfigDigest: string(img.ConfigDigest),
			Created:     img.Created().Unix(),
			Dangling:    isDangling,
			Digest:      string(img.Digest()),
			RepoDigests: repoDigests,
			History:     img.NamesHistory(),
			Names:       img.Names(),
			ReadOnly:    img.IsReadOnly(),
			SharedSize:  0,
			RepoTags:    img.Names(), // may include tags and digests
		}
		e.Labels, err = img.Labels(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving label for image %q: you may need to remove the image to resolve the error", img.ID())
		}

		ctnrs, err := img.Containers()
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving containers for image %q: you may need to remove the image to resolve the error", img.ID())
		}
		e.Containers = len(ctnrs)

		sz, err := img.Size()
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving size of image %q: you may need to remove the image to resolve the error", img.ID())
		}
		e.Size = sz
		// This is good enough for now, but has to be
		// replaced later with correct calculation logic
		e.VirtualSize = sz

		parent, err := img.Parent(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving parent of image %q: you may need to remove the image to resolve the error", img.ID())
		}
		if parent != nil {
			e.ParentId = parent.ID()
		}

		summaries = append(summaries, &e)
	}
	return summaries, nil
}
