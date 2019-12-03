package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) RegisterMonitorHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/monitor"), APIHandler(s.Context, handlers.UnsupportedHandler))
	return nil
}
