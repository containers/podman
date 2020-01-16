package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/generic"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerPingHandlers(r *mux.Router) error {
	// swagger:operation GET /_ping compat pingGET
	r.Handle("/_ping", APIHandler(s.Context, generic.PingGET)).Methods(http.MethodGet)
	// swagger:operation HEAD /_ping compat pingHEAD
	r.Handle("/_ping", APIHandler(s.Context, generic.PingHEAD)).Methods("HEAD")

	// swagger:operation GET /libpod/_ping libpod pingGET
	// libpod
	r.Handle("/libpod/_ping", APIHandler(s.Context, generic.PingGET)).Methods(http.MethodGet)
	return nil
}
