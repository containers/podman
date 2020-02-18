package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerHealthCheckHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/libpod/containers/{name}/runhealthcheck"), s.APIHandler(libpod.RunHealthCheck)).Methods(http.MethodGet)
	return nil
}
