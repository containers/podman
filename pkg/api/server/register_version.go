package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerVersionHandlers(r *mux.Router) error {
	r.Handle("/version", APIHandler(s.Context, handlers.VersionHandler))
	r.Handle(VersionedPath("/version"), APIHandler(s.Context, handlers.VersionHandler))
	return nil
}
