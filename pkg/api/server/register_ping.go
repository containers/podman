package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerPingHandlers(r *mux.Router) error {
	r.Handle("/_ping", APIHandler(s.Context, handlers.PingGET)).Methods("GET")
	r.Handle("/_ping", APIHandler(s.Context, handlers.PingHEAD)).Methods("HEAD")
	return nil
}
