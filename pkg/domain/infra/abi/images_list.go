package abi

import (
	"context"

	libpodImage "github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/domain/entities"
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
			Digests:      digests,
			History:      img.NamesHistory(),
			Names:        img.Names(),
			ParentId:     img.Parent,
			ReadOnly:     img.IsReadOnly(),
			SharedSize:   0,
			VirtualSize:  img.VirtualSize,
			RepoTags:     img.Names(), // may include tags and digests
		}
		e.Labels, _ = img.Labels(context.TODO())

		ctnrs, _ := img.Containers()
		e.Containers = len(ctnrs)

		sz, _ := img.Size(context.TODO())
		e.Size = int64(*sz)

		summaries = append(summaries, &e)
	}
	return summaries, nil
}
