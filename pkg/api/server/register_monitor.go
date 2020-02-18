package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerMonitorHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/monitor"), s.APIHandler(handlers.UnsupportedHandler))
	return nil
}
