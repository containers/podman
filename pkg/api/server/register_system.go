package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/compat"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerSystemHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/system/df"), s.APIHandler(compat.GetDiskUsage)).Methods(http.MethodGet)
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/system/df", s.APIHandler(compat.GetDiskUsage)).Methods(http.MethodGet)
	return nil
}
