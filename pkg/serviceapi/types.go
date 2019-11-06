package serviceapi

import (
	"context"
	"time"

	podmanImage "github.com/containers/libpod/libpod/image"
	podmanInspect "github.com/containers/libpod/pkg/inspect"
	docker "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	"github.com/pkg/errors"
)

type ImageInspect struct {
	docker.ImageInspect
}

type ContainerConfig struct {
	dockerContainer.Config
}

type ImageSummary struct {
	docker.ImageSummary
}

type Info struct {
	docker.Info
	BuildahVersion string
	CgroupVersion  string
	Rootless       bool
	SwapFree       int64
	SwapTotal      int64
	Uptime         string
}

type Container struct {
	docker.ContainerJSON
}

type ContainerStats struct {
	docker.ContainerStats
}

type Ping struct {
	docker.Ping
}

type Version struct {
	docker.Version
}

type DiskUsage struct {
	docker.DiskUsage
}

type VolumesPruneReport struct {
	docker.VolumesPruneReport
}

type ImagesPruneReport struct {
	docker.ImagesPruneReport
}

type BuildCachePruneReport struct {
	docker.BuildCachePruneReport
}

type NetworkPruneReport struct {
	docker.NetworksPruneReport
}

type ConfigCreateResponse struct {
	docker.ConfigCreateResponse
}

type PushResult struct {
	docker.PushResult
}

type BuildResult struct {
	docker.BuildResult
}

type ContainerWaitOKBody struct {
	StatusCode int
	Error      struct {
		Message string
	}
}

func ImageToImageSummary(p *podmanImage.Image) (*ImageSummary, error) {
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

	// FIXME: GetParent() panics
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

func ImageDataToImageInspect(p *podmanInspect.ImageData) (*ImageInspect, error) {
	return &ImageInspect{docker.ImageInspect{
		Architecture:    p.Architecture,
		Author:          p.Author,
		Comment:         p.Comment,
		Config:          &dockerContainer.Config{},
		Container:       "",
		ContainerConfig: nil,
		Created:         p.Created.Format(time.RFC3339Nano),
		DockerVersion:   "",
		GraphDriver:     docker.GraphDriverData{},
		ID:              p.ID,
		Metadata:        docker.ImageMetadata{},
		Os:              p.Os,
		OsVersion:       p.Version,
		Parent:          p.Parent,
		RepoDigests:     p.RepoDigests,
		RepoTags:        p.RepoTags,
		RootFS:          docker.RootFS{},
		Size:            p.Size,
		Variant:         "",
		VirtualSize:     p.VirtualSize,
	}}, nil
}
