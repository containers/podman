package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/compat"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerVersionHandlers(r *mux.Router) error {
	r.Handle("/version", s.APIHandler(compat.VersionHandler)).Methods(http.MethodGet)
	r.Handle(VersionedPath("/version"), s.APIHandler(compat.VersionHandler)).Methods(http.MethodGet)
	return nil
}
