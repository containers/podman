package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/generic"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerPingHandlers(r *mux.Router) error {
	r.Handle("/_ping", APIHandler(s.Context, generic.PingGET)).Methods(http.MethodGet)
	r.Handle("/_ping", APIHandler(s.Context, generic.PingHEAD)).Methods("HEAD")

	// libpod
	r.Handle("/libpod/_ping", APIHandler(s.Context, generic.PingGET)).Methods(http.MethodGet)
	return nil
}
