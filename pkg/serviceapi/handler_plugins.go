package serviceapi

import (
	"github.com/gorilla/mux"
)

func (s *APIServer) registerPluginsHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/plugins"), s.serviceHandler(s.unsupportedHandler))
	return nil
}
