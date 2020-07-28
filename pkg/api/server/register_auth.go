package server

import (
	"github.com/containers/podman/v2/pkg/api/handlers/compat"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerAuthHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/auth"), s.APIHandler(compat.UnsupportedHandler))
	// Added non version path to URI to support docker non versioned paths
	r.Handle("/auth", s.APIHandler(compat.UnsupportedHandler))
	return nil
}
