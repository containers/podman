package handlers

import (
	"github.com/containers/podman/v4/pkg/domain/entities"
	docker "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	dockerNetwork "github.com/docker/docker/api/types/network"
	"github.com/opencontainers/runtime-spec/specs-go"
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

// UpdateEntities used to wrap the oci resource spec in a swagger model
// swagger:model
type UpdateEntities struct {
	Resources *specs.LinuxResources
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
	EnvMerge               []string                       // preprocess env variables from image before injecting into containers
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

type ExecRemoveConfig struct {
	Force bool `json:"Force"`
}
