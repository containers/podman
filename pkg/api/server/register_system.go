package server

import (
	"github.com/containers/libpod/pkg/api/handlers/generic"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerSystemHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/system/df"), APIHandler(s.Context, generic.GetDiskUsage))
	return nil
}
