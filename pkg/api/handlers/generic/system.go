package generic

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
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
