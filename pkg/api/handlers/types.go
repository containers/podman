package handlers

import (
	"context"
	"time"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v3/pkg/domain/entities"
	docker "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	dockerNetwork "github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

type AuthConfig struct {
	docker.AuthConfig
}

type ImageInspect struct {
	docker.ImageInspect
}

type ContainerConfig struct {
	dockerContainer.Config
}

type LibpodImagesPullReport struct {
	entities.ImagePullReport
}

// LibpodImagesRemoveReport is the return type for image removal via the rest
// api.
type LibpodImagesRemoveReport struct {
	entities.ImageRemoveReport
	// Image removal requires is to return data and an error.
	Errors []string
}

type ContainersPruneReport struct {
	docker.ContainersPruneReport
}

type LibpodContainersPruneReport struct {
	ID             string `json:"id"`
	SpaceReclaimed int64  `json:"space"`
	PruneError     string `json:"error"`
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

// CreateContainerConfig used when compatible endpoint creates a container
type CreateContainerConfig struct {
	Name                   string                         // container name
	dockerContainer.Config                                // desired container configuration
	HostConfig             dockerContainer.HostConfig     // host dependent configuration for container
	NetworkingConfig       dockerNetwork.NetworkingConfig // network configuration for container
}

// swagger:model IDResponse
type IDResponse struct {
	// ID
	ID string `json:"Id"`
}

type ContainerTopOKBody struct {
	dockerContainer.ContainerTopOKBody
}

type PodTopOKBody struct {
	dockerContainer.ContainerTopOKBody
}

// swagger:model PodCreateConfig
type PodCreateConfig struct {
	Name         string   `json:"name"`
	CGroupParent string   `json:"cgroup-parent"`
	Hostname     string   `json:"hostname"`
	Infra        bool     `json:"infra"`
	InfraCommand string   `json:"infra-command"`
	InfraImage   string   `json:"infra-image"`
	Labels       []string `json:"labels"`
	Publish      []string `json:"publish"`
	Share        string   `json:"share"`
}

// HistoryResponse provides details on image layers
type HistoryResponse struct {
	ID        string `json:"Id"`
	Created   int64
	CreatedBy string
	Tags      []string
	Size      int64
	Comment   string
}

type ImageLayer struct{}

type ImageTreeResponse struct {
	ID     string       `json:"id"`
	Tags   []string     `json:"tags"`
	Size   string       `json:"size"`
	Layers []ImageLayer `json:"layers"`
}

type ExecCreateConfig struct {
	docker.ExecConfig
}

type ExecCreateResponse struct {
	docker.IDResponse
}

type ExecStartConfig struct {
	Detach bool `json:"Detach"`
	Tty    bool `json:"Tty"`
}

func ImageToImageSummary(l *libimage.Image) (*entities.ImageSummary, error) {
	imageData, err := l.Inspect(context.TODO(), true)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to obtain summary for image %s", l.ID())
	}

	containers, err := l.Containers()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to obtain Containers for image %s", l.ID())
	}
	containerCount := len(containers)

	is := entities.ImageSummary{
		ID:           l.ID(),
		ParentId:     imageData.Parent,
		RepoTags:     imageData.RepoTags,
		RepoDigests:  imageData.RepoDigests,
		Created:      l.Created().Unix(),
		Size:         imageData.Size,
		SharedSize:   0,
		VirtualSize:  imageData.VirtualSize,
		Labels:       imageData.Labels,
		Containers:   containerCount,
		ReadOnly:     l.IsReadOnly(),
		Dangling:     l.IsDangling(),
		Names:        l.Names(),
		Digest:       string(imageData.Digest),
		ConfigDigest: "", // TODO: libpod/image didn't set it but libimage should
		History:      imageData.NamesHistory,
	}
	return &is, nil
}

func ImageDataToImageInspect(ctx context.Context, l *libimage.Image) (*ImageInspect, error) {
	info, err := l.Inspect(context.Background(), true)
	if err != nil {
		return nil, err
	}
	ports, err := portsToPortSet(info.Config.ExposedPorts)
	if err != nil {
		return nil, err
	}

	// TODO: many fields in Config still need wiring
	config := dockerContainer.Config{
		User:         info.User,
		ExposedPorts: ports,
		Env:          info.Config.Env,
		Cmd:          info.Config.Cmd,
		Volumes:      info.Config.Volumes,
		WorkingDir:   info.Config.WorkingDir,
		Entrypoint:   info.Config.Entrypoint,
		Labels:       info.Labels,
		StopSignal:   info.Config.StopSignal,
	}

	rootfs := docker.RootFS{}
	if info.RootFS != nil {
		rootfs.Type = info.RootFS.Type
		rootfs.Layers = make([]string, 0, len(info.RootFS.Layers))
		for _, layer := range info.RootFS.Layers {
			rootfs.Layers = append(rootfs.Layers, string(layer))
		}
	}

	graphDriver := docker.GraphDriverData{
		Name: info.GraphDriver.Name,
		Data: info.GraphDriver.Data,
	}
	dockerImageInspect := docker.ImageInspect{
		Architecture:  info.Architecture,
		Author:        info.Author,
		Comment:       info.Comment,
		Config:        &config,
		Created:       l.Created().Format(time.RFC3339Nano),
		DockerVersion: info.Version,
		GraphDriver:   graphDriver,
		ID:            "sha256:" + l.ID(),
		Metadata:      docker.ImageMetadata{},
		Os:            info.Os,
		OsVersion:     info.Version,
		Parent:        info.Parent,
		RepoDigests:   info.RepoDigests,
		RepoTags:      info.RepoTags,
		RootFS:        rootfs,
		Size:          info.Size,
		Variant:       "",
		VirtualSize:   info.VirtualSize,
	}
	// TODO: consider filling the container config.
	return &ImageInspect{dockerImageInspect}, nil
}

// portsToPortSet converts libpods exposed ports to dockers structs
func portsToPortSet(input map[string]struct{}) (nat.PortSet, error) {
	ports := make(nat.PortSet)
	for k := range input {
		proto, port := nat.SplitProtoPort(k)
		switch proto {
		// See the OCI image spec for details:
		// https://github.com/opencontainers/image-spec/blob/e562b04403929d582d449ae5386ff79dd7961a11/config.md#properties
		case "tcp", "":
			p, err := nat.NewPort("tcp", port)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to create tcp port from %s", k)
			}
			ports[p] = struct{}{}
		case "udp":
			p, err := nat.NewPort("udp", port)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to create tcp port from %s", k)
			}
			ports[p] = struct{}{}
		default:
			return nil, errors.Errorf("invalid port proto %q in %q", proto, k)
		}
	}
	return ports, nil
}
