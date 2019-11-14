package serviceapi

import (
	"net/http"

	docker "github.com/docker/docker/api/types"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerSystemHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/system/df"), s.serviceHandler(s.diskUsage))
	return nil
}

func (s *APIServer) diskUsage(w http.ResponseWriter, r *http.Request) {
	s.WriteResponse(w, http.StatusOK, DiskUsage{docker.DiskUsage{
		LayersSize: 0,
		Images:     nil,
		Containers: nil,
		Volumes:    nil,
	}})
}
