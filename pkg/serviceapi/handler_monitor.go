package serviceapi

import (
	"github.com/gorilla/mux"
)

func (s *APIServer) registerMonitorHandlers(r *mux.Router) error {
	r.Handle(versionedPath("/monitor"), s.serviceHandler(s.unsupportedHandler))
	return nil
}
