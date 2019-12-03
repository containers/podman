package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) RegisterDistributionHandlers(r *mux.Router) error {
	r.HandleFunc(VersionedPath("/distribution/{name:..*}/json"), handlers.UnsupportedHandler)
	return nil
}
