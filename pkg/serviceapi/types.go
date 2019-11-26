package serviceapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/libpod/define"
	libpodImage "github.com/containers/libpod/libpod/image"
	docker "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	dockerNetwork "github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
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
	BuildahVersion     string
	CPURealtimePeriod  bool
	CPURealtimeRuntime bool
	CgroupVersion      string
	Rootless           bool
	SwapFree           int64
	SwapTotal          int64
	Uptime             string
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

type ContainerTopOKBody struct {
	dockerContainer.ContainerTopOKBody
	ID string `json:"Id"`
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

func ImageDataToImageInspect(ctx context.Context, l *libpodImage.Image) (*ImageInspect, error) {
	ports := make(nat.PortSet)
	info, err := l.Inspect(context.Background())
	if err != nil {
		return nil, err
	}
	if len(info.Config.ExposedPorts) > 0 {
		for k := range info.Config.ExposedPorts {
			npTCP, err := nat.NewPort("tcp", k)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to create tcp port from %s", k)
			}
			npUDP, err := nat.NewPort("udp", k)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to create udp port from %s", k)
			}
			ports[npTCP] = struct{}{}
			ports[npUDP] = struct{}{}
		}
	}
	// TODO the rest of these still need wiring!
	config := dockerContainer.Config{
		//	Hostname:        "",
		//	Domainname:      "",
		User: info.User,
		//	AttachStdin:     false,
		//	AttachStdout:    false,
		//	AttachStderr:    false,
		ExposedPorts: ports,
		//	Tty:             false,
		//	OpenStdin:       false,
		//	StdinOnce:       false,
		Env: info.Config.Env,
		Cmd: info.Config.Cmd,
		//	Healthcheck:     nil,
		//	ArgsEscaped:     false,
		//	Image:           "",
		//	Volumes:         nil,
		//	WorkingDir:      "",
		//	Entrypoint:      nil,
		//	NetworkDisabled: false,
		//	MacAddress:      "",
		//	OnBuild:         nil,
		//	Labels:          nil,
		//	StopSignal:      "",
		//	StopTimeout:     nil,
		//	Shell:           nil,
	}
	ic, err := l.ToImageRef(ctx)
	if err != nil {
		return nil, err
	}
	dockerImageInspect := docker.ImageInspect{
		Architecture:  l.Architecture,
		Author:        l.Author,
		Comment:       info.Comment,
		Config:        &config,
		Created:       l.Created().Format(time.RFC3339Nano),
		DockerVersion: "",
		GraphDriver:   docker.GraphDriverData{},
		ID:            fmt.Sprintf("sha256:%s", l.ID()),
		Metadata:      docker.ImageMetadata{},
		Os:            l.Os,
		OsVersion:     l.Version,
		Parent:        l.Parent,
		RepoDigests:   info.RepoDigests,
		RepoTags:      info.RepoTags,
		RootFS:        docker.RootFS{},
		Size:          info.Size,
		Variant:       "",
		VirtualSize:   info.VirtualSize,
	}
	bi := ic.ConfigInfo()
	// For docker images, we need to get the container id and config
	// and populate the image with it.
	if bi.MediaType == manifest.DockerV2Schema2ConfigMediaType {
		d := manifest.Schema2Image{}
		b, err := ic.ConfigBlob(ctx)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &d); err != nil {
			return nil, err
		}
		// populate the container id into the image
		dockerImageInspect.Container = d.Container
		containerConfig := dockerContainer.Config{}
		configBytes, err := json.Marshal(d.ContainerConfig)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(configBytes, &containerConfig); err != nil {
			return nil, err
		}
		// populate the container config in the image
		dockerImageInspect.ContainerConfig = &containerConfig
		// populate parent
		dockerImageInspect.Parent = d.Parent.String()
	}
	return &ImageInspect{dockerImageInspect}, nil

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
