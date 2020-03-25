// +build ABISupport

package abi

import (
	"context"

	libpodImage "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/domain/entities"
)

func (ir *ImageEngine) List(ctx context.Context, opts entities.ImageListOptions) ([]*entities.ImageSummary, error) {
	var (
		images []*libpodImage.Image
		err    error
	)

	// TODO: Future work support for domain.Filters
	// filters := utils.ToLibpodFilters(opts.Filters)

	if len(opts.Filter) > 0 {
		images, err = ir.Libpod.ImageRuntime().GetImagesWithFilters(opts.Filter)
	} else {
		images, err = ir.Libpod.ImageRuntime().GetImages()
	}
	if err != nil {
		return nil, err
	}

	summaries := make([]*entities.ImageSummary, len(images))
	for i, img := range images {
		var repoTags []string
		if opts.All {
			pairs, err := libpodImage.ReposToMap(img.Names())
			if err != nil {
				return nil, err
			}

			for repo, tags := range pairs {
				for _, tag := range tags {
					repoTags = append(repoTags, repo+":"+tag)
				}
			}
		} else {
			repoTags, _ = img.RepoTags()
		}

		digests := make([]string, len(img.Digests()))
		for j, d := range img.Digests() {
			digests[j] = string(d)
		}

		e := entities.ImageSummary{
			ID: img.ID(),

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
			RepoTags:     repoTags,
		}
		e.Labels, _ = img.Labels(context.TODO())

		ctnrs, _ := img.Containers()
		e.Containers = len(ctnrs)

		sz, _ := img.Size(context.TODO())
		e.Size = int64(*sz)

		summaries[i] = &e
	}
	return summaries, nil
}
