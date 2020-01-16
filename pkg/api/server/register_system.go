package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/generic"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerSystemHandlers(r *mux.Router) error {
	// swagger:operation GET /system/df compat getDiskUsage
	r.Handle(VersionedPath("/system/df"), APIHandler(s.Context, generic.GetDiskUsage)).Methods(http.MethodGet)
	return nil
}
