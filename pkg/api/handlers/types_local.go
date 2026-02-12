//go:build !remote

package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/containers/common/libimage"
	docker "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

func ImageDataToImageInspect(ctx context.Context, l *libimage.Image) (*ImageInspect, error) {
	options := &libimage.InspectOptions{WithParent: true, WithSize: true}
	info, err := l.Inspect(ctx, options)
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
				return nil, fmt.Errorf("unable to create tcp port from %s: %w", k, err)
			}
			ports[p] = struct{}{}
		case "udp":
			p, err := nat.NewPort("udp", port)
			if err != nil {
				return nil, fmt.Errorf("unable to create tcp port from %s: %w", k, err)
			}
			ports[p] = struct{}{}
		default:
			return nil, fmt.Errorf("invalid port proto %q in %q", proto, k)
		}
	}
	return ports, nil
}
