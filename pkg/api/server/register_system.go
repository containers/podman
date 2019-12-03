package server

import (
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/gorilla/mux"
)

func (s *APIServer) registerSystemHandlers(r *mux.Router) error {
	r.Handle(VersionedPath("/system/df"), APIHandler(s.Context, handlers.GetDiskUsage))
	return nil
}
