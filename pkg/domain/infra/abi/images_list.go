package abi

import (
	"context"
	"fmt"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

func (ir *ImageEngine) List(ctx context.Context, opts entities.ImageListOptions) ([]*entities.ImageSummary, error) {
	listImagesOptions := &libimage.ListImagesOptions{
		Filters:     opts.Filter,
		SetListData: true,
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
			return nil, fmt.Errorf("getting repoDigests from image %q: %w", img.ID(), err)
		}

		if img.ListData.IsDangling == nil { // Sanity check
			return nil, fmt.Errorf("%w: ListData.IsDangling is nil but should not", define.ErrInternal)
		}
		isDangling := *img.ListData.IsDangling
		parentID := ""
		if img.ListData.Parent != nil {
			parentID = img.ListData.Parent.ID()
		}

		e := entities.ImageSummary{
			ID:          img.ID(),
			Created:     img.Created().Unix(),
			Dangling:    isDangling,
			Digest:      string(img.Digest()),
			RepoDigests: repoDigests,
			History:     img.NamesHistory(),
			Names:       img.Names(),
			ReadOnly:    img.IsReadOnly(),
			SharedSize:  0,
			RepoTags:    img.Names(), // may include tags and digests
			ParentId:    parentID,
		}
		e.Labels, err = img.Labels(ctx)
		if err != nil {
			return nil, fmt.Errorf("retrieving label for image %q: you may need to remove the image to resolve the error: %w", img.ID(), err)
		}

		ctnrs, err := img.Containers()
		if err != nil {
			return nil, fmt.Errorf("retrieving containers for image %q: you may need to remove the image to resolve the error: %w", img.ID(), err)
		}
		e.Containers = len(ctnrs)

		sz, err := img.Size()
		if err != nil {
			return nil, fmt.Errorf("retrieving size of image %q: you may need to remove the image to resolve the error: %w", img.ID(), err)
		}
		e.Size = sz
		// This is good enough for now, but has to be
		// replaced later with correct calculation logic
		e.VirtualSize = sz

		summaries = append(summaries, &e)
	}
	return summaries, nil
}
