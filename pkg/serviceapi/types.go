package serviceapi

import (
	"context"

	podman "github.com/containers/libpod/libpod/image"
	docker "github.com/docker/docker/api/types"
	"github.com/pkg/errors"
)

type ImageSummary struct {
	docker.ImageSummary
}

func ImageToImageSummary(p *podman.Image) (*ImageSummary, error) {
	containers, err := p.Containers()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to obtain Containers for image %s", p.ID())
	}
	containerCount := len(containers)

	var digests []string
	for _, d := range p.Digests() {
		digests = append(digests, string(d))
	}

	tags, err := p.RepoTags()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to obtain RepoTags for image %s", p.ID())
	}

	// parent, err := p.GetParent(context.TODO())
	// if err != nil {
	// 	return nil, errors.Wrapf(err, "Failed to obtain ParentID for image %s", p.ID())
	// }

	labels, err := p.Labels(context.TODO())
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to obtain Labels for image %s", p.ID())
	}

	size, err := p.Size(context.TODO())
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to obtain Size for image %s", p.ID())
	}
	return &ImageSummary{docker.ImageSummary{
		Containers:  int64(containerCount),
		Created:     p.Created().Unix(),
		ID:          p.ID(),
		Labels:      labels,
		ParentID:    "parent.ID()",
		RepoDigests: digests,
		RepoTags:    tags,
		SharedSize:  0,
		Size:        int64(*size),
		VirtualSize: int64(*size),
	}}, nil
}
