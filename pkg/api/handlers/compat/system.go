package compat

import (
	"net/http"

	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	docker "github.com/docker/docker/api/types"
)

func GetDiskUsage(w http.ResponseWriter, r *http.Request) {
	utils.WriteResponse(w, http.StatusOK, handlers.DiskUsage{DiskUsage: docker.DiskUsage{
		LayersSize: 0,
		Images:     nil,
		Containers: nil,
		Volumes:    nil,
	}})
}
