package abi

import (
	"context"

	libpodImage "github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/pkg/errors"
)

func (ir *ImageEngine) List(ctx context.Context, opts entities.ImageListOptions) ([]*entities.ImageSummary, error) {
	images, err := ir.Libpod.ImageRuntime().GetImagesWithFilters(opts.Filter)
	if err != nil {
		return nil, err
	}

	if !opts.All {
		filter, err := ir.Libpod.ImageRuntime().IntermediateFilter(ctx, images)
		if err != nil {
			return nil, err
		}
		images = libpodImage.FilterImages(images, []libpodImage.ResultFilter{filter})
	}

	summaries := []*entities.ImageSummary{}
	for _, img := range images {
		digests := make([]string, len(img.Digests()))
		for j, d := range img.Digests() {
			digests[j] = string(d)
		}

		e := entities.ImageSummary{
			ID:           img.ID(),
			ConfigDigest: string(img.ConfigDigest),
			Created:      img.Created().Unix(),
			Dangling:     img.Dangling(),
			Digest:       string(img.Digest()),
			RepoDigests:  digests,
			History:      img.NamesHistory(),
			Names:        img.Names(),
			ReadOnly:     img.IsReadOnly(),
			SharedSize:   0,
			RepoTags:     img.Names(), // may include tags and digests
		}
		e.Labels, err = img.Labels(ctx)
		if err != nil {
			// Ignore empty manifest lists.
			if errors.Cause(err) != libpodImage.ErrImageIsBareList {
				return nil, errors.Wrapf(err, "error retrieving label for image %q: you may need to remove the image to resolve the error", img.ID())
			}
		}

		ctnrs, err := img.Containers()
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving containers for image %q: you may need to remove the image to resolve the error", img.ID())
		}
		e.Containers = len(ctnrs)

		sz, err := img.Size(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving size of image %q: you may need to remove the image to resolve the error", img.ID())
		}
		e.Size = int64(*sz)
		// This is good enough for now, but has to be
		// replaced later with correct calculation logic
		e.VirtualSize = int64(*sz)

		parent, err := img.ParentID(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving parent of image %q: you may need to remove the image to resolve the error", img.ID())
		}
		e.ParentId = parent

		summaries = append(summaries, &e)
	}
	return summaries, nil
}
