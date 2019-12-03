package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) RegisterAuthHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/auth"), APIHandler(s.Context, handlers.UnsupportedHandler))
	return nil
}
