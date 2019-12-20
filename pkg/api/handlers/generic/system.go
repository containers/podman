package generic

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"net/http"

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
