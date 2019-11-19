package serviceapi

import (
	"context"
	"strings"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	libpodImage "github.com/containers/libpod/libpod/image"
	libpodInspect "github.com/containers/libpod/pkg/inspect"
	docker "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	dockerNetwork "github.com/docker/docker/api/types/network"
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
	docker.Container
	docker.ContainerCreateConfig
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

type CreateContainer struct {
	Name string
	dockerContainer.Config
	HostConfig       dockerContainer.HostConfig
	NetworkingConfig dockerNetwork.NetworkingConfig
}

type CommitResponse struct {
	ID string `json:"id"`
}

type Stats struct {
	docker.StatsJSON
}

func ImageToImageSummary(l *libpodImage.Image) (*ImageSummary, error) {
	containers, err := l.Containers()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to obtain Containers for image %s", l.ID())
	}
	containerCount := len(containers)

	var digests []string
	for _, d := range l.Digests() {
		digests = append(digests, string(d))
	}

	tags, err := l.RepoTags()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to obtain RepoTags for image %s", l.ID())
	}

	// FIXME: GetParent() panics
	// parent, err := l.GetParent(context.TODO())
	// if err != nil {
	// 	return nil, errors.Wrapf(err, "Failed to obtain ParentID for image %s", l.ID())
	// }

	labels, err := l.Labels(context.TODO())
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to obtain Labels for image %s", l.ID())
	}

	size, err := l.Size(context.TODO())
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to obtain Size for image %s", l.ID())
	}
	return &ImageSummary{docker.ImageSummary{
		Containers:  int64(containerCount),
		Created:     l.Created().Unix(),
		ID:          l.ID(),
		Labels:      labels,
		ParentID:    "parent.ID()",
		RepoDigests: digests,
		RepoTags:    tags,
		SharedSize:  0,
		Size:        int64(*size),
		VirtualSize: int64(*size),
	}}, nil
}

func ImageDataToImageInspect(l *libpodInspect.ImageData) (*ImageInspect, error) {
	return &ImageInspect{docker.ImageInspect{
		Architecture:    l.Architecture,
		Author:          l.Author,
		Comment:         l.Comment,
		Config:          &dockerContainer.Config{},
		Container:       "",
		ContainerConfig: nil,
		Created:         l.Created.Format(time.RFC3339Nano),
		DockerVersion:   "",
		GraphDriver:     docker.GraphDriverData{},
		ID:              l.ID,
		Metadata:        docker.ImageMetadata{},
		Os:              l.Os,
		OsVersion:       l.Version,
		Parent:          l.Parent,
		RepoDigests:     l.RepoDigests,
		RepoTags:        l.RepoTags,
		RootFS:          docker.RootFS{},
		Size:            l.Size,
		Variant:         "",
		VirtualSize:     l.VirtualSize,
	}}, nil
}

func LibpodToContainer(l *libpod.Container, infoData []define.InfoData) (*Container, error) {
	imageName, imageId := l.Image()

	sizeRW, err := l.RWSize()
	if err != nil {
		return nil, err
	}

	SizeRootFs, err := l.RootFsSize()
	if err != nil {
		return nil, err
	}

	state, err := l.State()
	if err != nil {
		return nil, err
	}

	return &Container{docker.Container{
		ID:         l.ID(),
		Names:      []string{l.Name()},
		Image:      imageName,
		ImageID:    imageId,
		Command:    strings.Join(l.Command(), " "),
		Created:    l.CreatedTime().Unix(),
		Ports:      nil,
		SizeRw:     sizeRW,
		SizeRootFs: SizeRootFs,
		Labels:     l.Labels(),
		State:      string(state),
		Status:     "",
		HostConfig: struct {
			NetworkMode string `json:",omitempty"`
		}{
			"host"},
		NetworkSettings: nil,
		Mounts:          nil,
	},
		docker.ContainerCreateConfig{},
	}, nil
}
