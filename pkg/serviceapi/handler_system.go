package serviceapi

import (
	"net/http"

	"github.com/containers/libpod/libpod"
	docker "github.com/docker/docker/api/types"
	"github.com/gorilla/mux"
)

func registerSystemHandlers(r *mux.Router) error {
	r.Handle(unversionedPath("/system/df"), serviceHandler(diskUsage))
	return nil
}

func diskUsage(w http.ResponseWriter, r *http.Request, runtime *libpod.Runtime) {
	w.(ServiceWriter).WriteJSON(http.StatusOK, DiskUsage{docker.DiskUsage{
		LayersSize: 0,
		Images:     nil,
		Containers: nil,
		Volumes:    nil,
	}})
}
