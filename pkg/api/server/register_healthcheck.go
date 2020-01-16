package server

import (
	"net/http"

	"github.com/containers/libpod/pkg/api/handlers/libpod"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerHealthCheckHandlers(r *mux.Router) error {
	// swagger:operation GET /libpod/containers/{nameOrID}/runhealthcheck libpod libpodRunHealthCheck
	r.Handle(VersionedPath("/libpod/containers/{name:..*}/runhealthcheck"), APIHandler(s.Context, libpod.RunHealthCheck)).Methods(http.MethodGet)
	return nil
}
