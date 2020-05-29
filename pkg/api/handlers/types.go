package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/containers/image/v5/manifest"
	libpodImage "github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/domain/entities"
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

type LibpodImagesLoadReport struct {
	ID string `json:"id"`
}

type LibpodImagesPullReport struct {
	ID string `json:"id"`
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

type CreateContainerConfig struct {
	Name string
	dockerContainer.Config
	HostConfig       dockerContainer.HostConfig
	NetworkingConfig dockerNetwork.NetworkingConfig
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

type HistoryResponse struct {
	ID        string   `json:"Id"`
	Created   int64    `json:"Created"`
	CreatedBy string   `json:"CreatedBy"`
	Tags      []string `json:"Tags"`
	Size      int64    `json:"Size"`
	Comment   string   `json:"Comment"`
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

func ImageToImageSummary(l *libpodImage.Image) (*entities.ImageSummary, error) {
	containers, err := l.Containers()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to obtain Containers for image %s", l.ID())
	}
	containerCount := len(containers)

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

	repoTags, err := l.RepoTags()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to obtain RepoTags for image %s", l.ID())
	}

	history, err := l.History(context.TODO())
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to obtain History for image %s", l.ID())
	}
	historyIds := make([]string, len(history))
	for i, h := range history {
		historyIds[i] = h.ID
	}

	digests := make([]string, len(l.Digests()))
	for i, d := range l.Digests() {
		digests[i] = string(d)
	}

	is := entities.ImageSummary{
		ID:           l.ID(),
		ParentId:     l.Parent,
		RepoTags:     repoTags,
		Created:      l.Created(),
		Size:         int64(*size),
		SharedSize:   0,
		VirtualSize:  l.VirtualSize,
		Labels:       labels,
		Containers:   containerCount,
		ReadOnly:     l.IsReadOnly(),
		Dangling:     l.Dangling(),
		Names:        l.Names(),
		Digest:       string(l.Digest()),
		Digests:      digests,
		ConfigDigest: string(l.ConfigDigest),
		History:      historyIds,
	}
	return &is, nil
}

func ImageDataToImageInspect(ctx context.Context, l *libpodImage.Image) (*ImageInspect, error) {
	info, err := l.Inspect(context.Background())
	if err != nil {
		return nil, err
	}
	ports, err := portsToPortSet(info.Config.ExposedPorts)
	if err != nil {
		return nil, err
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
		Labels: info.Labels,
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
	// For docker images, we need to get the Container id and config
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
		// populate the Container id into the image
		dockerImageInspect.Container = d.Container
		containerConfig := dockerContainer.Config{}
		configBytes, err := json.Marshal(d.ContainerConfig)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(configBytes, &containerConfig); err != nil {
			return nil, err
		}
		// populate the Container config in the image
		dockerImageInspect.ContainerConfig = &containerConfig
		// populate parent
		dockerImageInspect.Parent = d.Parent.String()
	}
	return &ImageInspect{dockerImageInspect}, nil

}

// portsToPortSet converts libpods exposed ports to dockers structs
func portsToPortSet(input map[string]struct{}) (nat.PortSet, error) {
	ports := make(nat.PortSet)
	for k := range input {
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
	return ports, nil
}
