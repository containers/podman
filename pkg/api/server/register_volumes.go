package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerVolumeHandlers(r *mux.Router) error {
	r.Handle("/libpod/volumes/create", APIHandler(s.Context, libpod.CreateVolume)).Methods(http.MethodPost)
	r.Handle("/libpod/volumes/json", APIHandler(s.Context, libpod.ListVolumes)).Methods(http.MethodGet)
	r.Handle("/libpod/volumes/prune", APIHandler(s.Context, libpod.PruneVolumes)).Methods(http.MethodPost)
	r.Handle("/libpod/volumes/{name:..*}/json", APIHandler(s.Context, libpod.InspectVolume)).Methods(http.MethodGet)
	r.Handle("/libpod/volumes/{name:..*}", APIHandler(s.Context, libpod.RemoveVolume)).Methods(http.MethodDelete)
	return nil
}
