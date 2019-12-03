package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) RegisterPluginsHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/plugins"), APIHandler(s.Context, handlers.UnsupportedHandler))
	return nil
}
