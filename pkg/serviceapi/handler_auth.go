package serviceapi

import (
	"github.com/gorilla/mux"
)

func (s *APIServer) registerAuthHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/auth"), s.serviceHandler(s.unsupportedHandler))
	return nil
}
