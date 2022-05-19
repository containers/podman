package handlers

import (
	"context"
	"time"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v4/pkg/domain/entities"
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

type ContainersPruneReportLibpod struct {
	ID             string `json:"Id"`
	SpaceReclaimed int64  `json:"Size"`
	// Error which occurred during prune operation (if any).
	// This field is optional and may be omitted if no error occurred.
	//
	// Extensions:
	// x-omitempty: true
	// x-nullable: true
	PruneError string `json:"Err,omitempty"`
}

type LibpodContainersRmReport struct {
	ID string `json:"Id"`
	// Error which occurred during Rm operation (if any).
	// This field is optional and may be omitted if no error occurred.
	//
	// Extensions:
	// x-omitempty: true
	// x-nullable: true
	RmError string `json:"Err,omitempty"`
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
	Error      *struct {
		Message string
	}
}

// CreateContainerConfig used when compatible endpoint creates a container
// swagger:model
type CreateContainerConfig struct {
	Name                   string                         // container name
	dockerContainer.Config                                // desired container configuration
	HostConfig             dockerContainer.HostConfig     // host dependent configuration for container
	NetworkingConfig       dockerNetwork.NetworkingConfig // network configuration for container
	UnsetEnv               []string                       // unset specified default environment variables
	UnsetEnvAll            bool                           // unset all default environment variables
}

type ContainerTopOKBody struct {
	dockerContainer.ContainerTopOKBody
}

type PodTopOKBody struct {
	dockerContainer.ContainerTopOKBody
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

type ExecCreateConfig struct {
	docker.ExecConfig
}

type ExecStartConfig struct {
	Detach bool   `json:"Detach"`
	Tty    bool   `json:"Tty"`
	Height uint16 `json:"h"`
	Width  uint16 `json:"w"`
}

func ImageDataToImageInspect(ctx context.Context, l *libimage.Image) (*ImageInspect, error) {
	options := &libimage.InspectOptions{WithParent: true, WithSize: true}
	info, err := l.Inspect(context.Background(), options)
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
	// Add in basic ContainerConfig to satisfy docker-compose
	cc := new(dockerContainer.Config)
	cc.Hostname = info.ID[0:11] // short ID is the hostname
	cc.Volumes = info.Config.Volumes

	dockerImageInspect := docker.ImageInspect{
		Architecture:    info.Architecture,
		Author:          info.Author,
		Comment:         info.Comment,
		Config:          &config,
		ContainerConfig: cc,
		Created:         l.Created().Format(time.RFC3339Nano),
		DockerVersion:   info.Version,
		GraphDriver:     graphDriver,
		ID:              "sha256:" + l.ID(),
		Metadata:        docker.ImageMetadata{},
		Os:              info.Os,
		OsVersion:       info.Version,
		Parent:          info.Parent,
		RepoDigests:     info.RepoDigests,
		RepoTags:        info.RepoTags,
		RootFS:          rootfs,
		Size:            info.Size,
		Variant:         "",
		VirtualSize:     info.VirtualSize,
	}
	return &ImageInspect{dockerImageInspect}, nil
}

// portsToPortSet converts libpod's exposed ports to docker's structs
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
