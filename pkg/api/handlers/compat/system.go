//go:build !remote

package compat

import (
	"net/http"
	"strings"

	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/infra/abi"
	docker "github.com/docker/docker/api/types"
	dockerImage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
)

func GetDiskUsage(w http.ResponseWriter, r *http.Request) {
	options := entities.SystemDfOptions{}
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}
	df, err := ic.SystemDf(r.Context(), options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	imgs := make([]*dockerImage.Summary, len(df.Images))
	for i, o := range df.Images {
		t := dockerImage.Summary{
			Containers:  int64(o.Containers),
			Created:     o.Created.Unix(),
			ID:          o.ImageID,
			Labels:      map[string]string{},
			ParentID:    "",
			RepoDigests: nil,
			RepoTags:    []string{o.Tag},
			SharedSize:  o.SharedSize,
			Size:        o.Size,
			VirtualSize: o.Size - o.UniqueSize,
		}
		imgs[i] = &t
	}

	ctnrs := make([]*docker.Container, len(df.Containers))
	for i, o := range df.Containers {
		t := docker.Container{
			ID:         o.ContainerID,
			Names:      []string{o.Names},
			Image:      o.Image,
			ImageID:    o.Image,
			Command:    strings.Join(o.Command, " "),
			Created:    o.Created.Unix(),
			Ports:      nil,
			SizeRw:     o.RWSize,
			SizeRootFs: o.Size,
			Labels:     map[string]string{},
			State:      o.Status,
			Status:     o.Status,
			HostConfig: struct {
				NetworkMode string            `json:",omitempty"`
				Annotations map[string]string `json:",omitempty"`
			}{},
			NetworkSettings: nil,
			Mounts:          nil,
		}
		ctnrs[i] = &t
	}

	vols := make([]*volume.Volume, len(df.Volumes))
	for i, o := range df.Volumes {
		t := volume.Volume{
			CreatedAt:  "",
			Driver:     "",
			Labels:     map[string]string{},
			Mountpoint: "",
			Name:       o.VolumeName,
			Options:    nil,
			Scope:      "local",
			Status:     nil,
			UsageData: &volume.UsageData{
				RefCount: int64(o.Links),
				Size:     o.Size,
			},
		}
		vols[i] = &t
	}

	utils.WriteResponse(w, http.StatusOK, handlers.DiskUsage{DiskUsage: docker.DiskUsage{
		LayersSize:  0,
		Images:      imgs,
		Containers:  ctnrs,
		Volumes:     vols,
		BuildCache:  []*docker.BuildCache{},
		BuilderSize: 0,
	}})
}
