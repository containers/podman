package serviceapi

import (
	"context"
	goRuntime "runtime"
	"time"

	podmanDefine "github.com/containers/libpod/libpod/define"
	podmanImage "github.com/containers/libpod/libpod/image"
	podmanInspect "github.com/containers/libpod/pkg/inspect"
	"github.com/containers/storage/pkg/system"
	docker "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
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
	Rootless bool
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

func InfoDataToInfo(p []podmanDefine.InfoData) (*Info, error) {
	memInfo, err := system.ReadMemInfo()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to obtain system memory info")
	}

	return &Info{Info: docker.Info{
		Architecture:       goRuntime.GOARCH,
		BridgeNfIP6tables:  false,
		BridgeNfIptables:   false,
		CPUCfsPeriod:       false,
		CPUCfsQuota:        false,
		CPUSet:             false,
		CPUShares:          false,
		CgroupDriver:       "",
		ClusterAdvertise:   "",
		ClusterStore:       "",
		ContainerdCommit:   docker.Commit{},
		Containers:         0,
		ContainersPaused:   0,
		ContainersRunning:  0,
		ContainersStopped:  0,
		Debug:              false,
		DefaultRuntime:     "",
		DockerRootDir:      "",
		Driver:             "",
		DriverStatus:       nil,
		ExperimentalBuild:  false,
		GenericResources:   nil,
		HTTPProxy:          "",
		HTTPSProxy:         "",
		ID:                 "podman",
		IPv4Forwarding:     false,
		Images:             0,
		IndexServerAddress: "",
		InitBinary:         "",
		InitCommit:         docker.Commit{},
		Isolation:          "",
		KernelMemory:       false,
		KernelMemoryTCP:    false,
		KernelVersion:      "",
		Labels:             nil,
		LiveRestoreEnabled: false,
		LoggingDriver:      "",
		MemTotal:           memInfo.MemTotal,
		MemoryLimit:        false,
		NCPU:               goRuntime.NumCPU(),
		NEventsListener:    0,
		NFd:                0,
		NGoroutines:        0,
		Name:               "",
		NoProxy:            "",
		OSType:             "",
		OSVersion:          "",
		OomKillDisable:     false,
		OperatingSystem:    goRuntime.GOOS,
		PidsLimit:          false,
		Plugins:            docker.PluginsInfo{},
		ProductLicense:     "",
		RegistryConfig:     nil,
		RuncCommit:         docker.Commit{},
		Runtimes:           nil,
		SecurityOptions:    nil,
		ServerVersion:      "",
		SwapLimit:          false,
		Swarm:              swarm.Info{},
		SystemStatus:       nil,
		SystemTime:         time.Now().Format(time.RFC3339Nano),
		Warnings:           nil,
	},
		Rootless: false,
		BuildahVersion: "",
	}, nil
}
