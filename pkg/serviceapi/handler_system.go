package serviceapi

import (
	"encoding/json"
	"io"
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
	buffer, err := json.Marshal(DiskUsage{docker.DiskUsage{
		LayersSize: 0,
		Images:     nil,
		Containers: nil,
		Volumes:    nil,
	}})
	if err != nil {
		Error(w, "server error", http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, string(buffer))
}
