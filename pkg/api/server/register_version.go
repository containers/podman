package server

import (
	"github.com/containers/libpod/pkg/api/handlers/generic"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerVersionHandlers(r *mux.Router) error {
	r.Handle("/version", APIHandler(s.Context, generic.VersionHandler))
	r.Handle(VersionedPath("/version"), APIHandler(s.Context, generic.VersionHandler))
	return nil
}
