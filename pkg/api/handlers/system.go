package handlers

import (
	"net/http"

	docker "github.com/docker/docker/api/types"
)

func GetDiskUsage(w http.ResponseWriter, r *http.Request) {
	WriteResponse(w, http.StatusOK, DiskUsage{docker.DiskUsage{
		LayersSize: 0,
		Images:     nil,
		Containers: nil,
		Volumes:    nil,
	}})
}
